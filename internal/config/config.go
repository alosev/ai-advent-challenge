package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"llm-console/internal/profile"
)

const (
	defaultConfigFile = "config.json"
	defaultBaseURL    = "https://api.deepseek.com"
	defaultModel      = "deepseek-flash"
	defaultTimeout    = 60 * time.Second
)

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
	cfg.Profile, err = profile.Normalize(cfg.Profile)
	if err != nil {
		return Config{}, err
	}
	paths := []*string{&cfg.HistoryPath, &cfg.WorkingMemoryPath, &cfg.LongTermMemoryPath, &cfg.APIResponseLogPath}
	defaults := []string{"history.json", "working-memory.json", "long-term-memory.json", "api-responses.jsonl"}
	configPath, err := filepath.Abs(path)
	if err != nil {
		return Config{}, fmt.Errorf("путь конфигурации: %w", err)
	}
	seen := map[string]bool{configPath: true}
	for i, target := range paths {
		*target = strings.TrimSpace(*target)
		if target == &cfg.APIResponseLogPath && *target == "-" {
			continue
		}
		if *target == "" {
			*target = defaults[i]
		}
		if !filepath.IsAbs(*target) {
			*target = filepath.Join(filepath.Dir(path), *target)
		}
		absolute, err := filepath.Abs(*target)
		if err != nil {
			return Config{}, fmt.Errorf("путь хранилища: %w", err)
		}
		if seen[absolute] {
			return Config{}, fmt.Errorf("конфигурация, история, рабочая память, долговременная память и лог API должны храниться в разных файлах")
		}
		relative, err := filepath.Rel(filepath.Dir(configPath), absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return Config{}, fmt.Errorf("файлы агента должны находиться в каталоге конфигурации")
		}
		*target = absolute
		seen[absolute] = true
	}
	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("задайте api_key в %s", path)
	}
	if cfg.MaxTokens < 0 {
		return Config{}, fmt.Errorf("max_tokens не может быть отрицательным")
	}
	if cfg.Temperature != nil && (*cfg.Temperature < 0 || *cfg.Temperature > 2) {
		return Config{}, fmt.Errorf("temperature должна быть в диапазоне от 0 до 2")
	}
	if cfg.Thinking != "" && cfg.Thinking != "enabled" && cfg.Thinking != "disabled" {
		return Config{}, fmt.Errorf("thinking должен быть enabled или disabled")
	}
	return cfg, nil
}

func readFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
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
	cfg.ReasoningEffort = strings.TrimSpace(cfg.ReasoningEffort)
	cfg.Thinking = strings.TrimSpace(cfg.Thinking)
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
