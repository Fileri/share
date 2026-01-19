package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the server configuration
type Config struct {
	Domain     string        `yaml:"domain"`
	BaseURL    string        `yaml:"base_url"`
	ListenAddr string        `yaml:"listen_addr"`
	Storage    StorageConfig `yaml:"storage"`
	Limits     LimitsConfig  `yaml:"limits"`
	Auth       AuthConfig    `yaml:"auth"`
}

// StorageConfig holds S3-compatible storage configuration
type StorageConfig struct {
	Type            string `yaml:"type"` // "s3" or "filesystem"
	Endpoint        string `yaml:"endpoint"`
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	// For filesystem storage
	Path string `yaml:"path"`
}

// LimitsConfig holds rate limiting and size limits
type LimitsConfig struct {
	MaxFileSize  string `yaml:"max_file_size"`  // e.g., "100MB", "1GB", "0" for unlimited
	RateLimit    string `yaml:"rate_limit"`     // e.g., "10/minute", "0" for unlimited
	StorageQuota string `yaml:"storage_quota"`  // per-user quota
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Tokens []string `yaml:"tokens"` // valid API tokens
}

// Load reads configuration from file
func Load() (*Config, error) {
	configPath := os.Getenv("SHARE_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Return default config if no file exists
		return defaultConfig(), nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults
	cfg.applyDefaults()

	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Domain:     "localhost",
		BaseURL:    "http://localhost:8080",
		ListenAddr: ":8080",
		Storage: StorageConfig{
			Type: "filesystem",
			Path: "./data",
		},
		Limits: LimitsConfig{
			MaxFileSize:  "0",
			RateLimit:    "0",
			StorageQuota: "0",
		},
	}
}

func (c *Config) applyDefaults() {
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	if c.Storage.Type == "" {
		c.Storage.Type = "filesystem"
	}
	if c.Storage.Type == "filesystem" && c.Storage.Path == "" {
		c.Storage.Path = "./data"
	}
	if c.Limits.MaxFileSize == "" {
		c.Limits.MaxFileSize = "0"
	}
}
