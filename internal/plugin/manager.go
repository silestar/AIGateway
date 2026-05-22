package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	adapterregistry "github.com/silestar/AIGateway/pkg/adapter/registry"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Manager 插件管理器实现
type Manager struct {
	db              *gorm.DB
	logger          *zap.Logger
	pluginsDir      string
	logDir          string   // 日志根目录（从 config.Log.Dir 读取）
	client          *http.Client
	autoGrantPerms  bool                    // 自动授权所有权限
	permCache       map[string][]string     // plugin_name → 已授予权限列表
	permCacheMu     sync.RWMutex
	pluginUID       uint32   // agw-plugin 用户的 UID（启动时从系统解析）
	pluginGID       uint32   // agw-plugin 用户的 GID
	logFileHandles  map[uint]*os.File       // pluginID → 日志文件句柄（Stop 时关闭）
	logFileMu       sync.Mutex              // 保护 logFileHandles
	registry         *hookRegistry           // 钩子内存注册表
}

// ========== 钩子内存注册表 ==========

// hookRegistry 钩子内存注册表（启动时从 DB 加载，运行时增删改）
type hookRegistry struct {
	mu      sync.RWMutex
	entries map[HookName][]*hookEntry // 钩子名 → 已注册插件列表（按 priority 排序）
}

type hookEntry struct {
	PluginName string
	Port       int
	AuthToken  string // 从插件记录读取，用于内部 HTTP 调用鉴权
	Priority   int
	Status     string // running / stopped
}

// loadFromDB 从数据库加载所有 status=running 的插件到注册表
func (r *hookRegistry) loadFromDB(db *gorm.DB) error {
	var plugins []Plugin
	if err := db.Where("status = ?", StatusRunning).Find(&plugins).Error; err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = make(map[HookName][]*hookEntry)

	for _, p := range plugins {
		var hooks []string
		json.Unmarshal([]byte(p.Hooks), &hooks)
		for _, h := range hooks {
			entry := &hookEntry{
				PluginName: p.Name,
				Port:       p.Port,
				AuthToken:  p.AuthToken,
				Priority:   p.Priority,
				Status:     StatusRunning,
			}
			r.entries[HookName(h)] = append(r.entries[HookName(h)], entry)
		}
	}
	return nil
}

// insert 插件启动时插入注册表
func (r *hookRegistry) insert(p *Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var hooks []string
	json.Unmarshal([]byte(p.Hooks), &hooks)
	for _, h := range hooks {
		entry := &hookEntry{
			PluginName: p.Name,
			Port:       p.Port,
			AuthToken:  p.AuthToken,
			Priority:   p.Priority,
			Status:     StatusRunning,
		}
		r.entries[HookName(h)] = append(r.entries[HookName(h)], entry)
	}
}

// remove 插件停止时从注册表移除
func (r *hookRegistry) remove(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for hook, entries := range r.entries {
		filtered := make([]*hookEntry, 0, len(entries))
		for _, e := range entries {
			if e.PluginName != pluginName {
				filtered = append(filtered, e)
			}
		}
		r.entries[hook] = filtered
	}
}

// getEntries 获取某钩子的已注册插件列表（已按 priority 排序）
func (r *hookRegistry) getEntries(hook HookName) []*hookEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.entries[hook] // 加载时已排序
}

// updatePriority 更新优先级后重新排序
func (r *hookRegistry) updatePriority(pluginName string, priority int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entries := range r.entries {
		for _, e := range entries {
			if e.PluginName == pluginName {
				e.Priority = priority
			}
		}
	}
}

// NewManager 创建插件管理器
func NewManager(db *gorm.DB, logger *zap.Logger, pluginsDir string, sidecarTimeout int, autoGrant bool, logDir string) *Manager {
	if pluginsDir == "" {
		pluginsDir = "plugins"
	}
	// 确保转为绝对路径，避免 exec.Command 找不到二进制
	if absDir, err := filepath.Abs(pluginsDir); err == nil {
		pluginsDir = absDir
	}
	if logDir == "" {
		logDir = "logs"
	}
	if absLog, err := filepath.Abs(logDir); err == nil {
		logDir = absLog
	}
	timeout := 5 * time.Second
	if sidecarTimeout > 0 {
		timeout = time.Duration(sidecarTimeout) * time.Second
	}

	m := &Manager{
		db:             db,
		logger:         logger,
		pluginsDir:     pluginsDir,
		logDir:         logDir,
		client:         &http.Client{Timeout: timeout},
		autoGrantPerms: autoGrant,
		permCache:      make(map[string][]string),
		logFileHandles: make(map[uint]*os.File),
		pluginUID:      0, // 默认退化模式：以当前用户运行（Lookup 成功后更新为 agw-plugin UID）
		pluginGID:      0,
	}

	// 尝试从系统解析 agw-plugin 用户的 UID/GID
	if u, err := user.Lookup("agw-plugin"); err == nil {
		if uid, err := strconv.ParseUint(u.Uid, 10, 32); err == nil {
			m.pluginUID = uint32(uid)
		}
		if gid, err := strconv.ParseUint(u.Gid, 10, 32); err == nil {
			m.pluginGID = uint32(gid)
		}
	} else {
		logger.Warn("agw-plugin user not found, plugin UID isolation disabled",
			zap.Error(err))
	}

	// 确保插件日志根目录存在
	os.MkdirAll(filepath.Join(m.logDir, "plugins"), 0755)

	// 种子 hooks 数据（幂等）
	if err := m.seedHooks(); err != nil {
		logger.Warn("seed hooks failed", zap.Error(err))
	}

	// 初始化内存注册表
	m.registry = &hookRegistry{}
	if err := m.registry.loadFromDB(db); err != nil {
		logger.Warn("load hook registry from db failed", zap.Error(err))
	}

	return m
}

// AutoMigrate 自动迁移
func (m *Manager) AutoMigrate() error {
	return m.db.AutoMigrate(&Plugin{}, &Hook{}, &PluginPermission{})
}

// Upload 解析 ZIP 中的 manifest.json，返回预览信息（不安装）
func (m *Manager) Upload(ctx context.Context, zipPath string) (*Manifest, string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, "", fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	// 查找 manifest.json
	var manifestData []byte
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		if filepath.Base(f.Name) == "manifest.json" {
			manifestData, _ = io.ReadAll(rc)
		}
		rc.Close()
	}

	if manifestData == nil {
		return nil, "", fmt.Errorf("manifest.json not found in zip")
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, "", fmt.Errorf("parse manifest: %w", err)
	}

	// 检查是否已安装同名插件
	var count int64
	m.db.WithContext(ctx).Model(&Plugin{}).Where("name = ?", manifest.Name).Count(&count)
	if count > 0 {
		return nil, "", fmt.Errorf("plugin '%s' already installed", manifest.Name)
	}

	// 保存 ZIP 到待安装目录
	pendingDir := filepath.Join(m.pluginsDir, ".pending")
	os.MkdirAll(pendingDir, 0755)
	uploadID := fmt.Sprintf("%d", time.Now().UnixNano())
	pendingPath := filepath.Join(pendingDir, uploadID+".zip")

	// 复制 ZIP 到待安装目录
	src, err := os.Open(zipPath)
	if err != nil {
		return nil, "", fmt.Errorf("open source zip: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(pendingPath)
	if err != nil {
		return nil, "", fmt.Errorf("create pending zip: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(pendingPath)
		return nil, "", fmt.Errorf("copy zip: %w", err)
	}

	return &manifest, uploadID, nil
}

// InstallFromUpload 根据上传 ID 执行安装
func (m *Manager) InstallFromUpload(ctx context.Context, uploadID string) (*Plugin, error) {
	pendingPath := filepath.Join(m.pluginsDir, ".pending", uploadID+".zip")
	defer os.Remove(pendingPath) // 安装完成后清理临时 ZIP

	if _, err := os.Stat(pendingPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("upload not found or expired, please re-upload")
	}

	return m.Install(ctx, pendingPath)
}

// Install 安装插件
func (m *Manager) Install(ctx context.Context, zipPath string) (*Plugin, error) {
	// 1. 解压 ZIP
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	// 2. 查找 manifest.json
	var manifestData []byte

	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}

		if filepath.Base(f.Name) == "manifest.json" {
			manifestData, _ = io.ReadAll(rc)
		}
		rc.Close()
	}

	if manifestData == nil {
		return nil, fmt.Errorf("manifest.json not found in zip")
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// 3. 确定当前服务器架构对应的二进制文件名
	binaryName, err := resolveBinaryName(&manifest)
	if err != nil {
		return nil, err // 架构不匹配，拒绝安装
	}

	// 4. 创建插件目录并解压（只解压匹配架构的二进制 + manifest）
	pluginDir := filepath.Join(m.pluginsDir, manifest.Name)
	os.MkdirAll(pluginDir, 0755)

	binaryFound := false
	for _, f := range reader.File {
		baseName := filepath.Base(f.Name)

		// 跳过非目标架构的二进制文件
		if isPluginBinary(baseName, &manifest) && baseName != binaryName {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		outPath := filepath.Join(pluginDir, baseName)
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(outFile, rc)
		outFile.Chmod(0755) // 可执行
		outFile.Close()
		rc.Close()

		if baseName == binaryName {
			binaryFound = true
		}
	}

	if !binaryFound {
		return nil, fmt.Errorf("binary '%s' for current architecture (%s/%s) not found in zip", binaryName, runtime.GOOS, runtime.GOARCH)
	}

	// 5. 入库
	hooksJSON, _ := json.Marshal(manifest.Hooks)
	plugin := &Plugin{
		Name:         manifest.Name,
		Version:      manifest.Version,
		Description:  manifest.Description,
		Author:       manifest.Author,
		PluginType:   manifest.Type,
		Binary:       filepath.Join(pluginDir, binaryName),
		Port:         manifest.Port,
		Hooks:        string(hooksJSON),
		ConfigSchema: string(manifest.ConfigSchema),
		Manifest:     string(manifestData),
		Status:       StatusInstalled,
	}

	if err := m.db.WithContext(ctx).Create(plugin).Error; err != nil {
		return nil, fmt.Errorf("save plugin: %w", err)
	}

	// 5.5. 声明式建表（manifest.tables）
	var tablesCreated []string
	if len(manifest.Tables) > 0 {
		for _, t := range manifest.Tables {
			tableName := "plugin_" + manifest.Name + "_" + t.Name
			sql := buildCreateTableSQL(tableName, t)
			if err := m.db.WithContext(ctx).Exec(sql).Error; err != nil {
				return nil, fmt.Errorf("create table %s: %w", tableName, err)
			}
			tablesCreated = append(tablesCreated, tableName)
		}
	}
	plugin.HasDB = len(tablesCreated) > 0
	if len(tablesCreated) > 0 {
		tablesJSON, _ := json.Marshal(tablesCreated)
		plugin.TablesCreated = string(tablesJSON)
	}

	// 5.6. 补全新增字段
	plugin.DisplayName = manifest.DisplayName
	plugin.Priority = 100 // 默认优先级
	if manifest.Timeout > 0 {
		plugin.Timeout = manifest.Timeout
	}

	// 写回 DB
	if err := m.db.WithContext(ctx).Save(plugin).Error; err != nil {
		return nil, fmt.Errorf("update plugin fields: %w", err)
	}

	// 6. 同步权限声明
	if len(manifest.Permissions) > 0 {
		if err := m.SyncPermissions(ctx, manifest.Name, manifest.Version, manifest.Permissions); err != nil {
			m.logger.Warn("sync plugin permissions failed", zap.String("name", manifest.Name), zap.Error(err))
		}
	}

	m.logger.Info("plugin installed", zap.String("name", manifest.Name), zap.String("version", manifest.Version))
	return plugin, nil
}

// Start 启动插件
func (m *Manager) Start(ctx context.Context, pluginID uint) error {
	var plugin Plugin
	if err := m.db.WithContext(ctx).First(&plugin, pluginID).Error; err != nil {
		return fmt.Errorf("find plugin: %w", err)
	}

	if plugin.Status == StatusRunning {
		return fmt.Errorf("plugin already running")
	}

	// 检查必需权限是否被拒绝
	if missing, err := m.CheckRequiredPermissions(plugin.Name); err == nil && len(missing) > 0 {
		m.logger.Warn("plugin start blocked: required permissions denied",
			zap.String("plugin", plugin.Name),
			zap.Strings("missing_permissions", missing),
		)
		return fmt.Errorf("plugin %s cannot start: required permissions denied: %v", plugin.Name, missing)
	}

	// 诊断：检查二进制文件是否存在且可执行
	if info, err := os.Stat(plugin.Binary); err != nil {
		return fmt.Errorf("plugin binary not found at '%s': %w (pluginsDir=%s)", plugin.Binary, err, m.pluginsDir)
	} else {
		m.logger.Info("plugin binary found",
			zap.String("path", plugin.Binary),
			zap.Int64("size", info.Size()),
			zap.String("perm", info.Mode().String()),
		)
	}

	// 1. 确保插件安装目录权限正确
	m.preparePluginDir(plugin.Name)

	// 2. 确保日志目录存在
	logDir := filepath.Join(m.logDir, "plugins", plugin.Name)
	os.MkdirAll(logDir, 0755)

	// 3. 打开当天日志文件（append 模式）
	logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open plugin log file: %w", err)
	}

	// 4. 创建子进程命令（使用 Background context，避免请求结束后 context cancel 杀死子进程）
	cmd := exec.CommandContext(context.Background(), plugin.Binary)
	cmd.Dir = filepath.Dir(plugin.Binary)

	// 5. stdout/stderr 重定向到日志文件
	cmd.Stdout = f
	cmd.Stderr = f

	// 6. 环境变量
	cmd.Env = append(os.Environ(),
		"PLUGIN_AUTH_TOKEN="+plugin.AuthToken,
		fmt.Sprintf("PLUGIN_PORT=%d", plugin.Port),
		"PLUGIN_CONFIG="+plugin.Config,
		"PLUGIN_LOG_FILE="+logFile,          // 日志路径（只读告知）
		"PLUGIN_LOG_LEVEL=info",              // 日志级别
		"PLUGIN_HOME="+filepath.Dir(plugin.Binary), // 安装目录边界
	)

	// 7. UID 隔离：切换到 agw-plugin 用户（仅 Linux + UID 有效）
	if m.pluginUID != 0 {
		cmd.SysProcAttr = m.pluginSysProcAttr()
		// 确保插件安装目录权限正确（每次 Start 时重新 chown）
		os.Chown(filepath.Dir(plugin.Binary), int(m.pluginUID), int(m.pluginGID))
	}

	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("start plugin process: %w", err)
	}

	// 8. 保存日志文件句柄
	m.logFileMu.Lock()
	m.logFileHandles[pluginID] = f
	m.logFileMu.Unlock()

	pid := cmd.Process.Pid

	// 更新状态
	m.db.WithContext(ctx).Model(&plugin).Updates(map[string]interface{}{
		"pid":    pid,
		"status": StatusRunning,
	})

	m.logger.Info("plugin started", zap.String("name", plugin.Name), zap.Int("pid", pid))

	// 启动后健康确认：同步轮询 /health 端点（最多 3 次，间隔 1 秒）
	healthOK := false
	for i := 0; i < 3; i++ {
		time.Sleep(1 * time.Second)
		healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", plugin.Port+1)
		resp, err := m.client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				healthOK = true
				break
			}
		}
		m.logger.Info("plugin health check attempt",
			zap.String("name", plugin.Name),
			zap.Int("attempt", i+1),
			zap.String("url", healthURL),
			zap.Error(err),
		)
	}
	if !healthOK {
		// 诊断：检查进程是否还活着
		if proc, procErr := os.FindProcess(pid); procErr == nil {
			proc.Release()
			m.logger.Warn("plugin process was alive but health check timed out",
				zap.String("name", plugin.Name),
				zap.Int("pid", pid),
			)
		} else {
			m.logger.Warn("plugin process exited before health check could complete",
				zap.String("name", plugin.Name),
				zap.Int("pid", pid),
				zap.Error(procErr),
			)
		}
		// 健康确认失败，停止进程并标记 stopped
		if proc, err := os.FindProcess(pid); err == nil {
			proc.Kill()
		}
		m.db.WithContext(ctx).Model(&plugin).Updates(map[string]interface{}{
			"pid":    0,
			"status": StatusError,
		})
		m.logger.Warn("plugin health check failed after start, stopped",
			zap.String("name", plugin.Name),
			zap.Int("pid", pid),
		)
	} else {
		// 健康检查通过，尝试探测渠道类型
		m.discoverChannelType(ctx, plugin)
	}

	// 异步监听进程退出，自动更新状态
	go func() {
		err := cmd.Wait()
		exitCode := 0
		exitMsg := "exited normally"
		if err != nil {
			exitMsg = err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		m.db.Model(&Plugin{}).Where("id = ?", plugin.ID).Updates(map[string]interface{}{
			"pid":    0,
			"status": StatusStopped,
		})
		m.logger.Info("plugin process exited",
			zap.String("name", plugin.Name),
			zap.Int("pid", pid),
			zap.Int("exit_code", exitCode),
			zap.String("reason", exitMsg),
		)
	}()

	return nil
}

// Stop 停止插件
func (m *Manager) Stop(ctx context.Context, pluginID uint) error {
	var plugin Plugin
	if err := m.db.WithContext(ctx).First(&plugin, pluginID).Error; err != nil {
		return fmt.Errorf("find plugin: %w", err)
	}

	// 尝试优雅关闭
	url := fmt.Sprintf("http://127.0.0.1:%d/admin/shutdown", plugin.Port)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	req.Header.Set("Authorization", "Bearer "+plugin.AuthToken)
	m.client.Do(req) // 忽略错误，进程可能已停止

	// 等待 3 秒
	time.Sleep(3 * time.Second)

	// 关闭日志文件句柄
	m.logFileMu.Lock()
	if f, ok := m.logFileHandles[pluginID]; ok {
		f.Close()
		delete(m.logFileHandles, pluginID)
	}
	m.logFileMu.Unlock()

	// 如果进程还在，强制 kill
	if plugin.Pid > 0 {
		if proc, err := os.FindProcess(plugin.Pid); err == nil {
			proc.Kill()
		}
	}

	m.db.WithContext(ctx).Model(&plugin).Updates(map[string]interface{}{
		"pid":    0,
		"status": StatusStopped,
	})

	m.logger.Info("plugin stopped", zap.String("name", plugin.Name))
	return nil
}

// pluginSysProcAttr 返回插件子进程的 SysProcAttr，设置 UID/GID 隔离
func (m *Manager) pluginSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: m.pluginUID,
			Gid: m.pluginGID,
		},
	}
}

// preparePluginDir 确保插件安装目录存在且权限正确
func (m *Manager) preparePluginDir(pluginName string) error {
	installDir := filepath.Join(m.pluginsDir, pluginName)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}
	// 插件目录归 agw-plugin 用户所有，AGW（root）仍可管理
	if m.pluginUID != 0 {
		return os.Chown(installDir, int(m.pluginUID), int(m.pluginGID))
	}
	return nil
}

// EnsureSecurePermissions 确保 AGW 关键路径权限正确（仅 Linux，需 root）
func (m *Manager) EnsureSecurePermissions() error {
	// 这些路径必须由 root 拥有，插件用户不可读写
	secured := []struct {
		path string
		mode os.FileMode
	}{
		{filepath.Join(m.pluginsDir, "..", "data"), 0700},
		{filepath.Join(m.pluginsDir, "..", "config.yaml"), 0400},
		{filepath.Join(m.pluginsDir, "..", "config", ".env"), 0400},
	}
	for _, s := range secured {
		if err := os.Chmod(s.path, s.mode); err != nil {
			// 文件可能不存在（如 .env 非必须），只 warn 不 fatal
			m.logger.Warn("secure permission failed", zap.String("path", s.path), zap.Error(err))
		}
		// Chown 为 root:root（与当前进程 UID 相同，在 root 进程中即 0:0）
		os.Chown(s.path, 0, 0)
	}
	return nil
}

// Uninstall 卸载插件（兼容旧接口，不删表）
func (m *Manager) Uninstall(ctx context.Context, pluginID uint, keepLogs bool) error {
	return m.UninstallWithTables(ctx, pluginID, keepLogs, false)
}

// UninstallWithTables 卸载插件（支持表清理）
func (m *Manager) UninstallWithTables(ctx context.Context, pluginID uint, keepLogs bool, dropTables bool) error {
	// 先停止
	m.Stop(ctx, pluginID)

	var plugin Plugin
	if err := m.db.WithContext(ctx).First(&plugin, pluginID).Error; err != nil {
		return fmt.Errorf("find plugin: %w", err)
	}

	// 删除安装目录
	os.RemoveAll(filepath.Dir(plugin.Binary))

	// 删表（如果请求）
	if dropTables && plugin.TablesCreated != "" {
		var tables []string
		if err := json.Unmarshal([]byte(plugin.TablesCreated), &tables); err == nil {
			for _, t := range tables {
				if strings.HasPrefix(t, "plugin_") {
					m.db.Exec("DROP TABLE IF EXISTS " + t)
				}
			}
		}
	}

	// 日志处理
	pluginLogDir := filepath.Join(m.logDir, "plugins", plugin.Name)
	if !keepLogs {
		os.RemoveAll(pluginLogDir)
	}

	// 从注册表移除
	m.registry.remove(plugin.Name)

	// 标记权限记录
	m.db.WithContext(ctx).
		Model(&PluginPermission{}).
		Where("plugin_name = ?", plugin.Name).
		Update("status", StatusUninstalled)

	// 删除缓存
	m.permCacheMu.Lock()
	delete(m.permCache, plugin.Name)
	m.permCacheMu.Unlock()

	// 删除记录
	m.db.WithContext(ctx).Delete(&plugin)

	m.logger.Info("plugin uninstalled",
		zap.String("name", plugin.Name),
		zap.Bool("keep_logs", keepLogs),
		zap.Bool("drop_tables", dropTables))
	return nil
}

// LogsStatus 返回插件日志状态
func (m *Manager) LogsStatus(ctx context.Context, pluginID uint) (*LogsStatusResult, error) {
	var plugin Plugin
	if err := m.db.WithContext(ctx).First(&plugin, pluginID).Error; err != nil {
		return nil, fmt.Errorf("find plugin: %w", err)
	}

	pluginLogDir := filepath.Join(m.logDir, "plugins", plugin.Name)
	hasLogs := false
	if entries, err := os.ReadDir(pluginLogDir); err == nil && len(entries) > 0 {
		hasLogs = true
	}

	return &LogsStatusResult{
		HasLogs: hasLogs,
		LogDir:  pluginLogDir,
	}, nil
}

// TriggerHook 统一钩子调度引擎（新）
// 流程：查 hooks 表 enabled → 查内存注册表 → 参数校验 → 按 priority 调用 → 记录日志
func (m *Manager) TriggerHook(ctx context.Context, hook HookName, req *HookRequest) (*HookResponse, error) {
	// 1. 查 hooks 表：enabled？
	var hookCfg Hook
	if err := m.db.Where("name = ? AND enabled = ?", string(hook), true).First(&hookCfg).Error; err != nil {
		return ContinueHook(), nil
	}

	// 2. 查内存注册表：哪些插件注册了该钩子
	entries := m.registry.getEntries(hook)
	if len(entries) == 0 {
		return ContinueHook(), nil
	}

	// 3. 遍历插件（按 priority，已排序）
	for _, entry := range entries {
		timeout := hookCfg.Timeout

		// 权限过滤
		granted := m.GetGrantedPermissions(entry.PluginName)
		filteredReq := m.filterHookRequest(req, granted)

		callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
		defer cancel()

		url := fmt.Sprintf("http://127.0.0.1:%d/hook/%s", entry.Port, hook)
		body, _ := json.Marshal(filteredReq)
		httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			m.logger.Warn("create hook request failed",
				zap.String("hook", string(hook)),
				zap.String("plugin", entry.PluginName),
				zap.Error(err))
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+entry.AuthToken)

		startTime := time.Now()
		resp, err := m.client.Do(httpReq)
		elapsed := time.Since(startTime)

		if err != nil {
			m.logger.Warn("call plugin hook failed",
				zap.String("hook", string(hook)),
				zap.String("plugin", entry.PluginName),
				zap.Int("timeout_ms", timeout),
				zap.Duration("elapsed", elapsed),
				zap.Error(err))
			continue
		}

		var hookResp HookResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&hookResp)
		resp.Body.Close()

		if decodeErr != nil {
			m.logger.Warn("decode hook response failed",
				zap.String("hook", string(hook)),
				zap.String("plugin", entry.PluginName),
				zap.Error(decodeErr))
			continue
		}

		// 记录调用日志
		m.logger.Info("plugin hook called",
			zap.String("hook", string(hook)),
			zap.String("plugin", entry.PluginName),
			zap.Duration("elapsed", elapsed),
			zap.String("result", string(hookResp.Action)))

		// 拒绝 → 立即返回
		if hookResp.Action == ActionReject {
			return &hookResp, nil
		}

		// account_select → 返回 filter
		if hook == HookAccountSelect && hookResp.Action == ActionFilter {
			return &hookResp, nil
		}

		// 合并修改（pre_request / post_response）
		if hookResp.ModifiedRequest != nil {
			req.Request = hookResp.ModifiedRequest
		}
		if hookResp.ModifiedResponse != nil {
			req.Response = hookResp.ModifiedResponse
		}

		// connection_decorator：返回代理地址，由调用方处理
		if hook == HookConnectionDecorator && hookResp.Action == ActionContinue {
			return &hookResp, nil
		}
	}

	return ContinueHook(), nil
}

// List 列出所有插件
func (m *Manager) List(ctx context.Context) ([]Plugin, error) {
	var plugins []Plugin
	err := m.db.WithContext(ctx).Order("id ASC").Find(&plugins).Error
	return plugins, err
}

// GetByID 获取单个插件
func (m *Manager) GetByID(ctx context.Context, id uint) (*Plugin, error) {
	var plugin Plugin
	err := m.db.WithContext(ctx).First(&plugin, id).Error
	return &plugin, err
}

// UpdateConfig 更新插件配置
func (m *Manager) UpdateConfig(ctx context.Context, id uint, config string) error {
	return m.db.WithContext(ctx).Model(&Plugin{}).Where("id = ?", id).
		Update("config", config).Error
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck(ctx context.Context) {
	var plugins []Plugin
	m.db.WithContext(ctx).Where("status = ?", StatusRunning).Find(&plugins)

	for _, p := range plugins {
		url := fmt.Sprintf("http://127.0.0.1:%d/health", p.Port+1)
		resp, err := m.client.Get(url)
		if err != nil || resp.StatusCode != http.StatusOK {
			// 连续失败标记 unhealthy
			m.logger.Warn("plugin health check failed", zap.String("name", p.Name))
			// 可扩展：连续失败计数，达到阈值后自动禁用
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
}

// ChannelTypeDiscovery 插件渠道类型发现响应
type ChannelTypeDiscovery struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	BaseURL     string `json:"base_url,omitempty"`
	Description string `json:"description,omitempty"`
}

// discoverChannelType 探测插件是否提供渠道类型，如提供则注册到 adapter registry
func (m *Manager) discoverChannelType(ctx context.Context, plugin Plugin) {
	url := fmt.Sprintf("http://127.0.0.1:%d/.well-known/channel-type", plugin.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+plugin.AuthToken)

	resp, err := m.client.Do(req)
	if err != nil {
		return // 插件不提供渠道类型，静默跳过
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return // 插件不提供渠道类型端点
	}
	if resp.StatusCode != http.StatusOK {
		m.logger.Warn("plugin channel-type discovery returned non-200",
			zap.String("name", plugin.Name),
			zap.Int("status", resp.StatusCode),
		)
		return
	}

	var discovery ChannelTypeDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		m.logger.Warn("failed to decode plugin channel-type discovery",
			zap.String("name", plugin.Name),
			zap.Error(err),
		)
		return
	}

	if discovery.Type == "" {
		m.logger.Warn("plugin channel-type discovery missing 'type' field",
			zap.String("name", plugin.Name),
		)
		return
	}

	// 注册到 adapter registry
	adapterregistry.RegisterChannelType(adapterregistry.ChannelTypeInfo{
		Type:        discovery.Type,
		Name:        discovery.Name,
		IsPlugin:    true,
		BaseURL:     discovery.BaseURL,
		Description: discovery.Description,
	})

	m.logger.Info("plugin registered channel type",
		zap.String("plugin", plugin.Name),
		zap.String("channel_type", discovery.Type),
	)
}

// resolveBinaryName 根据当前服务器架构确定应使用的二进制文件名
// 优先查 binaries 映射，未命中则 fallback 到 binary 字段
// 如果两者都无法匹配当前架构，返回错误拒绝安装
func resolveBinaryName(m *Manifest) (string, error) {
	archKey := runtime.GOOS + "/" + runtime.GOARCH

	// 优先：binaries 映射
	if len(m.Binaries) > 0 {
		if name, ok := m.Binaries[archKey]; ok {
			return name, nil
		}
		// 列出 ZIP 支持的架构
		var supported []string
		for k := range m.Binaries {
			supported = append(supported, k)
		}
		return "", fmt.Errorf("plugin does not support current architecture %s (supported: %v)", archKey, supported)
	}

	// fallback：binary 字段（单架构 ZIP）
	if m.Binary != "" {
		return m.Binary, nil
	}

	return "", fmt.Errorf("manifest has no binary or binaries field")
}

// isPluginBinary 判断 ZIP 中的文件名是否是插件的二进制文件
// 通过 manifest 的 binary 和 binaries 字段来判断
func isPluginBinary(filename string, m *Manifest) bool {
	if filename == m.Binary {
		return true
	}
	for _, name := range m.Binaries {
		if filename == name {
			return true
		}
	}
	return false
}

// ========== 权限管理方法 ==========

// SyncPermissions 同步插件权限声明（安装/升级时调用）
func (m *Manager) SyncPermissions(ctx context.Context, pluginName, pluginVersion string, declarations []PermissionDecl) error {
	if len(declarations) == 0 {
		return nil
	}

	for _, decl := range declarations {
		var existing PluginPermission
		err := m.db.WithContext(ctx).
			Where("plugin_name = ? AND permission_name = ?", pluginName, decl.Name).
			First(&existing).Error

		if err == gorm.ErrRecordNotFound {
			// 新权限，创建记录
			record := PluginPermission{
				PluginName:     pluginName,
				PluginVersion:  pluginVersion,
				PermissionName: decl.Name,
				Status:         PermPending,
				Description:    decl.Description,
				Required:       decl.Required,
			}

			// 自动授权模式
			if m.autoGrantPerms {
				record.Status = PermGranted
				now := time.Now()
				record.GrantedBy = "auto"
				record.GrantedAt = &now
				m.logger.Info("plugin_permission_auto_granted",
					zap.String("plugin", pluginName),
					zap.String("permission", decl.Name),
					zap.String("reason", "auto_grant_permissions=true"),
				)
			}

			if err := m.db.WithContext(ctx).Create(&record).Error; err != nil {
				return fmt.Errorf("create permission %s for plugin %s: %w", decl.Name, pluginName, err)
			}
		} else if err == nil {
			// 已有记录，更新描述和 required（来自新版本 manifest）
			updates := map[string]interface{}{
				"description":    decl.Description,
				"required":       decl.Required,
				"plugin_version": pluginVersion,
			}
			if err := m.db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("update permission %s for plugin %s: %w", decl.Name, pluginName, err)
			}
		} else {
			return fmt.Errorf("query permission %s for plugin %s: %w", decl.Name, pluginName, err)
		}
	}

	// 刷新缓存
	m.refreshPermissionCache(ctx, pluginName)
	return nil
}

// GetPermissions 获取插件权限列表
func (m *Manager) GetPermissions(ctx context.Context, pluginName string) ([]PluginPermission, error) {
	var perms []PluginPermission
	err := m.db.WithContext(ctx).
		Where("plugin_name = ? AND status != ?", pluginName, StatusUninstalled).
		Order("id ASC").
		Find(&perms).Error
	return perms, err
}

// GrantPermission 授予插件权限
func (m *Manager) GrantPermission(ctx context.Context, pluginName, permissionName, grantedBy string) error {
	now := time.Now()
	result := m.db.WithContext(ctx).
		Model(&PluginPermission{}).
		Where("plugin_name = ? AND permission_name = ?", pluginName, permissionName).
		Updates(map[string]interface{}{
			"status":     PermGranted,
			"granted_by": grantedBy,
			"granted_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("permission %s not found for plugin %s", permissionName, pluginName)
	}

	m.logger.Info("plugin_permission_granted",
		zap.String("plugin", pluginName),
		zap.String("permission", permissionName),
		zap.String("granted_by", grantedBy),
	)
	m.refreshPermissionCache(ctx, pluginName)
	return nil
}

// DenyPermission 撤销插件权限
func (m *Manager) DenyPermission(ctx context.Context, pluginName, permissionName, grantedBy string) error {
	now := time.Now()
	result := m.db.WithContext(ctx).
		Model(&PluginPermission{}).
		Where("plugin_name = ? AND permission_name = ?", pluginName, permissionName).
		Updates(map[string]interface{}{
			"status":     PermDenied,
			"revoked_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("permission %s not found for plugin %s", permissionName, pluginName)
	}

	m.logger.Info("plugin_permission_denied",
		zap.String("plugin", pluginName),
		zap.String("permission", permissionName),
		zap.String("denied_by", grantedBy),
	)
	m.refreshPermissionCache(ctx, pluginName)
	return nil
}

// GrantAllPermissions 全部授予
func (m *Manager) GrantAllPermissions(ctx context.Context, pluginName, grantedBy string) error {
	now := time.Now()
	result := m.db.WithContext(ctx).
		Model(&PluginPermission{}).
		Where("plugin_name = ? AND status != ?", pluginName, StatusUninstalled).
		Updates(map[string]interface{}{
			"status":     PermGranted,
			"granted_by": grantedBy,
			"granted_at": now,
		})
	if result.Error != nil {
		return result.Error
	}

	m.logger.Info("plugin_permission_grant_all",
		zap.String("plugin", pluginName),
		zap.String("granted_by", grantedBy),
		zap.Int64("affected", result.RowsAffected),
	)
	m.refreshPermissionCache(ctx, pluginName)
	return nil
}

// DenyAllPermissions 全部撤销
func (m *Manager) DenyAllPermissions(ctx context.Context, pluginName, grantedBy string) error {
	now := time.Now()
	result := m.db.WithContext(ctx).
		Model(&PluginPermission{}).
		Where("plugin_name = ? AND status != ?", pluginName, StatusUninstalled).
		Updates(map[string]interface{}{
			"status":     PermDenied,
			"revoked_at": now,
		})
	if result.Error != nil {
		return result.Error
	}

	m.logger.Info("plugin_permission_deny_all",
		zap.String("plugin", pluginName),
		zap.String("denied_by", grantedBy),
		zap.Int64("affected", result.RowsAffected),
	)
	m.refreshPermissionCache(ctx, pluginName)
	return nil
}

// GetGrantedPermissions 从缓存获取已授予的权限列表
func (m *Manager) GetGrantedPermissions(pluginName string) []string {
	m.permCacheMu.RLock()
	defer m.permCacheMu.RUnlock()
	if perms, ok := m.permCache[pluginName]; ok {
		result := make([]string, len(perms))
		copy(result, perms)
		return result
	}
	return nil
}

// CheckRequiredPermissions 检查插件是否有未满足的必需权限
func (m *Manager) CheckRequiredPermissions(pluginName string) (missing []string, err error) {
	var denied []PluginPermission
	if err := m.db.Where("plugin_name = ? AND required = ? AND status = ?", pluginName, true, PermDenied).
		Find(&denied).Error; err != nil {
		return nil, err
	}
	for _, p := range denied {
		missing = append(missing, p.PermissionName)
	}
	return missing, nil
}

// refreshPermissionCache 刷新指定插件的权限缓存
func (m *Manager) refreshPermissionCache(ctx context.Context, pluginName string) {
	var perms []PluginPermission
	m.db.WithContext(ctx).
		Where("plugin_name = ? AND status = ?", pluginName, PermGranted).
		Find(&perms)

	granted := make([]string, 0, len(perms))
	for _, p := range perms {
		granted = append(granted, p.PermissionName)
	}

	m.permCacheMu.Lock()
	m.permCache[pluginName] = granted
	m.permCacheMu.Unlock()
}

// filterHookRequest 根据 granted 权限列表过滤 HookRequest 中的字段
// 如果 granted 为 nil（插件无权限声明），则不进行过滤（向后兼容）
func (m *Manager) filterHookRequest(req *HookRequest, granted []string) *HookRequest {
	if granted == nil {
		// 无权限声明，照原样传递
		return req
	}

	// 构建快速查找 map
	grantedSet := make(map[string]bool, len(granted))
	for _, p := range granted {
		grantedSet[p] = true
	}

	// 复制请求，避免修改原始数据
	filtered := *req

	// account_id
	if !grantedSet[string(PermAccountID)] {
		filtered.AccountID = 0
	}
	// channel_id
	if !grantedSet[string(PermChannelID)] {
		filtered.ChannelID = 0
	}
	// keys_id + keys_name
	if !grantedSet[string(PermKeysID)] {
		filtered.KeysID = 0
		filtered.KeysName = ""
	}
	// model_name
	if !grantedSet[string(PermModelName)] {
		filtered.Model = ""
	}
	// request_headers
	if !grantedSet[string(PermRequestHeaders)] && filtered.Request != nil {
		filtered.Request = &HookRequestBody{}
		if grantedSet[string(PermRequestBodySummary)] {
			filtered.Request.Body = req.Request.Body
		}
	}
	// request_body_summary
	if !grantedSet[string(PermRequestBodySummary)] && filtered.Request != nil {
		if filtered.Request == req.Request {
			// Request 未被上方复制过，需要浅拷贝
			clonedReq := *req.Request
			filtered.Request = &clonedReq
		}
		filtered.Request.Body = nil
	}
	// response_status
	if !grantedSet[string(PermResponseStatus)] && filtered.Response != nil {
		filtered.Response = &HookResponseBody{}
		if grantedSet[string(PermResponseBodySummary)] {
			filtered.Response.Body = req.Response.Body
		}
	}
	// response_body_summary
	if !grantedSet[string(PermResponseBodySummary)] && filtered.Response != nil {
		if filtered.Response == req.Response {
			clonedResp := *req.Response
			filtered.Response = &clonedResp
		}
		filtered.Response.Body = nil
	}
	// candidate_accounts — 归入 account_id 权限
	if !grantedSet[string(PermAccountID)] {
		filtered.CandidateAccounts = nil
	}

	return &filtered
}

// IsPermissionHighSensitive 判断权限是否是高敏感
func IsPermissionHighSensitive(permName string) bool {
	return HighSensitivePermissions[PermissionName(permName)]
}

// seedHooks 种子钩子数据（系统启动时自动执行，幂等）
func (m *Manager) seedHooks() error {
	hooks := []Hook{
		{
			Name:        "connection_decorator",
			HookType:    "request",
			Description: "在建立上游 TLS 连接前介入，允许插件代理/修改 TCP 连接",
			Timeout:     10000,
			Enabled:     true,
		},
		{
			Name:        "pre_request",
			HookType:    "request",
			Description: "请求转发前触发，可修改请求体/拒绝请求",
			Timeout:     5000,
			Enabled:     true,
		},
		{
			Name:        "post_response",
			HookType:    "request",
			Description: "收到上游响应后触发，可修改响应",
			Timeout:     5000,
			Enabled:     true,
		},
		{
			Name:        "account_select",
			HookType:    "request",
			Description: "账号选择时触发，可过滤候选账号",
			Timeout:     5000,
			Enabled:     true,
		},
		{
			Name:        "on_log",
			HookType:    "lifecycle",
			Description: "日志写入时触发",
			Timeout:     3000,
			Enabled:     true,
		},
	}

	for _, h := range hooks {
		if err := m.db.Where("name = ?", h.Name).FirstOrCreate(&h).Error; err != nil {
			return fmt.Errorf("seed hook %s: %w", h.Name, err)
		}
	}
	return nil
}

// ========== 阶段四新增：钩子管理 API ==========

// ListHooks 列出所有预埋钩子
func (m *Manager) ListHooks(ctx context.Context) ([]Hook, error) {
	var hooks []Hook
	err := m.db.WithContext(ctx).Order("id ASC").Find(&hooks).Error
	return hooks, err
}

// UpdateHookEnabled 启用/禁用钩子
func (m *Manager) UpdateHookEnabled(ctx context.Context, hookID uint, enabled bool) error {
	return m.db.WithContext(ctx).Model(&Hook{}).Where("id = ?", hookID).Update("enabled", enabled).Error
}

// GetHookPlugins 获取某钩子下已注册的插件列表
func (m *Manager) GetHookPlugins(ctx context.Context, hookName string) ([]Plugin, error) {
	var plugins []Plugin
	err := m.db.WithContext(ctx).
		Where("status = ? AND hooks LIKE ?", StatusRunning, "%\""+hookName+"\"%").
		Order("priority ASC").
		Find(&plugins).Error
	return plugins, err
}

// UpdatePluginPriority 更新插件优先级（DB + 内存注册表）
func (m *Manager) UpdatePluginPriority(ctx context.Context, pluginID uint, priority int) error {
	if err := m.db.WithContext(ctx).Model(&Plugin{}).Where("id = ?", pluginID).
		Update("priority", priority).Error; err != nil {
		return err
	}
	var p Plugin
	if err := m.db.First(&p, pluginID).Error; err == nil {
		m.registry.updatePriority(p.Name, priority)
	}
	return nil
}

// GetPluginTables 获取插件创建的表名清单
func (m *Manager) GetPluginTables(ctx context.Context, pluginID uint) ([]string, error) {
	var p Plugin
	if err := m.db.WithContext(ctx).First(&p, pluginID).Error; err != nil {
		return nil, err
	}
	var tables []string
	json.Unmarshal([]byte(p.TablesCreated), &tables)
	return tables, nil
}

// buildCreateTableSQL 根据 ManifestTable 生成 CREATE TABLE SQL（SQLite 方言）
func buildCreateTableSQL(tableName string, t ManifestTable) string {
	var b strings.Builder
	b.WriteString("CREATE TABLE IF NOT EXISTS ")
	b.WriteString(tableName)
	b.WriteString(" (")

	for i, col := range t.Columns {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(col.Name)
		b.WriteString(" ")
		b.WriteString(col.Type)

		if col.PrimaryKey {
			b.WriteString(" PRIMARY KEY")
		}
		if col.AutoIncrement {
			b.WriteString(" AUTOINCREMENT")
		}
		if col.NotNull {
			b.WriteString(" NOT NULL")
		}
		if col.Unique {
			b.WriteString(" UNIQUE")
		}
		if col.Default != "" {
			b.WriteString(" DEFAULT ")
			b.WriteString(col.Default)
		}
	}

	for _, idx := range t.Indexes {
		b.WriteString(", ")
		if idx.Unique {
			b.WriteString("UNIQUE ")
		}
		b.WriteString("INDEX ")
		b.WriteString(idx.Name)
		b.WriteString(" (")
		b.WriteString(strings.Join(idx.Columns, ", "))
		b.WriteString(")")
	}

	b.WriteString(")")
	return b.String()
}
