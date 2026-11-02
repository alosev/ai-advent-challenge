package app

import (
	"context"
	"fmt"
	"strings"

	"llm-console/internal/llm"
)

// ChatService contains application behavior and has no dependency on a UI.
// Add a History repository here later without changing CLI or desktop code.
type ChatService struct {
	client llm.Client
}

func NewChatService(client llm.Client) *ChatService {
	return &ChatService{client: client}
}

func (s *ChatService) Ask(ctx context.Context, prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt must not be empty")
	}
	return s.client.Complete(ctx, []llm.Message{{Role: "user", Content: prompt}})
}
