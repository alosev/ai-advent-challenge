package config

import (
	"encoding/json"
	"time"
)

// Config contains runtime settings. Keeping it independent from the UI makes it
// reusable by a CLI, desktop application, or HTTP server.
type Config struct {
	APIKey          string          `json:"api_key"`
	BaseURL         string          `json:"base_url"`
	Model           string          `json:"model"`
	TimeoutSeconds  int             `json:"timeout_seconds"`
	MaxTokens       int             `json:"max_tokens"`
	Temperature     *float64        `json:"temperature"`
	Thinking        string          `json:"thinking"`
	ReasoningEffort string          `json:"reasoning_effort"`
	StopSequences   []string        `json:"stop_sequences"`
	ResponseSchema  json.RawMessage `json:"response_schema"`
	Timeout         time.Duration   `json:"-"`
}
