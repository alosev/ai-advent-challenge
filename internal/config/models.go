package config

import (
	"time"

	"llm-console/internal/profile"
)

// Config contains runtime settings. Keeping it independent from the UI makes it
// reusable by a CLI, desktop application, or HTTP server.
type Config struct {
	Profiles           []profile.Profile `json:"profiles"`
	ActiveProfile      string            `json:"active_profile"`
	APIResponseLogPath string            `json:"api_response_log_path"`
	WorkingMemoryPath  string            `json:"working_memory_path"`
	LongTermMemoryPath string            `json:"long_term_memory_path"`
	HistoryPath        string            `json:"history_path"`
	APIKey             string            `json:"api_key"`
	BaseURL            string            `json:"base_url"`
	Model              string            `json:"model"`
	TimeoutSeconds     int               `json:"timeout_seconds"`
	MaxTokens          int               `json:"max_tokens"`
	Temperature        *float64          `json:"temperature"`
	Thinking           string            `json:"thinking"`
	ReasoningEffort    string            `json:"reasoning_effort"`
	Timeout            time.Duration     `json:"-"`
}
