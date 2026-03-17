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
	Log        LogConfig        `yaml:"log"`
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
}

// ClassifierConfig holds classifier settings.
type ClassifierConfig struct {
	RulesPath string `yaml:"rules_path"`
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
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}

	return cfg, nil
}
