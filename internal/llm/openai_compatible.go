package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-console/internal/tools"
)

// OpenAICompatibleClient supports providers exposing POST /chat/completions.
type OpenAICompatibleClient struct {
	baseURL      string
	apiKey       string
	model        string
	options      CompletionOptions
	httpClient   *http.Client
	onToolCall   func(ToolCall)
	onToolResult func(ToolCall, string)
}

func NewOpenAICompatibleClient(baseURL, apiKey, model string, timeout time.Duration, options CompletionOptions) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		options:    options,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// SetToolCallObserver registers a callback invoked before a tool call is executed.
// Configure the observer before calling Complete.
func (c *OpenAICompatibleClient) SetToolCallObserver(observer func(ToolCall)) {
	c.onToolCall = observer
}

// SetToolResultObserver registers a callback invoked after each tool execution,
// including failures encoded as tool output. Configure before calling Complete.
func (c *OpenAICompatibleClient) SetToolResultObserver(observer func(ToolCall, string)) {
	c.onToolResult = observer
}

func (c *OpenAICompatibleClient) Complete(ctx context.Context, messages []Message) (Result, error) {
	requestMessages := messages
	if c.options.ResponseSchema != nil {
		requestMessages = append([]Message{jsonSchemaInstruction(c.options.ResponseSchema.Schema)}, messages...)
	}

	// Keep caller-owned messages intact while appending tool interactions.
	requestMessages = append([]Message(nil), requestMessages...)
	generatedStart := len(requestMessages)
	var total *Usage
	for round := 0; round < 10; round++ {
		response, err := c.completeOnce(ctx, requestMessages)
		if err != nil {
			return Result{Usage: total}, err
		}
		if response.Usage != nil {
			if total == nil {
				total = &Usage{}
			}
			total.PromptTokens += response.Usage.PromptTokens
			total.CompletionTokens += response.Usage.CompletionTokens
			total.TotalTokens += response.Usage.TotalTokens
		}
		if len(response.Choices) == 0 {
			return Result{Usage: total}, fmt.Errorf("API returned no choices")
		}
		message := response.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			if strings.TrimSpace(message.Content) == "" {
				return Result{Usage: total}, fmt.Errorf("API returned an empty answer")
			}
			requestMessages = append(requestMessages, message)
			return Result{Content: message.Content, Usage: total, Messages: requestMessages[generatedStart:]}, nil
		}
		requestMessages = append(requestMessages, message)
		for _, call := range message.ToolCalls {
			if c.onToolCall != nil {
				c.onToolCall(call)
			}
			if call.ID == "" {
				return Result{Usage: total}, fmt.Errorf("API returned a tool call without an ID")
			}
			var output string
			var callErr error
			if call.Type != "function" {
				callErr = fmt.Errorf("unsupported tool type: %s", call.Type)
			} else {
				output, callErr = tools.Execute(call.Function.Name, call.Function.Arguments)
			}
			if callErr != nil {
				encoded, _ := json.Marshal(map[string]string{"error": callErr.Error()})
				output = string(encoded)
			}
			if c.onToolResult != nil {
				c.onToolResult(call, output)
			}
			requestMessages = append(requestMessages, Message{Role: "tool", ToolCallID: call.ID, Content: output})
		}
	}
	return Result{Usage: total}, fmt.Errorf("tool call limit exceeded")
}

func (c *OpenAICompatibleClient) completeOnce(ctx context.Context, messages []Message) (completionResponse, error) {
	payload, err := json.Marshal(completionRequest{
		Tools:           tools.Definitions(),
		Model:           c.model,
		Messages:        messages,
		MaxTokens:       c.options.MaxTokens,
		Temperature:     c.options.Temperature,
		Thinking:        newThinkingOption(c.options.Thinking),
		ReasoningEffort: c.options.ReasoningEffort,
		StopSequences:   c.options.StopSequences,
		ResponseFormat:  newResponseFormat(c.options.ResponseSchema),
	})
	if err != nil {
		return completionResponse{}, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return completionResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return completionResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return completionResponse{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return completionResponse{}, fmt.Errorf("API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var result completionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return completionResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func newThinkingOption(value string) *thinkingOption {
	if value == "" {
		return nil
	}
	return &thinkingOption{Type: value}
}

func newResponseFormat(schema *ResponseSchema) *responseFormat {
	if schema == nil {
		return nil
	}
	return &responseFormat{Type: "json_object"}
}

func jsonSchemaInstruction(schema json.RawMessage) Message {
	return Message{
		Role:    "system",
		Content: "Return only a valid JSON object that conforms to this JSON Schema: " + string(schema),
	}
}
