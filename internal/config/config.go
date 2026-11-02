package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultConfigFile = "config.json"
	defaultBaseURL    = "https://api.deepseek.com"
	defaultModel      = "deepseek-flash"
	defaultTimeout    = 60 * time.Second
)

// Config contains runtime settings. Keeping it independent from the UI makes it
// reusable by a CLI, desktop application, or HTTP server.
type Config struct {
	APIKey         string        `json:"api_key"`
	BaseURL        string        `json:"base_url"`
	Model          string        `json:"model"`
	TimeoutSeconds int           `json:"timeout_seconds"`
	Timeout        time.Duration `json:"-"`
}

// Load reads application settings from a JSON file only.
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultConfigFile
	}
	cfg, err := readFile(path)
	if err != nil {
		return Config{}, err
	}
	normalize(&cfg)
	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("задайте api_key в %s", path)
	}
	return cfg, nil
}

func readFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("прочитать конфигурацию %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("разобрать конфигурацию %s: %w", path, err)
	}
	return cfg, nil
}

func normalize(cfg *Config) {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.Timeout = defaultTimeout
	} else {
		cfg.Timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
}
