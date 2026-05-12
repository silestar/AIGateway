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
	"path/filepath"
	"runtime"
	"sync"
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
	client          *http.Client
	autoGrantPerms  bool                    // 自动授权所有权限
	permCache       map[string][]string     // plugin_name → 已授予权限列表
	permCacheMu     sync.RWMutex
}

// NewManager 创建插件管理器
func NewManager(db *gorm.DB, logger *zap.Logger, pluginsDir string, sidecarTimeout int, autoGrant bool) *Manager {
	if pluginsDir == "" {
		pluginsDir = "plugins"
	}
	// 确保转为绝对路径，避免 exec.Command 找不到二进制
	if absDir, err := filepath.Abs(pluginsDir); err == nil {
		pluginsDir = absDir
	}
	timeout := 5 * time.Second
	if sidecarTimeout > 0 {
		timeout = time.Duration(sidecarTimeout) * time.Second
	}
	return &Manager{
		db:             db,
		logger:         logger,
		pluginsDir:     pluginsDir,
		client:         &http.Client{Timeout: timeout},
		autoGrantPerms: autoGrant,
		permCache:      make(map[string][]string),
	}
}

// AutoMigrate 自动迁移
func (m *Manager) AutoMigrate() error {
	return m.db.AutoMigrate(&Plugin{}, &ChannelPluginSetting{}, &PluginPermission{})
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

	// 启动子进程（使用 Background context，避免请求结束后 context cancel 杀死子进程）
	cmd := exec.CommandContext(context.Background(), plugin.Binary)
	cmd.Env = append(os.Environ(),
		"PLUGIN_AUTH_TOKEN="+plugin.AuthToken,
		fmt.Sprintf("PLUGIN_PORT=%d", plugin.Port),
		"PLUGIN_CONFIG="+plugin.Config,
	)
	cmd.Dir = filepath.Dir(plugin.Binary)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start plugin process: %w", err)
	}

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

// Uninstall 卸载插件
func (m *Manager) Uninstall(ctx context.Context, pluginID uint) error {
	// 先停止
	m.Stop(ctx, pluginID)

	var plugin Plugin
	if err := m.db.WithContext(ctx).First(&plugin, pluginID).Error; err != nil {
		return fmt.Errorf("find plugin: %w", err)
	}

	// 删除目录
	os.RemoveAll(filepath.Dir(plugin.Binary))

	// 标记权限记录为 uninstalled（保留用于审计）
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

	m.logger.Info("plugin uninstalled", zap.String("name", plugin.Name))
	return nil
}

// TriggerHook 触发钩子
func (m *Manager) TriggerHook(ctx context.Context, hook HookName, req *HookRequest) (*HookResponse, error) {
	// 查找订阅该钩子的运行中插件
	var plugins []Plugin
	m.db.WithContext(ctx).Where("status = ?", StatusRunning).Find(&plugins)

	for _, p := range plugins {
		var hooks []string
		json.Unmarshal([]byte(p.Hooks), &hooks)

		subscribed := false
		for _, h := range hooks {
			if h == string(hook) {
				subscribed = true
				break
			}
		}
		if !subscribed {
			continue
		}

		// 根据权限过滤 HookRequest
		granted := m.GetGrantedPermissions(p.Name)
		filteredReq := m.filterHookRequest(req, granted)

		// HTTP 调用插件
		url := fmt.Sprintf("http://127.0.0.1:%d/hook/%s", p.Port, hook)
		body, _ := json.Marshal(filteredReq)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			m.logger.Warn("create hook request failed", zap.String("plugin", p.Name), zap.Error(err))
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.AuthToken)

		resp, err := m.client.Do(httpReq)
		if err != nil {
			m.logger.Warn("call plugin hook failed", zap.String("plugin", p.Name), zap.Error(err))
			continue
		}

		var hookResp HookResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&hookResp)
		resp.Body.Close() // 循环内手动关闭，不用 defer
		if decodeErr != nil {
			m.logger.Warn("decode hook response failed", zap.String("plugin", p.Name), zap.Error(decodeErr))
			continue
		}

		// 如果插件拒绝，直接返回拒绝
		if hookResp.Action == ActionReject {
			return &hookResp, nil
		}

		// 对于 account_select，合并 exclude_ids
		if hook == HookAccountSelect && hookResp.Action == ActionFilter {
			return &hookResp, nil
		}

		// 对于 pre_request / post_response，如果修改了请求/响应，更新 req
		if hookResp.ModifiedRequest != nil {
			req.Request = hookResp.ModifiedRequest
		}
		if hookResp.ModifiedResponse != nil {
			req.Response = hookResp.ModifiedResponse
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

// GetConnectionDecoratorAddr 查询指定渠道启用的 connection_decorator 插件地址
// 返回 "127.0.0.1:{port}" 格式，如果没有则返回空字符串
func (m *Manager) GetConnectionDecoratorAddr(channelID uint) string {
	if channelID == 0 {
		return ""
	}

	// 查找 hooks 包含 connection_decorator 且 status=running 的插件
	var plugins []Plugin
	m.db.Where("status = ? AND hooks LIKE ?", StatusRunning, "%connection_decorator%").Find(&plugins)
	if len(plugins) == 0 {
		return ""
	}

	// 遍历找到该渠道已启用的插件
	for _, p := range plugins {
		var setting ChannelPluginSetting
		err := m.db.Where("channel_id = ? AND plugin_id = ?", channelID, p.ID).First(&setting).Error
		if err != nil {
			continue // 没有渠道级配置 → 跳过
		}

		// 解析渠道配置，检查 enabled
		var cfg struct {
			Enabled bool `json:"enabled"`
		}
		if json.Unmarshal([]byte(setting.Config), &cfg) == nil && cfg.Enabled {
			return fmt.Sprintf("127.0.0.1:%d", p.Port)
		}
	}

	return ""
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

// GetChannelPluginConfig 获取渠道级插件配置，没有则返回空字符串
func (m *Manager) GetChannelPluginConfig(ctx context.Context, channelID, pluginID uint) (string, error) {
	var setting ChannelPluginSetting
	err := m.db.WithContext(ctx).Where("channel_id = ? AND plugin_id = ?", channelID, pluginID).First(&setting).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return setting.Config, nil
}

// SetChannelPluginConfig 设置渠道级插件配置
func (m *Manager) SetChannelPluginConfig(ctx context.Context, channelID, pluginID uint, config string) error {
	var setting ChannelPluginSetting
	err := m.db.WithContext(ctx).Where("channel_id = ? AND plugin_id = ?", channelID, pluginID).First(&setting).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		return m.db.WithContext(ctx).Create(&ChannelPluginSetting{
			ChannelID: channelID,
			PluginID:  pluginID,
			Config:    config,
		}).Error
	}
	return m.db.WithContext(ctx).Model(&setting).Update("config", config).Error
}

// DeleteChannelPluginConfig 删除渠道级插件配置
func (m *Manager) DeleteChannelPluginConfig(ctx context.Context, channelID, pluginID uint) error {
	return m.db.WithContext(ctx).Where("channel_id = ? AND plugin_id = ?", channelID, pluginID).Delete(&ChannelPluginSetting{}).Error
}

// ListChannelPluginConfigs 列出某插件的所有渠道级配置
func (m *Manager) ListChannelPluginConfigs(ctx context.Context, pluginID uint) ([]ChannelPluginSetting, error) {
	var settings []ChannelPluginSetting
	err := m.db.WithContext(ctx).Where("plugin_id = ?", pluginID).Find(&settings).Error
	return settings, err
}

// GetEffectiveConfig 获取插件在某渠道的生效配置（渠道级覆盖全局）
func (m *Manager) GetEffectiveConfig(ctx context.Context, channelID, pluginID uint) (string, error) {
	// 先查渠道级配置
	channelConfig, err := m.GetChannelPluginConfig(ctx, channelID, pluginID)
	if err != nil {
		return "", err
	}
	if channelConfig != "" {
		return channelConfig, nil
	}
	// 没有渠道级配置，用全局配置
	var plugin Plugin
	if err := m.db.WithContext(ctx).First(&plugin, pluginID).Error; err != nil {
		return "", err
	}
	return plugin.Config, nil
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
