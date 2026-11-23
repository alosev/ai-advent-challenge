package llm

import "llm-console/internal/tools"

// CompletionOptions contains optional controls supported by the OpenAI chat
// completions API.
type CompletionOptions struct {
	MaxTokens       int
	Temperature     *float64
	Thinking        string
	ReasoningEffort string
	// FinalTool ends the turn with its arguments, without executing it or
	// sending a tool result back to the provider.
	FinalTool *tools.Definition
}
