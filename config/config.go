package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Collector  CollectorConfig  `yaml:"collector"`
	Classifier ClassifierConfig `yaml:"classifier"`
	AI         AIConfig         `yaml:"ai"`
	Auth       AuthConfig       `yaml:"auth"`
	Log        LogConfig        `yaml:"log"`
	WeChat     WeChatConfig     `yaml:"wechat"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret        string        `yaml:"jwt_secret"`
	JWTExpiry        time.Duration `yaml:"jwt_expiry"`
	BcryptCost       int           `yaml:"bcrypt_cost"`
	MaxLoginAttempts int           `yaml:"max_login_attempts"`
	LockoutDuration  time.Duration `yaml:"lockout_duration"`
	RateLimitPerIP   int           `yaml:"rate_limit_per_ip"`
}

// AIConfig holds AI (LLM summarizer) settings.
type AIConfig struct {
	APIBase       string        `yaml:"api_base"`
	APIKey        string        `yaml:"api_key"`
	Model         string        `yaml:"model"`
	MaxConcurrent int           `yaml:"max_concurrent"`
	Timeout       time.Duration `yaml:"timeout"`
}

// IsEnabled returns true if the AI config has the minimum required fields set.
func (a AIConfig) IsEnabled() bool {
	key := a.APIKey
	if key == "" {
		key = os.Getenv("AI_API_KEY")
	}
	return key != "" && a.APIBase != ""
}

// GetAPIKey returns the API key, preferring the environment variable.
func (a AIConfig) GetAPIKey() string {
	if envKey := os.Getenv("AI_API_KEY"); envKey != "" {
		return envKey
	}
	return a.APIKey
}

// GetAPIBase returns the API base URL, preferring the environment variable.
func (a AIConfig) GetAPIBase() string {
	if v := os.Getenv("AI_API_BASE"); v != "" {
		return v
	}
	return a.APIBase
}

// GetModel returns the model name, preferring the environment variable.
func (a AIConfig) GetModel() string {
	if v := os.Getenv("AI_MODEL"); v != "" {
		return v
	}
	return a.Model
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Addr returns the listen address in "host:port" format.
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// CollectorConfig holds RSS/HTML collector settings.
type CollectorConfig struct {
	UserAgent     string        `yaml:"user_agent"`
	Timeout       time.Duration `yaml:"timeout"`
	MaxConcurrent int           `yaml:"max_concurrent"`
	RequestInterval string      `yaml:"request_interval"`
	RSSMaxItems    int         `yaml:"rss_max_items"`
}

// ClassifierConfig holds classifier settings.
type ClassifierConfig struct {
	RulesPath string `yaml:"rules_path"`
}

// WeChatConfig holds WeChat public account settings for article publishing.
type WeChatConfig struct {
	AppID     string `yaml:"appid"`
	Secret    string `yaml:"secret"`
	AccountID string `yaml:"account_id"`
}

// IsEnabled returns true if WeChat publishing is configured.
func (w WeChatConfig) IsEnabled() bool {
	return w.GetAppID() != "" && w.GetSecret() != ""
}

// GetAppID returns the AppID, preferring environment variable.
func (w WeChatConfig) GetAppID() string {
	if v := os.Getenv("WX_APPID"); v != "" {
		return v
	}
	return w.AppID
}

// GetSecret returns the Secret, preferring environment variable.
func (w WeChatConfig) GetSecret() string {
	if v := os.Getenv("WX_SECRET"); v != "" {
		return v
	}
	return w.Secret
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string `yaml:"level"`
}

// Load reads and parses the YAML config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	// Apply defaults.
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/news.db"
	}
	if cfg.Collector.UserAgent == "" {
		cfg.Collector.UserAgent = "ai-news-hub/1.0"
	}
	if cfg.Collector.Timeout == 0 {
		cfg.Collector.Timeout = 30 * time.Second
	}
	if cfg.Collector.MaxConcurrent == 0 {
		cfg.Collector.MaxConcurrent = 5
	}
	if cfg.Collector.RequestInterval == "" {
		cfg.Collector.RequestInterval = "2-5s"
	}
	if cfg.Collector.RSSMaxItems == 0 {
		cfg.Collector.RSSMaxItems = 20
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}

	// AI defaults
	if cfg.AI.APIBase == "" {
		cfg.AI.APIBase = "https://open.bigmodel.cn/api/paas/v4"
	}
	if cfg.AI.Model == "" {
		cfg.AI.Model = "glm-4-flash"
	}
	if cfg.AI.MaxConcurrent == 0 {
		cfg.AI.MaxConcurrent = 3
	}
	if cfg.AI.Timeout == 0 {
		cfg.AI.Timeout = 15 * time.Second
	}

	// Auth defaults
	if cfg.Auth.JWTExpiry == 0 {
		cfg.Auth.JWTExpiry = 168 * time.Hour
	}
	if cfg.Auth.BcryptCost == 0 {
		cfg.Auth.BcryptCost = 10
	}
	if cfg.Auth.MaxLoginAttempts == 0 {
		cfg.Auth.MaxLoginAttempts = 5
	}
	if cfg.Auth.LockoutDuration == 0 {
		cfg.Auth.LockoutDuration = 5 * time.Minute
	}
	if cfg.Auth.RateLimitPerIP == 0 {
		cfg.Auth.RateLimitPerIP = 10
	}

	// Environment variable overrides for auth
	if env := os.Getenv("JWT_SECRET"); env != "" {
		cfg.Auth.JWTSecret = env
	}

	return cfg, nil
}
