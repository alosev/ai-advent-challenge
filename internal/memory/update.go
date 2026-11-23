package memory

// WorkingSnapshot replaces the active task as a whole, so obsolete stages and
// constraints cannot leak into a new task.
type WorkingSnapshot struct {
	Task   string            `json:"task"`
	Plan   []string          `json:"plan"`
	Stage  string            `json:"current_stage"`
	Status string            `json:"status"`
	Facts  map[string]string `json:"facts"`
}

func (w WorkingSnapshot) State() State {
	return State{Task: w.Task, Plan: w.Plan, Stage: w.Stage, Status: w.Status, Facts: w.Facts}
}

// FactsPatch contains independent optional operations. The response decoder
// initializes absent operations as an empty map or slice before validation.
type FactsPatch struct {
	Upsert map[string]string `json:"upsert"`
	Delete []string          `json:"delete"`
}

// A nil layer means that the model chose not to change it.
type Update struct {
	Working  *WorkingSnapshot `json:"working_memory"`
	LongTerm *FactsPatch      `json:"long_term_memory"`
}

// SavedLayers records successful writes performed by the application, not
// claims made by the model. It is display metadata and never enters the prompt.
type SavedLayers struct {
	Working  bool `json:"working,omitempty"`
	LongTerm bool `json:"long_term,omitempty"`
}
