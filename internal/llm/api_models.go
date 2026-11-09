package llm

import "llm-console/internal/tools"

// completionRequest and completionResponse describe the provider's chat API.
type completionRequest struct {
	Tools           []tools.Definition `json:"tools,omitempty"`
	Model           string             `json:"model"`
	Messages        []Message          `json:"messages"`
	MaxTokens       int                `json:"max_tokens,omitempty"`
	Temperature     *float64           `json:"temperature,omitempty"`
	Thinking        *thinkingOption    `json:"thinking,omitempty"`
	ReasoningEffort string             `json:"reasoning_effort,omitempty"`
	StopSequences   []string           `json:"stop,omitempty"`
	ResponseFormat  *responseFormat    `json:"response_format,omitempty"`
}

type completionResponse struct {
	Usage   *Usage             `json:"usage"`
	Choices []completionChoice `json:"choices"`
}

type completionChoice struct {
	Message Message `json:"message"`
}

type thinkingOption struct {
	Type string `json:"type"`
}

type responseFormat struct {
	Type string `json:"type"`
}
