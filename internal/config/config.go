package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 全局配置结构体
type Config struct {
	Server         ServerConfig         `mapstructure:"server"`
	DB             DBConfig             `mapstructure:"db"`
	Redis          RedisConfig          `mapstructure:"redis"`
	AccountManager AccountManagerConfig `mapstructure:"account_manager"`
	Log            LogConfig            `mapstructure:"log"`
	Proxy          ProxyConfig          `mapstructure:"proxy"`
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
	Level      string `mapstructure:"level"` // debug / info / warn / error
	Dir        string `mapstructure:"dir"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
}

type ProxyConfig struct {
	ConnectTimeout  int `mapstructure:"connect_timeout"` // 秒
	ReadTimeout     int `mapstructure:"read_timeout"`    // 秒
	MaxIdleConns    int `mapstructure:"max_idle_conns"`
	IdleConnTimeout int `mapstructure:"idle_conn_timeout"` // 秒
}

// Load 加载配置：config.yaml → .env → 环境变量，后者覆盖前者
func Load(configPath string) (*Config, error) {
	// 加载 .env（忽略不存在的情况）
	_ = godotenv.Load("./config/.env")

	v := viper.New()

	// 设置默认值
	setDefaults(v)

	// 读取 config.yaml
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
		// 配置文件不存在，使用默认值
	}

	// 环境变量覆盖（AGW_ 前缀）
	v.SetEnvPrefix("AGW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// 手动读取 api_token（兼容 docker-compose 直接注入的情况）
	if cfg.Server.APIToken == "" {
		cfg.Server.APIToken = os.Getenv("AGW_SERVER_API_TOKEN")
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// server
	v.SetDefault("server.port", 7860)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.mode", "debug")

	// db
	v.SetDefault("db.type", "sqlite")
	v.SetDefault("db.path", "data/agw.db")

	// redis
	v.SetDefault("redis.enabled", false)
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.max_retries", 3)

	// account_manager（与 02-账号池设计文档一致）
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

	// log
	v.SetDefault("log.level", "info")
	v.SetDefault("log.dir", "logs")
	v.SetDefault("log.max_age_days", 30)

	// proxy
	v.SetDefault("proxy.connect_timeout", 5)
	v.SetDefault("proxy.read_timeout", 60)
	v.SetDefault("proxy.max_idle_conns", 100)
	v.SetDefault("proxy.idle_conn_timeout", 90)
}

// GetSecretKey 获取加密密钥，优先环境变量
func GetSecretKey() string {
	if key := os.Getenv("SECRET_KEY"); key != "" {
		return key
	}
	return ""
}