package llm

import "encoding/json"

// CompletionOptions contains optional controls supported by the OpenAI chat
// completions API.
type CompletionOptions struct {
	MaxTokens       int
	Temperature     *float64
	Thinking        string
	ReasoningEffort string
	StopSequences   []string
	ResponseSchema  *ResponseSchema
}

// ResponseSchema describes the desired JSON response.
type ResponseSchema struct {
	Schema json.RawMessage
}
