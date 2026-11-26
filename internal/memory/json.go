// Package memory stores conversation working context and durable facts separately.
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Task      string               `json:"task,omitempty"`
	Plan      []string             `json:"plan,omitempty"`
	Stage     string               `json:"current_stage,omitempty"`
	Status    string               `json:"status,omitempty"`
	Facts     map[string]Fact      `json:"facts"`
	Changes   []Change             `json:"changes,omitempty"`
	DeletedAt map[string]time.Time `json:"deleted_at,omitempty"`
}

func (s State) Clone() State {
	copy := State{Task: s.Task, Plan: append([]string(nil), s.Plan...), Stage: s.Stage, Status: s.Status, Facts: make(map[string]Fact, len(s.Facts)), Changes: append([]Change(nil), s.Changes...), DeletedAt: make(map[string]time.Time, len(s.DeletedAt))}
	for key, value := range s.Facts {
		copy.Facts[key] = cloneFact(value)
	}
	for key, at := range s.DeletedAt {
		copy.DeletedAt[key] = at
	}
	for i, change := range copy.Changes {
		if change.Before != nil {
			copy.Changes[i].Before = factPointer(*change.Before)
		}
		if change.After != nil {
			copy.Changes[i].After = factPointer(*change.After)
		}
	}
	return copy
}

type JSONStore struct{ path string }

func NewJSONStore(path string) *JSONStore { return &JSONStore{path: path} }

func (s *JSONStore) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return State{Facts: map[string]Fact{}}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("чтение памяти: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("повреждён JSON памяти: %w", err)
	}

	validate := ValidateFact
	if state.Task != "" {
		validate = ValidateWorkingFact
	}
	for key, value := range state.Facts {
		if err := validate(key, value); err != nil {
			return State{}, err
		}
	}
	for key, at := range state.DeletedAt {
		if key == "" || at.IsZero() {
			return State{}, fmt.Errorf("некорректная отметка удаления")
		}
	}
	for _, change := range state.Changes {
		if change.Key == "" || change.RecordedAt.IsZero() || change.Before == nil {
			return State{}, fmt.Errorf("некорректный журнал памяти")
		}
		if err := ValidateFact(change.Key, *change.Before); err != nil {
			return State{}, err
		}
		if change.After != nil {
			if err := ValidateFact(change.Key, *change.After); err != nil {
				return State{}, err
			}
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
