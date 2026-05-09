package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
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
	Port     int    `mapstructure:"port"`
	Host     string `mapstructure:"host"`
	Mode     string `mapstructure:"mode"`      // debug / release
	APIToken string `mapstructure:"api_token"` // 管理端认证 Token
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
	AffinityTTL                 int     `mapstructure:"affinity_ttl"`
	ConsecutiveFailureThreshold int     `mapstructure:"consecutive_failure_threshold"`
	MinDisableDuration          int     `mapstructure:"min_disable_duration"`
	ProbeInterval               int     `mapstructure:"probe_interval"`
	ProbeActiveRatioThreshold   float64 `mapstructure:"probe_active_ratio_threshold"`
	MaxProbeFailures            int     `mapstructure:"max_probe_failures"`
	MaxProbeRecoverPerCycle     int     `mapstructure:"max_probe_recover_per_cycle"`
	ProbeCooldownDuration       int     `mapstructure:"probe_cooldown_duration"`
	ProbeCooldownDurationL2     int     `mapstructure:"probe_cooldown_duration_l2"`
	GlobalHealthCheckInterval   int     `mapstructure:"global_health_check_interval"`
	AccountStatusCacheTTL       int     `mapstructure:"account_status_cache_ttl"`
	AccountKeyCacheTTL          int     `mapstructure:"account_key_cache_ttl"`
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
	ConnectTimeout  int `mapstructure:"connect_timeout"` // 秒
	ReadTimeout     int `mapstructure:"read_timeout"`    // 秒
	MaxIdleConns    int `mapstructure:"max_idle_conns"`
	IdleConnTimeout int `mapstructure:"idle_conn_timeout"` // 秒
}

type PluginConfig struct {
	PluginRegistryURL string `mapstructure:"plugin_registry_url"` // 插件注册中心 API 地址，为空时不启用
	UseRegistryAuth   bool   `mapstructure:"use_registry_auth"`   // 注册中心是否需要认证
	PluginDir         string `mapstructure:"plugin_dir"`          // 本地插件存储目录
	SidecarTimeout    int    `mapstructure:"sidecar_timeout"`     // Sidecar 钩子调用超时（秒）
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

	if cfg.Server.APIToken == "" {
		cfg.Server.APIToken = os.Getenv("AGW_SERVER_API_TOKEN")
	}

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
			"connect_timeout":  c.Proxy.ConnectTimeout,
			"read_timeout":     c.Proxy.ReadTimeout,
			"max_idle_conns":   c.Proxy.MaxIdleConns,
			"idle_conn_timeout": c.Proxy.IdleConnTimeout,
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
			"global_health_check_interval":   c.AccountManager.GlobalHealthCheckInterval,
			"account_status_cache_ttl":       c.AccountManager.AccountStatusCacheTTL,
			"account_key_cache_ttl":          c.AccountManager.AccountKeyCacheTTL,
		},
		"plugin": map[string]interface{}{
			"plugin_registry_url": c.Plugin.PluginRegistryURL,
			"use_registry_auth":   c.Plugin.UseRegistryAuth,
			"plugin_dir":          c.Plugin.PluginDir,
			"sidecar_timeout":     c.Plugin.SidecarTimeout,
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
			if v, ok := toInt(am["global_health_check_interval"]); ok {
				c.AccountManager.GlobalHealthCheckInterval = v
			}
			if v, ok := toInt(am["account_status_cache_ttl"]); ok {
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
	v.Set("account_manager.global_health_check_interval", c.AccountManager.GlobalHealthCheckInterval)
	v.Set("account_manager.account_status_cache_ttl", c.AccountManager.AccountStatusCacheTTL)
	v.Set("account_manager.account_key_cache_ttl", c.AccountManager.AccountKeyCacheTTL)

	v.Set("plugin.plugin_registry_url", c.Plugin.PluginRegistryURL)
	v.Set("plugin.use_registry_auth", c.Plugin.UseRegistryAuth)
	v.Set("plugin.plugin_dir", c.Plugin.PluginDir)
	v.Set("plugin.sidecar_timeout", c.Plugin.SidecarTimeout)

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
	v.SetDefault("account_manager.global_health_check_interval", 3600)
	v.SetDefault("account_manager.account_status_cache_ttl", 30)
	v.SetDefault("account_manager.account_key_cache_ttl", 60)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.dir", "logs")
	v.SetDefault("log.max_age_days", 30)
	v.SetDefault("log.detail_log_enabled", true)
	v.SetDefault("log.refresh_interval", 5)

	v.SetDefault("proxy.connect_timeout", 5)
	v.SetDefault("proxy.read_timeout", 60)
	v.SetDefault("proxy.max_idle_conns", 100)
	v.SetDefault("proxy.idle_conn_timeout", 90)

	v.SetDefault("plugin.plugin_registry_url", "")
	v.SetDefault("plugin.use_registry_auth", false)
	v.SetDefault("plugin.plugin_dir", "./plugins")
	v.SetDefault("plugin.sidecar_timeout", 5)
}

// GetSecretKey 获取加密密钥，优先环境变量
func GetSecretKey() string {
	if key := os.Getenv("SECRET_KEY"); key != "" {
		return key
	}
	return ""
}