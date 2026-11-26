package app

import "llm-console/internal/tools"

const submitResponseName = "submit_response"

// SubmitResponseTool describes the terminal result. Its arguments are decoded
// by memoryClient, and ChatService applies memory updates after validation.
func SubmitResponseTool() *tools.Definition { return submitResponseTool(false) }

// BarbershopSubmitResponseTool requires the structured reply checked by invariants.
func BarbershopSubmitResponseTool() *tools.Definition { return submitResponseTool(true) }

func submitResponseTool(barbershop bool) *tools.Definition {
	object := func(properties map[string]any, required ...string) map[string]any {
		schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	text := map[string]any{"type": "string", "minLength": 1}
	stringsMap := map[string]any{"type": "object", "additionalProperties": text}
	nullable := func(value map[string]any) map[string]any {
		return map[string]any{"anyOf": []any{value, map[string]any{"type": "null"}}}
	}
	working := object(map[string]any{
		"task":          text,
		"plan":          map[string]any{"type": "array", "minItems": 1, "items": text},
		"current_stage": text,
		"status":        map[string]any{"type": "string", "enum": []string{"in_progress", "blocked", "completed"}},
		"facts":         stringsMap,
	}, "task", "plan", "current_stage", "status", "facts")
	factInput := object(map[string]any{
		"value":   text,
		"item_id": map[string]any{"type": "string", "minLength": 1, "description": "Устойчивый ID элемента; обязателен для decision.* и history.*, запрещён для одиночных фактов. Исправление использует прежний ID."},
	}, "value")
	longTerm := object(map[string]any{
		"upsert": map[string]any{"type": "object", "additionalProperties": factInput,
			"description": "Добавить или заменить факты. Если операция не нужна, поле можно пропустить.", "default": map[string]any{}},
		"delete": map[string]any{"type": "array", "uniqueItems": true, "items": text,
			"description": "Удалить перечисленные ключи. Если операция не нужна, поле можно пропустить.", "default": []string{}},
	})
	definition := &tools.Definition{Type: "function", Function: tools.FunctionDefinition{
		Name:        submitResponseName,
		Description: "Завершить текущий ход: передать ответ пользователю и обновления памяти. Вызывай отдельно, после получения всех нужных результатов других инструментов. Для неизменившихся слоёв памяти передай null.",
		Parameters: object(map[string]any{
			"answer":           text,
			"working_memory":   nullable(working),
			"long_term_memory": nullable(longTerm),
		}, "answer", "working_memory", "long_term_memory"),
	}}
	if barbershop {
		definition.Function.Parameters["properties"].(map[string]any)["barber_response"] = object(map[string]any{
			"kind":             map[string]any{"type": "string", "enum": []string{"booking", "rules", "clarify", "dialogue", "refuse"}},
			"start":            text,
			"duration_minutes": map[string]any{"type": "integer", "minimum": 1, "maximum": 1440, "description": "По умолчанию 40 минут: верхняя граница оценки 30–40 минут. У клиента не спрашивать."},
		}, "kind")
		definition.Function.Parameters["required"] = []string{"answer", "working_memory", "long_term_memory", "barber_response"}
		definition.Function.Description += " Обязательно передай barber_response; без него ответ будет заблокирован."
	}
	return definition

}
