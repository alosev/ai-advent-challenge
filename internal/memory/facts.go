package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Entry IDs identify decisions/events so corrections replace an item instead of
// adding a contradictory neighbour. Singleton facts use an empty ID.
type Entry struct {
	ID         string    `json:"id,omitempty"`
	Value      string    `json:"value"`
	RecordedAt time.Time `json:"recorded_at"`
}

type Fact struct {
	Entries []Entry `json:"entries"`
}

type FactInput struct {
	Value  string `json:"value"`
	ItemID string `json:"item_id,omitempty"`
}

type Change struct {
	Key        string    `json:"key"`
	Before     *Fact     `json:"before"`
	After      *Fact     `json:"after"`
	RecordedAt time.Time `json:"recorded_at"`
	Reason     string    `json:"reason"`
}

func ListKey(key string) bool {
	return strings.HasPrefix(key, "decision.") || strings.HasPrefix(key, "history.")
}

func DurableKey(key string) bool {
	for _, prefix := range []string{"profile.", "decision.", "knowledge.", "history."} {
		if strings.HasPrefix(key, prefix) && len(key) > len(prefix) && key == strings.TrimSpace(key) {
			return true
		}
	}
	return false
}

func cloneFact(f Fact) Fact    { f.Entries = append([]Entry(nil), f.Entries...); return f }
func factPointer(f Fact) *Fact { copy := cloneFact(f); return &copy }

func ValidateFact(key string, f Fact) error {
	if !DurableKey(key) {
		return fmt.Errorf("некорректный ключ долговременной памяти %q", key)
	}
	return validateEntries(key, f, ListKey(key))
}

func ValidateWorkingFact(key string, f Fact) error {
	return validateEntries(key, f, false)
}

func validateEntries(key string, f Fact, list bool) error {
	if strings.TrimSpace(key) == "" || len(f.Entries) == 0 {
		return fmt.Errorf("факт %q не содержит значений", key)
	}
	if !list && (len(f.Entries) != 1 || f.Entries[0].ID != "") {
		return fmt.Errorf("факт %q должен иметь одно значение без id", key)
	}
	seen := map[string]bool{}
	for _, e := range f.Entries {
		if strings.TrimSpace(e.Value) == "" || e.RecordedAt.IsZero() || list && (strings.TrimSpace(e.ID) == "" || e.ID != strings.TrimSpace(e.ID) || seen[e.ID]) {
			return fmt.Errorf("факт %q: некорректные значение, id или время записи", key)
		}
		seen[e.ID] = true
	}
	return nil
}

// Put uses the observation time, supplied by the application when a request
// starts. A late response from an earlier request cannot overwrite newer data.
func (s *State) Put(key string, input FactInput, at time.Time) bool {
	if s.Facts == nil {
		s.Facts = map[string]Fact{}
	}
	if at.IsZero() || !at.After(s.DeletedAt[key]) {
		return false
	}
	old, exists := s.Facts[key]
	next := cloneFact(old)
	for i, e := range next.Entries {
		if e.ID == input.ItemID {
			if !at.After(e.RecordedAt) {
				return false
			}
			next.Entries[i] = Entry{ID: input.ItemID, Value: input.Value, RecordedAt: at}
			s.Facts[key] = next
			if e.Value != input.Value {
				s.Changes = append(s.Changes, Change{Key: key, Before: factPointer(old), After: factPointer(next), RecordedAt: at, Reason: "replace"})
			}
			return true
		}
	}
	next.Entries = append(next.Entries, Entry{ID: input.ItemID, Value: input.Value, RecordedAt: at})
	s.Facts[key] = next
	// Keep additions out of the audit: it records replacements and removals.
	if exists {
		s.Changes = append(s.Changes, Change{Key: key, Before: factPointer(old), After: factPointer(next), RecordedAt: at, Reason: "append"})
	}
	return true
}

func (s *State) DeleteFact(key string, at time.Time, reason string) bool {
	if !at.After(s.DeletedAt[key]) {
		return false
	}
	old, exists := s.Facts[key]
	for _, e := range old.Entries {
		if !at.After(e.RecordedAt) {
			return false
		}
	}
	if s.DeletedAt == nil {
		s.DeletedAt = map[string]time.Time{}
	}
	s.DeletedAt[key] = at
	delete(s.Facts, key)
	if exists {
		s.Changes = append(s.Changes, Change{Key: key, Before: factPointer(old), RecordedAt: at, Reason: reason})
	}
	return true
}

func (s State) Keys() []string {
	keys := make([]string, 0, len(s.Facts))
	for key := range s.Facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
