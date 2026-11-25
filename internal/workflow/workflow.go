// Package workflow implements configured task state machines independently of LLM text.
package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Definition struct {
	Description string          `json:"description"`
	Activation  string          `json:"activation"` // startup or on_request
	Trigger     string          `json:"trigger"`
	Task        string          `json:"task,omitempty"` // initial goal for startup
	Initial     string          `json:"initial"`
	Steps       map[string]Step `json:"steps"`
}

type Step struct {
	Stage          string                `json:"stage"`
	ExpectedAction string                `json:"expected_action"`
	Terminal       bool                  `json:"terminal,omitempty"`
	Tools          []string              `json:"tools,omitempty"`
	Actions        map[string]Transition `json:"actions,omitempty"`
}

type Transition struct {
	Actor    string   `json:"actor"` // user or assistant; never supplied by the model
	To       string   `json:"to"`
	Fields   []string `json:"fields,omitempty"` // keys this action may write
	Required []string `json:"required,omitempty"`
	Clear    []string `json:"clear,omitempty"`
}

type State struct {
	Workflow string            `json:"workflow"`
	Task     string            `json:"task"`
	Step     string            `json:"step"`
	Data     map[string]string `json:"data"`
}

func (s *State) clone() *State {
	if s == nil {
		return nil
	}
	copy := *s
	copy.Data = make(map[string]string, len(s.Data))
	for k, v := range s.Data {
		copy.Data[k] = v
	}
	return &copy
}

func validText(s string) bool { return strings.TrimSpace(s) != "" && s == strings.TrimSpace(s) }

// Validate rejects broken graphs, including nonterminal steps with no path to an end.
func Validate(definitions map[string]Definition) error {
	startup := ""
	for name, d := range definitions {
		if !validText(name) || !validText(d.Description) || d.Initial == "" || len(d.Steps) == 0 {
			return fmt.Errorf("процесс %q: обязательны название, description, initial и steps", name)
		}
		if d.Activation != "startup" && d.Activation != "on_request" || !validText(d.Trigger) {
			return fmt.Errorf("процесс %q: требуются activation startup или on_request и непустой trigger", name)
		}
		if d.Activation == "startup" {
			if startup != "" || !validText(d.Task) {
				return fmt.Errorf("допустим один startup-процесс с непустой task")
			}
			startup = name
		}
		if start, ok := d.Steps[d.Initial]; !ok || start.Terminal {
			return fmt.Errorf("процесс %q: initial должен указывать на незавершённый шаг", name)
		}
		knownFields := map[string]bool{}
		for _, step := range d.Steps {
			for _, transition := range step.Actions {
				for _, field := range transition.Fields {
					knownFields[field] = true
				}
			}
		}
		for id, step := range d.Steps {
			if !validText(id) || !validText(step.Stage) || !validText(step.ExpectedAction) {
				return fmt.Errorf("процесс %q, шаг %q: обязательны stage и expected_action", name, id)
			}
			if step.Terminal && (len(step.Actions) > 0 || len(step.Tools) > 0) || !step.Terminal && len(step.Actions) == 0 {
				return fmt.Errorf("процесс %q, шаг %q: конечный шаг не имеет действий и инструментов, остальные должны иметь действия", name, id)
			}
			for _, tool := range step.Tools {
				if !validText(tool) || tool == "task_action" || tool == "task_start" || tool == "submit_response" {
					return fmt.Errorf("процесс %q: некорректный обычный инструмент %q", name, tool)
				}
			}
			for action, transition := range step.Actions {
				if !validText(action) || transition.Actor != "user" && transition.Actor != "assistant" {
					return fmt.Errorf("процесс %q: действие %q требует actor user или assistant", name, action)
				}
				if _, ok := d.Steps[transition.To]; !ok {
					return fmt.Errorf("процесс %q: действие %q ссылается на неизвестный шаг %q", name, action, transition.To)
				}
				for _, keys := range [][]string{transition.Fields, transition.Required, transition.Clear} {
					seen := map[string]bool{}
					for _, key := range keys {
						if !validText(key) || seen[key] {
							return fmt.Errorf("процесс %q: пустой или повторяющийся ключ в действии %q", name, action)
						}
						seen[key] = true
					}
				}
				for _, key := range append(append([]string(nil), transition.Required...), transition.Clear...) {
					if !knownFields[key] {
						return fmt.Errorf("процесс %q: поле %q в действии %q не определено ни одним fields", name, key, action)
					}
				}
			}
		}
		// Reverse reachability proves a possible end exists, without requiring a linear graph.
		canFinish := map[string]bool{}
		for id, step := range d.Steps {
			canFinish[id] = step.Terminal
		}
		for changed := true; changed; {
			changed = false
			for id, step := range d.Steps {
				for _, t := range step.Actions {
					if !canFinish[id] && canFinish[t.To] {
						canFinish[id], changed = true, true
					}
				}
			}
		}
		reachable := map[string]bool{}
		var visit func(string)
		visit = func(id string) {
			if reachable[id] {
				return
			}
			reachable[id] = true
			for _, t := range d.Steps[id].Actions {
				visit(t.To)
			}
		}
		visit(d.Initial)
		for id := range d.Steps {
			if !canFinish[id] || !reachable[id] {
				return fmt.Errorf("процесс %q: шаг %q недостижим или не ведёт к завершению", name, id)
			}
		}
	}
	return nil
}

type Store interface {
	Load() (*State, error)
	Save(*State) error
}

type Machine struct {
	definitions map[string]Definition
	state       *State
	store       Store
}

func New(definitions map[string]Definition, store Store) (*Machine, error) {
	// Own the configuration so callers cannot change the graph after validation.
	data, err := json.Marshal(definitions)
	if err != nil {
		return nil, err
	}
	var owned map[string]Definition
	if err := json.Unmarshal(data, &owned); err != nil {
		return nil, err
	}
	if err := Validate(owned); err != nil {
		return nil, err
	}
	m := &Machine{definitions: owned, store: store}
	if store != nil {
		m.state, err = store.Load()
		if err != nil {
			return nil, err
		}
	}
	if m.state != nil {
		d, ok := owned[m.state.Workflow]
		if !ok {
			return nil, fmt.Errorf("процесс сохранённой задачи %q отсутствует в конфигурации", m.state.Workflow)
		}
		if _, ok := d.Steps[m.state.Step]; !ok || !validText(m.state.Task) || m.state.Data == nil {
			return nil, fmt.Errorf("некорректный шаг, название или данные сохранённой задачи")
		}
		for k, v := range m.state.Data {
			if !validText(k) || strings.TrimSpace(v) == "" {
				return nil, fmt.Errorf("пустые данные сохранённой задачи")
			}
			known := false
			for _, step := range d.Steps {
				for _, t := range step.Actions {
					if contains(t.Fields, k) {
						known = true
					}
				}
			}
			if !known {
				return nil, fmt.Errorf("неизвестное поле сохранённой задачи %q", k)
			}
		}
		m.state = m.state.clone()
	}
	// Restore an existing task as is; only an empty task slot activates startup.
	if m.state == nil {
		for name, definition := range owned {
			if definition.Activation == "startup" {
				if err := m.Start(name, definition.Task); err != nil {
					return nil, err
				}
			}
		}
	}
	return m, nil
}

func (m *Machine) Active() bool {
	return m.state != nil && !m.definitions[m.state.Workflow].Steps[m.state.Step].Terminal
}

func (m *Machine) Start(name, task string) error {
	if m.Active() {
		return fmt.Errorf("сначала завершите или отмените текущую задачу")
	}
	d, ok := m.definitions[name]
	if !ok {
		return fmt.Errorf("неизвестный процесс %q", name)
	}
	task = strings.TrimSpace(task)
	if task == "" {
		return fmt.Errorf("укажите название задачи")
	}
	return m.commit(&State{Workflow: name, Task: task, Step: d.Initial, Data: map[string]string{}})
}

func (m *Machine) Cancel() error { return m.commit(nil) }

func (m *Machine) commit(next *State) error {
	if m.store != nil {
		if err := m.store.Save(next.clone()); err != nil {
			return fmt.Errorf("задача не изменена: %w", err)
		}
	}
	m.state = next.clone()
	return nil
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// Apply validates the actor, edge and data before persisting any change.
func (m *Machine) Apply(actor, action string, fields map[string]string) error {
	if !m.Active() {
		return fmt.Errorf("активной задачи нет")
	}
	step := m.definitions[m.state.Workflow].Steps[m.state.Step]
	t, ok := step.Actions[action]
	if !ok || t.Actor != actor {
		return fmt.Errorf("действие %q недоступно для %s на шаге %q", action, actor, m.state.Step)
	}
	next := m.state.clone()
	for _, k := range t.Clear {
		delete(next.Data, k)
	}
	for k, v := range fields {
		if !contains(t.Fields, k) || strings.TrimSpace(v) == "" {
			return fmt.Errorf("действие %q не разрешает поле %q или его значение пусто", action, k)
		}
		next.Data[k] = v
	}
	for _, key := range t.Required {
		if strings.TrimSpace(next.Data[key]) == "" {
			return fmt.Errorf("для действия %q требуется поле %q", action, key)
		}
	}
	next.Step = t.To
	return m.commit(next)
}

// CheckTool is called immediately before execution, including for hidden/forged calls.
func (m *Machine) CheckTool(name string) error {
	if !m.Active() {
		return nil
	}
	step := m.definitions[m.state.Workflow].Steps[m.state.Step]
	if !contains(step.Tools, name) {
		return fmt.Errorf("инструмент %q запрещён на шаге %q", name, m.state.Step)
	}
	return nil
}

func (m *Machine) View() string {
	var task any
	if m.state != nil {
		task = struct {
			*State
			Stage          string                `json:"stage"`
			ExpectedAction string                `json:"expected_action"`
			Terminal       bool                  `json:"terminal"`
			Actions        map[string]Transition `json:"actions"`
			Tools          []string              `json:"tools"`
		}{m.state.clone(), m.definitions[m.state.Workflow].Steps[m.state.Step].Stage,
			m.definitions[m.state.Workflow].Steps[m.state.Step].ExpectedAction,
			m.definitions[m.state.Workflow].Steps[m.state.Step].Terminal,
			m.definitions[m.state.Workflow].Steps[m.state.Step].Actions,
			m.definitions[m.state.Workflow].Steps[m.state.Step].Tools}
	}
	data, _ := json.MarshalIndent(task, "", "  ")
	return string(data)
}

func (m *Machine) List() string {
	names := make([]string, 0, len(m.definitions))
	for name := range m.definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	var lines []string
	for _, name := range names {
		d := m.definitions[name]
		lines = append(lines, name+" ["+d.Activation+"] — "+d.Description+"; запуск по запросу: "+d.Trigger)
	}
	if len(lines) == 0 {
		return "Процессы не заданы: агент работает как обычный чат."
	}
	return strings.Join(lines, "\n")
}

// Catalog gives the model activation rules without allowing it to edit the graph.
func (m *Machine) Catalog() string {
	type entry struct {
		Description string `json:"description"`
		Activation  string `json:"activation"`
		Trigger     string `json:"trigger"`
	}
	catalog := map[string]entry{}
	for name, d := range m.definitions {
		catalog[name] = entry{d.Description, d.Activation, d.Trigger}
	}
	data, _ := json.Marshal(catalog)
	return string(data)
}
