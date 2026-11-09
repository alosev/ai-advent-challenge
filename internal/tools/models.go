package tools

// Definition describes a function in the OpenAI-compatible tools format.
type Definition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type calculateArguments struct {
	Operation string   `json:"operation"`
	A         *float64 `json:"a"`
	B         *float64 `json:"b"`
}

type textLengthArguments struct {
	Text *string `json:"text"`
}
