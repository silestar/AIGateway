package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config 全局配置结构体
type Config struct {
	mu             sync.RWMutex
	Server         ServerConfig         `mapstructure:"server"`
	DB             DBConfig             `mapstructure:"db"`
	Redis          RedisConfig          `mapstructure:"redis"`
	AccountManager AccountManagerConfig `mapstructure:"account_manager"`
	Log            LogConfig            `mapstructure:"log"`
	Proxy          ProxyConfig          `mapstructure:"proxy"`
	Plugin         PluginConfig         `mapstructure:"plugin"`
}

type ServerConfig struct {
	Port      int    `mapstructure:"port"`
	Host      string `mapstructure:"host"`
	Mode      string `mapstructure:"mode"`       // debug / release
	AdminUser string `mapstructure:"admin_user"` // 管理端用户名
	AdminPass string `mapstructure:"admin_pass"` // 管理端密码
}

type DBConfig struct {
	Type     string `mapstructure:"type"` // sqlite / mysql / postgres
	Path     string `mapstructure:"path"` // SQLite 文件路径
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
}

type RedisConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Password   string `mapstructure:"password"`
	DB         int    `mapstructure:"db"`
	MaxRetries int    `mapstructure:"max_retries"`
}

type AccountManagerConfig struct {
	AffinityTTL                 int      `mapstructure:"affinity_ttl"`
	ConsecutiveFailureThreshold int      `mapstructure:"consecutive_failure_threshold"`
	MinDisableDuration          int      `mapstructure:"min_disable_duration"`
	ProbeInterval               int      `mapstructure:"probe_interval"`
	ProbeActiveRatioThreshold   float64  `mapstructure:"probe_active_ratio_threshold"`
	MaxProbeFailures            int      `mapstructure:"max_probe_failures"`
	MaxProbeRecoverPerCycle     int      `mapstructure:"max_probe_recover_per_cycle"`
	ProbeCooldownDuration       int      `mapstructure:"probe_cooldown_duration"`
	ProbeCooldownDurationL2     int      `mapstructure:"probe_cooldown_duration_l2"`
	AccountStatusCacheTTL       int      `mapstructure:"account_status_cache_ttl"`
	AccountKeyCacheTTL          int      `mapstructure:"account_key_cache_ttl"`

	// 新增：渠道监控与自动处置
	ChannelHealthCheckInterval    int      `mapstructure:"channel_health_check_interval"`     // 全量健康检查间隔（秒），默认 43200（12小时）
	ChannelDisableLatencyThreshold int     `mapstructure:"channel_disable_latency_threshold"` // 响应时间超此值自动禁用（秒），0=不限制
	ChannelDisableOnFailure        bool    `mapstructure:"channel_disable_on_failure"`        // 测试失败时累积失败次数
	ChannelDisableStatusCodes      []int   `mapstructure:"channel_disable_status_codes"`      // 立即禁用账号的状态码
	ChannelRetryStatusCodes        []int   `mapstructure:"channel_retry_status_codes"`        // 触发重试的状态码
	ChannelDisableKeywords         []string `mapstructure:"channel_disable_keywords"`          // 立即禁用账号的关键词
	FailureExcludeKeywords          []string `mapstructure:"failure_exclude_keywords"`           // 不计入连续失败的错误关键词
}

type LogConfig struct {
	Level            string `mapstructure:"level"`              // debug / info / warn / error
	Dir              string `mapstructure:"dir"`                // 日志目录
	MaxAgeDays       int    `mapstructure:"max_age_days"`       // 保留天数
	MaxSizeMB        int    `mapstructure:"max_size_mb"`        // 单文件最大MB（0=不限制）
	DetailLogEnabled bool   `mapstructure:"detail_log_enabled"` // 是否记录详细请求内容文件
	RefreshInterval  int    `mapstructure:"refresh_interval"`   // 请求日志实时追踪刷新间隔(秒)
}

type ProxyConfig struct {
	ConnectTimeout    int `mapstructure:"connect_timeout"`     // 秒
	ReadTimeout       int `mapstructure:"read_timeout"`          // 秒（非流式请求整体超时）
	StreamReadTimeout int `mapstructure:"stream_read_timeout"`  // 秒（流式读取 chunk 间的最大等待时间，0=不限）
	MaxIdleConns      int `mapstructure:"max_idle_conns"`
	IdleConnTimeout   int `mapstructure:"idle_conn_timeout"` // 秒
}

type PluginConfig struct {
	PluginRegistryURL   string `mapstructure:"plugin_registry_url"`    // 插件注册中心 API 地址，为空时不启用
	UseRegistryAuth     bool   `mapstructure:"use_registry_auth"`      // 注册中心是否需要认证
	PluginDir           string `mapstructure:"plugin_dir"`              // 本地插件存储目录
	SidecarTimeout      int    `mapstructure:"sidecar_timeout"`         // Sidecar 钩子调用超时（秒）
	AutoGrantPermissions bool  `mapstructure:"auto_grant_permissions"` // 自动授予所有插件权限（开发/自用场景）
}

// Load 加载配置：config.yaml → .env → 环境变量，后者覆盖前者
func Load(configPath string) (*Config, error) {
	_ = godotenv.Load("./config/.env")

	v := viper.New()
	setDefaults(v)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	v.SetEnvPrefix("AGW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// 兼容旧字段 global_health_check_interval → channel_health_check_interval
	// 已移到 migration.go 的 migrateConfigFields()，此处移除
	_ = v.Get("account_manager.global_health_check_interval") // 保留 Get 用于触发 viper 读取

	if cfg.Server.AdminUser == "" {
		cfg.Server.AdminUser = os.Getenv("AGW_ADMIN_USER")
	}
	if cfg.Server.AdminPass == "" {
		cfg.Server.AdminPass = os.Getenv("AGW_ADMIN_PASS")
	}

	// 启动时检查并补全缺失的配置项到 config.yaml
	// 注意：此时 logger 还未初始化，传 nil 静默处理
	actualConfigPath := configPath
	if actualConfigPath == "" {
		actualConfigPath = v.ConfigFileUsed()
	}
	EnsureConfigCompleteness(actualConfigPath, v, nil)

	return &cfg, nil
}

// GetHotReloadableConfig 返回所有可热加载的配置项（排除密钥、加密、DB、Redis）
func (c *Config) GetHotReloadableConfig() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"server": map[string]interface{}{
			"mode": c.Server.Mode,
		},
		"log": map[string]interface{}{
			"level":              c.Log.Level,
			"dir":                c.Log.Dir,
			"max_age_days":       c.Log.MaxAgeDays,
			"detail_log_enabled": c.Log.DetailLogEnabled,
			"refresh_interval":   c.Log.RefreshInterval,
		},
		"proxy": map[string]interface{}{
			"connect_timeout":     c.Proxy.ConnectTimeout,
			"read_timeout":        c.Proxy.ReadTimeout,
			"stream_read_timeout": c.Proxy.StreamReadTimeout,
			"max_idle_conns":      c.Proxy.MaxIdleConns,
			"idle_conn_timeout":   c.Proxy.IdleConnTimeout,
		},
		"account_manager": map[string]interface{}{
			"affinity_ttl":                   c.AccountManager.AffinityTTL,
			"consecutive_failure_threshold":  c.AccountManager.ConsecutiveFailureThreshold,
			"min_disable_duration":           c.AccountManager.MinDisableDuration,
			"probe_interval":                 c.AccountManager.ProbeInterval,
			"probe_active_ratio_threshold":   c.AccountManager.ProbeActiveRatioThreshold,
			"max_probe_failures":             c.AccountManager.MaxProbeFailures,
			"max_probe_recover_per_cycle":    c.AccountManager.MaxProbeRecoverPerCycle,
			"probe_cooldown_duration":        c.AccountManager.ProbeCooldownDuration,
			"probe_cooldown_duration_l2":     c.AccountManager.ProbeCooldownDurationL2,
			"channel_health_check_interval":    c.AccountManager.ChannelHealthCheckInterval,
			"channel_disable_latency_threshold": c.AccountManager.ChannelDisableLatencyThreshold,
			"channel_disable_on_failure":        c.AccountManager.ChannelDisableOnFailure,
		"channel_disable_status_codes":       c.AccountManager.ChannelDisableStatusCodes,
			"channel_retry_status_codes":         c.AccountManager.ChannelRetryStatusCodes,
		"channel_disable_keywords":           c.AccountManager.ChannelDisableKeywords,
		"failure_exclude_keywords":           c.AccountManager.FailureExcludeKeywords,
		"account_status_cache_ttl":       c.AccountManager.AccountStatusCacheTTL,
			"account_key_cache_ttl":          c.AccountManager.AccountKeyCacheTTL,
		},
		"plugin": map[string]interface{}{
			"plugin_registry_url":    c.Plugin.PluginRegistryURL,
			"use_registry_auth":      c.Plugin.UseRegistryAuth,
			"plugin_dir":             c.Plugin.PluginDir,
			"sidecar_timeout":        c.Plugin.SidecarTimeout,
			"auto_grant_permissions": c.Plugin.AutoGrantPermissions,
		},
	}
}

// UpdateHotReloadableConfig 热更新可修改的配置项并写回 config.yaml
func (c *Config) UpdateHotReloadableConfig(updates map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 逐项更新内存
	if serverRaw, ok := updates["server"]; ok {
		if server, ok := serverRaw.(map[string]interface{}); ok {
			if v, ok := server["mode"].(string); ok {
				c.Server.Mode = v
			}
		}
	}

	if logRaw, ok := updates["log"]; ok {
		if logMap, ok := logRaw.(map[string]interface{}); ok {
			if v, ok := logMap["level"].(string); ok {
				c.Log.Level = v
			}
			if v, ok := logMap["dir"].(string); ok {
				c.Log.Dir = v
			}
			if v, ok := toInt(logMap["max_age_days"]); ok {
				c.Log.MaxAgeDays = v
			}
			if v, ok := logMap["detail_log_enabled"].(bool); ok {
				c.Log.DetailLogEnabled = v
			}
			if v, ok := toInt(logMap["refresh_interval"]); ok {
				c.Log.RefreshInterval = v
			}
		}
	}

	if proxyRaw, ok := updates["proxy"]; ok {
		if proxyMap, ok := proxyRaw.(map[string]interface{}); ok {
			if v, ok := toInt(proxyMap["connect_timeout"]); ok {
				c.Proxy.ConnectTimeout = v
			}
			if v, ok := toInt(proxyMap["read_timeout"]); ok {
				c.Proxy.ReadTimeout = v
			}
			if v, ok := toInt(proxyMap["stream_read_timeout"]); ok {
				c.Proxy.StreamReadTimeout = v
			}
			if v, ok := toInt(proxyMap["max_idle_conns"]); ok {
				c.Proxy.MaxIdleConns = v
			}
			if v, ok := toInt(proxyMap["idle_conn_timeout"]); ok {
				c.Proxy.IdleConnTimeout = v
			}
		}
	}

	if amRaw, ok := updates["account_manager"]; ok {
		if am, ok := amRaw.(map[string]interface{}); ok {
			if v, ok := toInt(am["affinity_ttl"]); ok {
				c.AccountManager.AffinityTTL = v
			}
			if v, ok := toInt(am["consecutive_failure_threshold"]); ok {
				c.AccountManager.ConsecutiveFailureThreshold = v
			}
			if v, ok := toInt(am["min_disable_duration"]); ok {
				c.AccountManager.MinDisableDuration = v
			}
			if v, ok := toInt(am["probe_interval"]); ok {
				c.AccountManager.ProbeInterval = v
			}
			if v, ok := toFloat64(am["probe_active_ratio_threshold"]); ok {
				c.AccountManager.ProbeActiveRatioThreshold = v
			}
			if v, ok := toInt(am["max_probe_failures"]); ok {
				c.AccountManager.MaxProbeFailures = v
			}
			if v, ok := toInt(am["max_probe_recover_per_cycle"]); ok {
				c.AccountManager.MaxProbeRecoverPerCycle = v
			}
			if v, ok := toInt(am["probe_cooldown_duration"]); ok {
				c.AccountManager.ProbeCooldownDuration = v
			}
			if v, ok := toInt(am["probe_cooldown_duration_l2"]); ok {
				c.AccountManager.ProbeCooldownDurationL2 = v
			}
			if v, ok := toInt(am["channel_health_check_interval"]); ok {
				c.AccountManager.ChannelHealthCheckInterval = v
			}
			if v, ok := toInt(am["channel_disable_latency_threshold"]); ok {
				c.AccountManager.ChannelDisableLatencyThreshold = v
			}
			if v, ok := am["channel_disable_on_failure"].(bool); ok {
				c.AccountManager.ChannelDisableOnFailure = v
			}
		if v, ok := toIntSlice(am["channel_disable_status_codes"]);ok {
				c.AccountManager.ChannelDisableStatusCodes = v
			}
			if v, ok := toIntSlice(am["channel_retry_status_codes"]); ok {
				c.AccountManager.ChannelRetryStatusCodes = v
			}
		if v, ok := toStringSlice(am["channel_disable_keywords"]); ok {
			c.AccountManager.ChannelDisableKeywords = v
		}
		if v, ok := toStringSlice(am["failure_exclude_keywords"]); ok {
			c.AccountManager.FailureExcludeKeywords = v
		}
		if v, ok := toInt(am["account_status_cache_ttl"]);ok {
				c.AccountManager.AccountStatusCacheTTL = v
			}
			if v, ok := toInt(am["account_key_cache_ttl"]); ok {
				c.AccountManager.AccountKeyCacheTTL = v
			}
		}
	}

	if pluginRaw, ok := updates["plugin"]; ok {
		if pl, ok := pluginRaw.(map[string]interface{}); ok {
			if v, ok := pl["plugin_registry_url"].(string); ok {
				c.Plugin.PluginRegistryURL = v
			}
			if v, ok := pl["use_registry_auth"].(bool); ok {
				c.Plugin.UseRegistryAuth = v
			}
			if v, ok := pl["plugin_dir"].(string); ok {
				c.Plugin.PluginDir = v
			}
			if v, ok := toInt(pl["sidecar_timeout"]); ok {
				c.Plugin.SidecarTimeout = v
			}
			if v, ok := pl["auto_grant_permissions"].(bool); ok {
				c.Plugin.AutoGrantPermissions = v
			}
		}
	}

	// 写回 config.yaml
	return c.writeConfigFile()
}

// writeConfigFile 将当前配置写回 config.yaml
func (c *Config) writeConfigFile() error {
	v := viper.New()
	setDefaults(v)

	// 先读取现有文件以保留注释
	configPath := "config/config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.yaml"
	}
	v.SetConfigFile(configPath)
	_ = v.ReadInConfig()

	// 写入热加载项
	v.Set("server.mode", c.Server.Mode)
	v.Set("log.level", c.Log.Level)
	v.Set("log.dir", c.Log.Dir)
	v.Set("log.max_age_days", c.Log.MaxAgeDays)
	v.Set("log.detail_log_enabled", c.Log.DetailLogEnabled)
	v.Set("log.refresh_interval", c.Log.RefreshInterval)
	v.Set("proxy.connect_timeout", c.Proxy.ConnectTimeout)
	v.Set("proxy.read_timeout", c.Proxy.ReadTimeout)
	v.Set("proxy.stream_read_timeout", c.Proxy.StreamReadTimeout)
	v.Set("proxy.max_idle_conns", c.Proxy.MaxIdleConns)
	v.Set("proxy.idle_conn_timeout", c.Proxy.IdleConnTimeout)
	v.Set("account_manager.affinity_ttl", c.AccountManager.AffinityTTL)
	v.Set("account_manager.consecutive_failure_threshold", c.AccountManager.ConsecutiveFailureThreshold)
	v.Set("account_manager.min_disable_duration", c.AccountManager.MinDisableDuration)
	v.Set("account_manager.probe_interval", c.AccountManager.ProbeInterval)
	v.Set("account_manager.probe_active_ratio_threshold", c.AccountManager.ProbeActiveRatioThreshold)
	v.Set("account_manager.max_probe_failures", c.AccountManager.MaxProbeFailures)
	v.Set("account_manager.max_probe_recover_per_cycle", c.AccountManager.MaxProbeRecoverPerCycle)
	v.Set("account_manager.probe_cooldown_duration", c.AccountManager.ProbeCooldownDuration)
	v.Set("account_manager.probe_cooldown_duration_l2", c.AccountManager.ProbeCooldownDurationL2)
	v.Set("account_manager.channel_health_check_interval", c.AccountManager.ChannelHealthCheckInterval)
	v.Set("account_manager.channel_disable_latency_threshold", c.AccountManager.ChannelDisableLatencyThreshold)
	v.Set("account_manager.channel_disable_on_failure", c.AccountManager.ChannelDisableOnFailure)
	v.Set("account_manager.channel_disable_status_codes", c.AccountManager.ChannelDisableStatusCodes)
	v.Set("account_manager.channel_retry_status_codes", c.AccountManager.ChannelRetryStatusCodes)
	v.Set("account_manager.channel_disable_keywords", c.AccountManager.ChannelDisableKeywords)
	v.Set("account_manager.failure_exclude_keywords", c.AccountManager.FailureExcludeKeywords)
	v.Set("account_manager.account_status_cache_ttl", c.AccountManager.AccountStatusCacheTTL)
	v.Set("account_manager.account_key_cache_ttl", c.AccountManager.AccountKeyCacheTTL)

	v.Set("plugin.plugin_registry_url", c.Plugin.PluginRegistryURL)
	v.Set("plugin.use_registry_auth", c.Plugin.UseRegistryAuth)
	v.Set("plugin.plugin_dir", c.Plugin.PluginDir)
	v.Set("plugin.sidecar_timeout", c.Plugin.SidecarTimeout)
	v.Set("plugin.auto_grant_permissions", c.Plugin.AutoGrantPermissions)

	return v.WriteConfig()
}

// toInt 安全地将 interface{} 转为 int（JSON number 可能是 float64）
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func toIntSlice(v interface{}) ([]int, bool) {
	switch arr := v.(type) {
	case []interface{}:
		result := make([]int, 0, len(arr))
		for _, item := range arr {
			if n, ok := toInt(item); ok {
				result = append(result, n)
			}
		}
		return result, true
	case []int:
		return arr, true
	}
	return nil, false
}

func toStringSlice(v interface{}) ([]string, bool) {
	switch arr := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result, true
	case []string:
		return arr, true
	}
	return nil, false
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 7860)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.mode", "debug")

	v.SetDefault("db.type", "sqlite")
	v.SetDefault("db.path", "data/agw.db")

	v.SetDefault("redis.enabled", false)
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.max_retries", 3)

	v.SetDefault("account_manager.affinity_ttl", 3600)
	v.SetDefault("account_manager.consecutive_failure_threshold", 5)
	v.SetDefault("account_manager.min_disable_duration", 120)
	v.SetDefault("account_manager.probe_interval", 30)
	v.SetDefault("account_manager.probe_active_ratio_threshold", 0.4)
	v.SetDefault("account_manager.max_probe_failures", 10)
	v.SetDefault("account_manager.max_probe_recover_per_cycle", 1)
	v.SetDefault("account_manager.probe_cooldown_duration", 7200)
	v.SetDefault("account_manager.probe_cooldown_duration_l2", 86400)
	v.SetDefault("account_manager.account_status_cache_ttl", 30)
	v.SetDefault("account_manager.account_key_cache_ttl", 60)

	// 新增：渠道监控与自动处置
	v.SetDefault("account_manager.channel_health_check_interval", 43200)
	v.SetDefault("account_manager.channel_disable_latency_threshold", 0)
	v.SetDefault("account_manager.channel_disable_on_failure", true)
	v.SetDefault("account_manager.channel_disable_status_codes", []int{401, 403, 429})
	v.SetDefault("account_manager.channel_retry_status_codes", []int{502, 503, 504})
	v.SetDefault("account_manager.channel_disable_keywords", []string{
		"Your credit balance is too low",
		"This organization has been disabled",
		"You exceeded your current quota",
		"Permission denied",
		"The security token included is invalid",
		"Operation not allowed",
		"Your account is not authorized",
		"you have reached the limit of the free model quota",
		"invalid_api_key",
		"account_deactivated",
		"Insufficient authentication scope",
	})
	v.SetDefault("account_manager.failure_exclude_keywords", []string{"context canceled"})

	// API 密钥加密盐值
	v.SetDefault("log.dir", "logs")
	v.SetDefault("log.max_age_days", 30)
	v.SetDefault("log.detail_log_enabled", true)
	v.SetDefault("log.refresh_interval", 5)

	v.SetDefault("proxy.connect_timeout", 5)
	v.SetDefault("proxy.read_timeout", 60)
	v.SetDefault("proxy.stream_read_timeout", 300)
	v.SetDefault("proxy.max_idle_conns", 100)
	v.SetDefault("proxy.idle_conn_timeout", 90)

	v.SetDefault("plugin.plugin_registry_url", "")
	v.SetDefault("plugin.use_registry_auth", false)
	v.SetDefault("plugin.plugin_dir", "./plugins")
	v.SetDefault("plugin.sidecar_timeout", 5)
	v.SetDefault("plugin.auto_grant_permissions", false)
}

// ensureConfigKeys 定义所有应在 config.yaml 中存在的配置项及其注释
var ensureConfigKeys = []struct {
	Key     string
	Comment string
}{
	// account_manager
	{"account_manager.affinity_ttl", "账号粘性绑定 TTL（秒）"},
	{"account_manager.consecutive_failure_threshold", "连续失败多少次后禁用账号"},
	{"account_manager.min_disable_duration", "账号最小禁用时长（秒）"},
	{"account_manager.probe_interval", "按需探测间隔（秒）"},
	{"account_manager.probe_active_ratio_threshold", "触发探测的活跃账号比例阈值"},
	{"account_manager.max_probe_failures", "探测最大失败次数"},
	{"account_manager.max_probe_recover_per_cycle", "每轮探测最大恢复数"},
	{"account_manager.probe_cooldown_duration", "探测冷却时长 L1（秒）"},
	{"account_manager.probe_cooldown_duration_l2", "探测冷却时长 L2（秒）"},
	{"account_manager.channel_health_check_interval", "全量健康检查间隔（秒），默认 43200（12小时）"},
	{"account_manager.channel_disable_latency_threshold", "响应时间超此值自动禁用（秒），0=不限制"},
	{"account_manager.channel_disable_on_failure", "测试失败时累积失败次数"},
	{"account_manager.failure_exclude_keywords", "不计入连续失败的错误关键词"},
	{"account_manager.channel_disable_status_codes", "立即禁用账号的状态码"},
	{"account_manager.account_key_cache_ttl", "账号密钥缓存 TTL（秒）"},
	// log
	{"log.level", "日志级别：debug/info/warn/error"},
	{"log.dir", "日志目录"},
	{"log.max_age_days", "日志保留天数"},
	{"log.detail_log_enabled", "是否记录详细请求内容"},
	{"log.refresh_interval", "请求日志刷新间隔（秒）"},
	// proxy
	{"proxy.connect_timeout", "连接超时（秒）"},
	{"proxy.read_timeout", "非流式请求整体超时（秒）"},
	{"proxy.stream_read_timeout", "流式读取 chunk 间最大等待（秒），0=不限"},
	{"proxy.max_idle_conns", "最大空闲连接数"},
	{"proxy.idle_conn_timeout", "空闲连接超时（秒）"},
	// plugin
	{"plugin.plugin_registry_url", "插件注册中心 URL"},
	{"plugin.use_registry_auth", "注册中心是否需要认证"},
	{"plugin.plugin_dir", "本地插件存储目录"},
	{"plugin.sidecar_timeout", "Sidecar 钩子调用超时（秒）"},
	{"plugin.auto_grant_permissions", "自动授予所有插件权限（开发/自用场景）"},
}

// EnsureConfigCompleteness 检查 config.yaml 中缺失的配置项并自动补全
// 不修改客户已有的值，只在对应 section 块内追加缺失的 key
func EnsureConfigCompleteness(configPath string, v *viper.Viper, logger *zap.Logger) {
	if configPath == "" {
		return
	}

	// 1. 读取文件全部内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		if logger != nil {
			logger.Warn("ensure config: cannot read config file", zap.Error(err))
		}
		return
	}
	lines := strings.Split(string(data), "\n")

	// 2. 解析现有 key：记录每个子项 key 属于哪个 section，以及是否已存在
	type yamlEntry struct {
		section string // 顶层 section 名（如 "account_manager"）
		subKey  string // 子项名（如 "channel_health_check_interval"）
	}
	existingEntries := make(map[yamlEntry]bool)

	currentSection := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 判断是否是顶层 section（无缩进，以冒号结尾或冒号后只有空格+值）
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if idx := strings.Index(trimmed, ":"); idx > 0 {
				key := strings.TrimSpace(trimmed[:idx])
				val := strings.TrimSpace(trimmed[idx+1:])
				if val == "" {
					// 顶层 section（如 "account_manager:"）
					currentSection = key
				}
				existingEntries[yamlEntry{section: currentSection, subKey: key}] = true
			}
		} else {
			// 缩进行，属于当前 section
			if idx := strings.Index(trimmed, ":"); idx > 0 {
				key := strings.TrimSpace(trimmed[:idx])
				existingEntries[yamlEntry{section: currentSection, subKey: key}] = true
			}
		}
	}

	// 3. 按 section 分组收集缺失项
	type missingItem struct {
		subKey  string
		comment string
		value   string
	}
	missingBySection := make(map[string][]missingItem) // section → 缺失项列表
	sectionOrder := []string{}                         // 保持 section 顺序

	for _, item := range ensureConfigKeys {
		parts := strings.SplitN(item.Key, ".", 2)
		section := ""
		subKey := ""
		if len(parts) == 2 {
			section = parts[0]
			subKey = parts[1]
		} else {
			subKey = parts[0]
		}

		if existingEntries[yamlEntry{section: section, subKey: subKey}] {
			continue
		}

		// 获取默认值
		defaultVal := v.Get(item.Key)
		valStr := formatYamlValue(defaultVal)

		if _, exists := missingBySection[section]; !exists {
			sectionOrder = append(sectionOrder, section)
		}
		missingBySection[section] = append(missingBySection[section], missingItem{
			subKey:  subKey,
			comment: item.Comment,
			value:   valStr,
		})
	}

	if len(missingBySection) == 0 {
		return
	}

	// 4. 在文件中对应 section 的末尾插入缺失项
	// 先找到每个顶层 section 的行范围
	type sectionRange struct {
		name      string
		startLine int // section 声明行
		endLine   int // section 最后一行（下一个顶层 section 之前）
	}
	var sections []sectionRange
	currentSec := ""
	secStart := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if idx := strings.Index(trimmed, ":"); idx > 0 {
				key := strings.TrimSpace(trimmed[:idx])
				val := strings.TrimSpace(trimmed[idx+1:])
				if val == "" && key != "" {
					// 新的顶层 section
					if currentSec != "" {
						sections = append(sections, sectionRange{name: currentSec, startLine: secStart, endLine: i - 1})
					}
					currentSec = key
					secStart = i
				}
			}
		}
	}
	// 最后一个 section
	if currentSec != "" {
		sections = append(sections, sectionRange{name: currentSec, startLine: secStart, endLine: len(lines) - 1})
	}
	// 更新每个 section 的真实 endLine（考虑尾部空行和注释行）
	for i := range sections {
		end := sections[i].endLine
		for end > sections[i].startLine {
			trimmed := strings.TrimSpace(lines[end])
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				break
			}
			end--
		}
		sections[i].endLine = end
	}

	sectionEndMap := make(map[string]int)
	for _, s := range sections {
		sectionEndMap[s.name] = s.endLine
	}

	// 5. 构建要插入的内容，按行号倒序插入（避免偏移）
	type insert struct {
		afterLine int
		content   string
	}
	var inserts []insert

	for _, secName := range sectionOrder {
		items, ok := missingBySection[secName]
		if !ok {
			continue
		}
		endLine := -1
		if secName != "" {
			endLine = sectionEndMap[secName]
		}
		// 找到最后一个非空行作为插入点
		if endLine == -1 {
			// 无 section 的顶层项，插到文件末尾
			endLine = len(lines) - 1
			for endLine > 0 && strings.TrimSpace(lines[endLine]) == "" {
				endLine--
			}
		}

		var sb strings.Builder
		sb.WriteString("\n")
		indent := ""
		if secName != "" {
			indent = "    "
		}
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("%s# [auto-added] %s\n", indent, item.comment))
			sb.WriteString(fmt.Sprintf("%s%s: %s\n", indent, item.subKey, item.value))
		}
		inserts = append(inserts, insert{afterLine: endLine, content: sb.String()})
	}

	// 倒序插入
	for i := len(inserts) - 1; i >= 0; i-- {
		ins := inserts[i]
		newLines := make([]string, 0, len(lines)+10)
		newLines = append(newLines, lines[:ins.afterLine+1]...)
		newLines = append(newLines, ins.content)
		newLines = append(newLines, lines[ins.afterLine+1:]...)
		lines = newLines
	}

	// 6. 写回文件
	output := strings.Join(lines, "\n")
	if err := os.WriteFile(configPath, []byte(output), 0644); err != nil {
		if logger != nil {
			logger.Warn("ensure config: cannot write config file", zap.Error(err))
		}
		return
	}

	if logger != nil {
		totalMissing := 0
		for _, items := range missingBySection {
			totalMissing += len(items)
		}
		logger.Info("auto-added missing config keys",
			zap.Int("count", totalMissing),
			zap.String("path", configPath),
		)
		for _, secName := range sectionOrder {
			for _, item := range missingBySection[secName] {
				fullKey := item.subKey
				if secName != "" {
					fullKey = secName + "." + item.subKey
				}
				logger.Info("  + " + fullKey)
			}
		}
	}
}

// formatYamlValue 将 Go 值格式化为 YAML 兼容的字符串
func formatYamlValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		if v == "" {
			return "\"\""
		}
		// 包含特殊字符则加引号
		if strings.ContainsAny(v, ":#&*?|<>{}[],!%@`") || v == "true" || v == "false" {
			return fmt.Sprintf("\"%s\"", v)
		}
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = fmt.Sprintf("- %s", formatYamlValue(item))
		}
		return "\n" + strings.Join(parts, "\n")
	case []string:
		if len(v) == 0 {
			return "[]"
		}
		parts := make([]string, len(v))
		for i, s := range v {
			parts[i] = fmt.Sprintf("- \"%s\"", s)
		}
		return "\n" + strings.Join(parts, "\n")
	case []int:
		if len(v) == 0 {
			return "[]"
		}
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = fmt.Sprintf("- %d", n)
		}
		return "\n" + strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// GetSecretKey 获取加密密钥，优先环境变量
func GetSecretKey() string {
	if key := os.Getenv("SECRET_KEY"); key != "" {
		return key
	}
	return ""
}