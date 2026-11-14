package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"llm-console/internal/history"
	"llm-console/internal/llm"
)

type HistoryStore interface {
	Load() (history.State, error)
	Save(history.State) error
	Clear() error
}

// ChatService owns conversation state independently of its user interface.
type ChatService struct {
	client   llm.Client
	messages []history.Message
	usage    *llm.Usage
	store    HistoryStore
}

func NewChatService(client llm.Client) *ChatService {
	return &ChatService{client: client}
}

func NewChatServiceWithHistory(client llm.Client, store HistoryStore) (*ChatService, error) {
	state, err := store.Load()
	if err != nil {
		return nil, err
	}
	return &ChatService{client: client, messages: state.Messages, usage: state.Usage, store: store}, nil
}

func (s *ChatService) TotalUsage() *llm.Usage {
	if s.usage == nil {
		return nil
	}
	usage := *s.usage
	return &usage
}

// History returns a snapshot so displaying history cannot mutate the context.
func (s *ChatService) History() history.State {
	messages := append([]history.Message(nil), s.messages...)
	for i := range messages {
		messages[i].ToolCalls = append([]llm.ToolCall(nil), messages[i].ToolCalls...)
	}
	for i := range messages {
		messages[i].Usage = copyUsage(messages[i].Usage)
		messages[i].TotalUsage = copyUsage(messages[i].TotalUsage)
	}
	return history.State{Version: 1, Messages: messages, Usage: s.TotalUsage()}
}

func copyUsage(usage *llm.Usage) *llm.Usage {
	if usage == nil {
		return nil
	}
	copy := *usage
	return &copy
}

func (s *ChatService) Clear() error {
	if s.store != nil {
		if err := s.store.Clear(); err != nil {
			return err
		}
	}
	s.messages = nil
	s.usage = nil
	return nil
}

func (s *ChatService) Ask(ctx context.Context, prompt string) (llm.Result, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return llm.Result{}, fmt.Errorf("prompt must not be empty")
	}
	// Extract only API fields; token statistics are local history metadata.
	messages := make([]llm.Message, 0, len(s.messages)+1)
	for _, message := range s.messages {
		messages = append(messages, message.Message)
	}
	messages = append(messages, llm.Message{Role: "user", Content: prompt})
	result, askErr := s.client.Complete(ctx, messages)
	if result.Usage != nil {
		if s.usage == nil {
			s.usage = &llm.Usage{}
		}
		s.usage.TotalTokens += result.Usage.TotalTokens
		s.usage.PromptTokens += result.Usage.PromptTokens
		s.usage.CompletionTokens += result.Usage.CompletionTokens
	}
	if askErr == nil {
		s.messages = append(s.messages, history.Message{Message: llm.Message{Role: "user", Content: prompt}})
		if len(result.Messages) > 0 {
			for _, message := range result.Messages {
				s.messages = append(s.messages, history.Message{Message: message})
			}
		} else {
			s.messages = append(s.messages, history.Message{Message: llm.Message{Role: "assistant", Content: result.Content}})
		}
		last := &s.messages[len(s.messages)-1]
		last.Usage = copyUsage(result.Usage)
		last.TotalUsage = s.TotalUsage()
	}
	// Persist known token usage even when a later API call in this turn failed.
	if s.store != nil && (askErr == nil || result.Usage != nil) {
		if err := s.store.Save(history.State{Messages: s.messages, Usage: s.usage}); err != nil {
			return result, errors.Join(askErr, fmt.Errorf("не удалось сохранить историю; текущий контекст остался в памяти: %w", err))
		}
	}
	return result, askErr
}
