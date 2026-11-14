// Package history stores a conversation independently of its user interface.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"llm-console/internal/llm"
)

// Message adds history-only statistics to a provider-neutral message.
type Message struct {
	llm.Message
	Usage      *llm.Usage `json:"usage,omitempty"`
	TotalUsage *llm.Usage `json:"total_usage,omitempty"`
}

type State struct {
	Version  int        `json:"version"`
	Messages []Message  `json:"messages"`
	Usage    *llm.Usage `json:"usage,omitempty"`
}

type JSONStore struct {
	path string
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

func (s *JSONStore) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return State{Version: 1}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("чтение истории: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("повреждён JSON истории: %w", err)
	}
	if state.Version != 1 {
		return State{}, fmt.Errorf("неподдерживаемая версия истории: %d", state.Version)
	}
	for _, message := range state.Messages {
		switch message.Role {
		case "user", "assistant", "tool":
		default:
			return State{}, fmt.Errorf("неизвестная роль в истории: %q", message.Role)
		}
	}
	if state.Usage != nil && (state.Usage.TotalTokens < 0 || state.Usage.PromptTokens < 0 || state.Usage.CompletionTokens < 0) {
		return State{}, fmt.Errorf("отрицательный расход токенов в истории")
	}
	for _, message := range state.Messages {
		if message.Usage != nil || message.TotalUsage != nil {
			if message.Role != "assistant" || len(message.ToolCalls) != 0 {
				return State{}, fmt.Errorf("статистика привязана не к итоговому ответу")
			}
		}
		for _, usage := range []*llm.Usage{message.Usage, message.TotalUsage} {
			if usage != nil && (usage.TotalTokens < 0 || usage.PromptTokens < 0 || usage.CompletionTokens < 0) {
				return State{}, fmt.Errorf("отрицательный расход токенов ответа в истории")
			}
		}
	}
	return state, nil
}

func (s *JSONStore) Save(state State) error {
	state.Version = 1
	if state.Messages == nil {
		state.Messages = []Message{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование истории: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("создание каталога истории: %w", err)
	}
	file, err := os.CreateTemp(dir, ".history-*.tmp")
	if err != nil {
		return fmt.Errorf("создание временного файла истории: %w", err)
	}
	defer os.Remove(file.Name())
	// Always close the temporary file, including after a failed write.
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("запись истории: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("синхронизация истории: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("закрытие файла истории: %w", err)
	}
	if err := os.Rename(file.Name(), s.path); err != nil {
		return fmt.Errorf("замена файла истории: %w", err)
	}
	return nil
}

func (s *JSONStore) Clear() error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("удаление истории: %w", err)
	}
	return nil
}
