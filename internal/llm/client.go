package llm

import "context"

// Message is a provider-neutral chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Result struct {
	Content string
	Usage   *Usage
}

// Client is the boundary between application logic and an LLM provider.
// A desktop UI can reuse the application service with any Client implementation.
type Client interface {
	Complete(ctx context.Context, messages []Message) (Result, error)
}
