// Package invariants stores and checks barbershop rules independently of chat memory.
package invariants

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Change struct {
	Timestamp string `json:"timestamp"`
	Key       string `json:"key"`
	OldValue  any    `json:"old_value"`
	NewValue  any    `json:"new_value"`
	Source    string `json:"source"`
}
type state struct {
	Rules   *BarbershopRules `json:"rules"`
	Changes []Change         `json:"changes"`
}
type Store struct {
	path  string
	state state
}

func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := strict(data, &s.state); err != nil {
		return nil, err
	}
	if s.state.Rules == nil {
		return nil, fmt.Errorf("отсутствуют rules")
	}
	if err := s.state.Rules.Validate(); err != nil {
		return nil, err
	}
	for _, change := range s.state.Changes {
		if _, err := time.Parse(time.RFC3339Nano, change.Timestamp); err != nil {
			return nil, fmt.Errorf("некорректное время изменения")
		}
		if change.Key == "" || change.Source != "user_command" {
			return nil, fmt.Errorf("некорректная запись изменения")
		}
	}
	return s, nil
}
func (s *Store) Rules() *BarbershopRules {
	if s.state.Rules == nil {
		return nil
	}
	copy := *s.state.Rules
	return &copy
}
func Minutes(value string) (int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil || t.Format("15:04") != value {
		return 0, fmt.Errorf("время должно иметь формат HH:MM (00:00–23:59)")
	}
	return t.Hour()*60 + t.Minute(), nil
}
func (r BarbershopRules) Validate() error {
	start, err := Minutes(r.WorkingHours.Start)
	if err != nil {
		return err
	}
	end, err := Minutes(r.WorkingHours.End)
	if err != nil {
		return err
	}
	if start >= end {
		return fmt.Errorf("начало рабочего дня должно быть раньше окончания")
	}
	return nil
}
func (r BarbershopRules) Description() string {
	return fmt.Sprintf("Обслуживание целиком в интервале %s–%s.", r.WorkingHours.Start, r.WorkingHours.End)
}
func (s *Store) Command(command string) (string, error) {
	fields := strings.Fields(command)
	help := "/invariant — правила; /invariant set <ключ> <значение> — изменить; /invariant history — журнал"
	if len(fields) == 1 && fields[0] == "/invariant" {
		if r := s.Rules(); r != nil {
			return r.Description() + "\n" + help, nil
		}
		return "Инварианты не включены.\n" + help, nil
	}
	if len(fields) == 2 && fields[1] == "history" {
		data, err := json.MarshalIndent(s.state.Changes, "", "  ")
		return string(data), err
	}
	var next BarbershopRules
	var old, value any
	var key string
	if len(fields) == 4 && fields[1] == "set" {
		if s.state.Rules == nil {
			return "", fmt.Errorf("задайте правила в invariants.json в директории агента")
		}
		next = *s.state.Rules
		key = fields[2]
		value = fields[3]
		switch key {
		case "working_hours.start":
			old = next.WorkingHours.Start
			next.WorkingHours.Start = fields[3]
		case "working_hours.end":
			old = next.WorkingHours.End
			next.WorkingHours.End = fields[3]
		default:
			return "", fmt.Errorf("неизвестный ключ; допустимы working_hours.start, working_hours.end")
		}
	} else {
		return "", fmt.Errorf("%s", help)
	}
	if err := next.Validate(); err != nil {
		return "", err
	}
	if old == value {
		return "Правило не изменилось.", nil
	}
	changes := append(append([]Change(nil), s.state.Changes...), Change{time.Now().Format(time.RFC3339Nano), key, old, value, "user_command"})
	candidate := state{&next, changes}
	if err := s.save(candidate); err != nil {
		return "", err
	}
	s.state = candidate
	before, _ := json.Marshal(old)
	after, _ := json.Marshal(value)
	return fmt.Sprintf("Изменено %s: %s → %s.\n%s", key, before, after, next.Description()), nil
}
func (s *Store) save(value state) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".invariants-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err = file.Write(append(data, '\n')); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.path)
}
func strict(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("лишние данные после JSON")
	}
	return nil
}

// Для услуг без известной длительности резервируем верхнюю границу оценки 30–40 минут.
const defaultServiceDurationMinutes = 40

func (r BarbershopRules) Render(data []byte, answer string) (string, error) {
	var response BarbershopResponse
	if err := strict(data, &response); err != nil {
		return "", err
	}
	if response.Kind != "booking" && (response.Start != "" || response.Duration != 0) {
		return "", fmt.Errorf("поля записи допустимы только для booking")
	}
	switch response.Kind {
	case "rules":
		return r.Description(), nil
	case "clarify", "dialogue":
		if strings.TrimSpace(answer) == "" {
			return "", fmt.Errorf("уточняющий вопрос не должен быть пустым")
		}
		return answer, nil
	case "refuse":
		return "Не могу предложить решение в обход действующих инвариантов. " + r.Description() + " Изменение возможно только командой /invariant set <ключ> <значение>.", nil
	case "booking":
		start, err := Minutes(response.Start)
		if err != nil {
			return "", err
		}
		if response.Duration < 0 || response.Duration > 1440 {
			return "", fmt.Errorf("длительность должна быть от 0 до 1440 минут; 0 означает оценку по умолчанию")
		}
		if response.Duration < defaultServiceDurationMinutes {
			response.Duration = defaultServiceDurationMinutes
		}
		opening, _ := Minutes(r.WorkingHours.Start)
		closing, _ := Minutes(r.WorkingHours.End)
		if start < opening || start+response.Duration > closing {
			return "", fmt.Errorf("обслуживание выходит за часы работы: %s", r.Description())
		}
		return answer, nil
	default:
		return "", fmt.Errorf("неизвестный вид ответа барбера")
	}
}
