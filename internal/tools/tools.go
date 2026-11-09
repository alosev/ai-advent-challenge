package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"
	"unicode/utf8"
)

// Definitions returns all available tools and their JSON Schema parameters.
func Definitions() []Definition {
	object := func(properties map[string]any, required ...string) map[string]any {
		if required == nil {
			required = []string{}
		}
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	return []Definition{
		{Type: "function", Function: FunctionDefinition{Name: "current_time", Description: "Get the current date and time in UTC.", Parameters: object(map[string]any{})}},
		{Type: "function", Function: FunctionDefinition{Name: "calculate", Description: "Perform arithmetic on two numbers.", Parameters: object(map[string]any{
			"operation": map[string]any{"type": "string", "enum": []string{"add", "subtract", "multiply", "divide"}},
			"a":         map[string]any{"type": "number"}, "b": map[string]any{"type": "number"},
		}, "operation", "a", "b")}},
		{Type: "function", Function: FunctionDefinition{Name: "text_length", Description: "Count Unicode characters in text.", Parameters: object(map[string]any{"text": map[string]any{"type": "string"}}, "text")}},
	}
}

// Execute validates arguments and returns a JSON result for the model.
func Execute(name, arguments string) (string, error) {
	var result any
	switch name {
	case "current_time":
		var args struct{}
		if err := decode(arguments, &args); err != nil {
			return "", err
		}
		result = map[string]any{"utc": time.Now().UTC().Format(time.RFC3339)}
	case "calculate":
		var args calculateArguments
		if err := decode(arguments, &args); err != nil {
			return "", err
		}
		if args.A == nil || args.B == nil {
			return "", fmt.Errorf("a and b are required numbers")
		}
		var value float64
		switch args.Operation {
		case "add":
			value = *args.A + *args.B
		case "subtract":
			value = *args.A - *args.B
		case "multiply":
			value = *args.A * *args.B
		case "divide":
			if *args.B == 0 {
				return "", fmt.Errorf("division by zero")
			}
			value = *args.A / *args.B
		default:
			return "", fmt.Errorf("invalid operation")
		}
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return "", fmt.Errorf("result is not finite")
		}
		result = map[string]any{"result": value}
	case "text_length":
		var args textLengthArguments
		if err := decode(arguments, &args); err != nil {
			return "", err
		}
		if args.Text == nil {
			return "", fmt.Errorf("text is required")
		}
		result = map[string]any{"length": utf8.RuneCountInString(*args.Text)}
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	data, err := json.Marshal(result)
	return string(data), err
}

func decode(input string, target any) error {
	if !json.Valid([]byte(input)) || len(bytes.TrimSpace([]byte(input))) == 0 || bytes.TrimSpace([]byte(input))[0] != '{' {
		return fmt.Errorf("arguments must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("invalid trailing arguments")
	}
	return nil
}
