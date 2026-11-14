package app

import (
	"context"
	"fmt"
	"strings"

	"llm-console/internal/llm"
)

// ChatService contains application behavior and has no dependency on a UI.
// Conversation state lives in memory for the lifetime of the service.
type ChatService struct {
	client   llm.Client
	messages []llm.Message
}

func NewChatService(client llm.Client) *ChatService {
	return &ChatService{client: client}
}

func (s *ChatService) Ask(ctx context.Context, prompt string) (llm.Result, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return llm.Result{}, fmt.Errorf("prompt must not be empty")
	}
	messages := append(append([]llm.Message(nil), s.messages...), llm.Message{Role: "user", Content: prompt})
	result, err := s.client.Complete(ctx, messages)
	if err != nil {
		return result, err
	}
	s.messages = messages
	if len(result.Messages) > 0 {
		s.messages = append(s.messages, result.Messages...)
	} else {
		s.messages = append(s.messages, llm.Message{Role: "assistant", Content: result.Content})
	}
	return result, nil
}
