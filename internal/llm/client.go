package llm

import "context"

// Client is the boundary between application logic and an LLM provider.
// A desktop UI can reuse the application service with any Client implementation.
type Client interface {
	Complete(ctx context.Context, messages []Message) (Result, error)
}
