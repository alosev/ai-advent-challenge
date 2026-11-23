// Package memory stores task progress and durable facts separately.
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type State struct {
	Task   string            `json:"task,omitempty"`
	Plan   []string          `json:"plan,omitempty"`
	Stage  string            `json:"current_stage,omitempty"`
	Status string            `json:"status,omitempty"`
	Facts  map[string]string `json:"facts"`
}

func (s State) Clone() State {
	copy := State{Task: s.Task, Plan: append([]string(nil), s.Plan...), Stage: s.Stage, Status: s.Status, Facts: make(map[string]string, len(s.Facts))}
	for key, value := range s.Facts {
		copy.Facts[key] = value
	}
	return copy
}

type JSONStore struct{ path string }

func NewJSONStore(path string) *JSONStore { return &JSONStore{path: path} }

func (s *JSONStore) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return State{Facts: map[string]string{}}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("чтение памяти: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("повреждён JSON памяти: %w", err)
	}

	for key, value := range state.Facts {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return State{}, fmt.Errorf("память содержит пустой ключ или значение")
		}
	}
	return state, nil
}

func (s *JSONStore) Save(state State) error {
	state = state.Clone()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".memory-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.path)
}
