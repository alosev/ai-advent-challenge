package memory

import "time"

// WorkingSnapshot replaces the current conversation context as a whole.
// Its goal and status are independent of any workflow instance.
type WorkingSnapshot struct {
	Task   string            `json:"task"`
	Plan   []string          `json:"plan"`
	Stage  string            `json:"current_stage"`
	Status string            `json:"status"`
	Facts  map[string]string `json:"facts"`
}

func (w WorkingSnapshot) State() State {
	var facts map[string]Fact
	if w.Facts != nil {
		facts = map[string]Fact{}
	}
	for key, value := range w.Facts {
		facts[key] = Fact{Entries: []Entry{{Value: value}}}
	}
	return State{Task: w.Task, Plan: w.Plan, Stage: w.Stage, Status: w.Status, Facts: facts}
}

// FactsPatch contains independent optional operations. The response decoder
// initializes absent operations as an empty map or slice before validation.
type FactsPatch struct {
	Upsert map[string]FactInput `json:"upsert"`
	Delete []string             `json:"delete"`
}

// A nil layer means that the model chose not to change it.
type Update struct {
	RecordedAt time.Time        `json:"-"`
	Working    *WorkingSnapshot `json:"working_memory"`
	LongTerm   *FactsPatch      `json:"long_term_memory"`
}

// SavedLayers records successful writes performed by the application, not
// claims made by the model. It is display metadata and never enters the prompt.
type SavedLayers struct {
	Working  bool `json:"working,omitempty"`
	LongTerm bool `json:"long_term,omitempty"`
}
