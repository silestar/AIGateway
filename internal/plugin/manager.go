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
	"time"

	adapterregistry "github.com/silestar/AIGateway/pkg/adapter/registry"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Manager 插件管理器实现
type Manager struct {
	db        *gorm.DB
	logger    *zap.Logger
	pluginsDir string
	client    *http.Client
}

// NewManager 创建插件管理器
func NewManager(db *gorm.DB, logger *zap.Logger, pluginsDir string, sidecarTimeout int) *Manager {
	if pluginsDir == "" {
		pluginsDir = "plugins"
	}
	timeout := 5 * time.Second
	if sidecarTimeout > 0 {
		timeout = time.Duration(sidecarTimeout) * time.Second
	}
	return &Manager{
		db:         db,
		logger:     logger,
		pluginsDir: pluginsDir,
		client:     &http.Client{Timeout: timeout},
	}
}

// AutoMigrate 自动迁移
func (m *Manager) AutoMigrate() error {
	return m.db.AutoMigrate(&Plugin{}, &ChannelPluginSetting{})
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
	isSystemPlugin := manifest.Type == "system"
	binaryName, err := resolveBinaryName(&manifest)
	if err != nil {
		return nil, err // 架构不匹配，拒绝安装
	}

	// 4. 创建插件目录并解压（只解压匹配架构的二进制 + manifest）
	// System 类型插件不需要二进制文件，跳过解压
	pluginDir := filepath.Join(m.pluginsDir, manifest.Name)
	os.MkdirAll(pluginDir, 0755)

	binaryFound := isSystemPlugin // system 类型无需二进制
	if !isSystemPlugin {
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
	}

	if !binaryFound {
		return nil, fmt.Errorf("binary '%s' for current architecture (%s/%s) not found in zip", binaryName, runtime.GOOS, runtime.GOARCH)
	}

	// 5. 入库
	hooksJSON, _ := json.Marshal(manifest.Hooks)
	binaryPath := filepath.Join(pluginDir, binaryName)
	if isSystemPlugin {
		binaryPath = "builtin" // system 插件没有独立二进制
	}
	plugin := &Plugin{
		Name:         manifest.Name,
		Version:      manifest.Version,
		Description:  manifest.Description,
		Author:       manifest.Author,
		PluginType:   manifest.Type,
		Binary:       binaryPath,
		Port:         manifest.Port,
		Hooks:        string(hooksJSON),
		ConfigSchema: string(manifest.ConfigSchema),
		Manifest:     string(manifestData),
		Status:       StatusInstalled,
	}

	if err := m.db.WithContext(ctx).Create(plugin).Error; err != nil {
		return nil, fmt.Errorf("save plugin: %w", err)
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

	// System 类型插件已编译进主程序，无需独立进程，直接标记为 running
	if plugin.PluginType == "system" {
		m.db.WithContext(ctx).Model(&plugin).Updates(map[string]interface{}{
			"status": StatusRunning,
		})
		m.logger.Info("system plugin activated", zap.String("name", plugin.Name))
		return nil
	}

	// 启动子进程
	cmd := exec.CommandContext(ctx, plugin.Binary)
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
		healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", plugin.Port)
		resp, err := m.client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				healthOK = true
				break
			}
		}
	}
	if !healthOK {
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

	// System 类型插件没有独立进程，直接标记为 stopped
	if plugin.PluginType == "system" {
		m.db.WithContext(ctx).Model(&plugin).Updates(map[string]interface{}{
			"pid":    0,
			"status": StatusStopped,
		})
		m.logger.Info("system plugin deactivated", zap.String("name", plugin.Name))
		return nil
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

	// 删除目录（system 插件没有独立目录，跳过）
	if plugin.Binary != "builtin" {
		os.RemoveAll(filepath.Dir(plugin.Binary))
	}

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

		// HTTP 调用插件
		url := fmt.Sprintf("http://127.0.0.1:%d/hook/%s", p.Port, hook)
		body, _ := json.Marshal(req)
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
		url := fmt.Sprintf("http://127.0.0.1:%d/health", p.Port)
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
	// System 类型插件不需要独立二进制文件
	if m.Type == "system" {
		return "", nil
	}

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
