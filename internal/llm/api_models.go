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
	ToolChoice      string             `json:"tool_choice,omitempty"`
}

type completionResponse struct {
	diagnostic ResponseRecord
	Usage      *Usage             `json:"usage"`
	Choices    []completionChoice `json:"choices"`
}

type completionChoice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type thinkingOption struct {
	Type string `json:"type"`
}
