package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"llm-console/internal/invariants"
	"llm-console/internal/workflow"
)

// Enforce business rules where the task changes, not just in the final reply.
// Both assistant tools and user commands call the same guarded Machine.Apply.
func (s *ChatService) setWorkflowInvariantGuard() {
	if s.workflow == nil {
		return
	}
	s.workflow.SetTransitionGuard(func(next workflow.State, initial bool) error {
		if s.invariants == nil || next.Workflow != "booking" {
			return nil
		}
		rules := s.invariants.Rules()
		if rules == nil {
			return nil
		}
		slot := strings.TrimSpace(next.Data["slot"])
		if slot == "" {
			// Collection and revise may omit/clear the slot. Later steps require it.
			if initial {
				return nil
			}
			return fmt.Errorf("для перехода записи требуется время slot")
		}
		duration := 0 // The common booking validator reserves 40 minutes by default.
		if value, exists := next.Data["duration_minutes"]; exists {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("duration_minutes должно быть целым числом")
			}
			duration = parsed
		}
		data, err := json.Marshal(invariants.BarbershopResponse{Kind: "booking", Start: slot, Duration: duration})
		if err != nil {
			return err
		}
		if _, err = rules.Render(data, "Проверка параметров записи"); err != nil {
			return fmt.Errorf("инварианты барбершопа: %w; исправьте время через предусмотренный возврат /task action revise", err)
		}
		return nil
	})
}
