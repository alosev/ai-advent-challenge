package invariants

type BarbershopRules struct {
	WorkingHours struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"working_hours"`
}

// BarbershopResponse contains a structured reply for the barbershop scenario.
// Booking parameters are checked locally; conversational wording comes from the model.
type BarbershopResponse struct {
	Kind     string `json:"kind"`
	Start    string `json:"start,omitempty"`
	Duration int    `json:"duration_minutes,omitempty"`
}
