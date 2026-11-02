package llm

import "context"

// Message is a provider-neutral chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client is the boundary between application logic and an LLM provider.
// A desktop UI can reuse the application service with any Client implementation.
type Client interface {
	Complete(ctx context.Context, messages []Message) (string, error)
}
