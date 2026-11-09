package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatibleClient supports providers exposing POST /chat/completions.
type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	model      string
	options    CompletionOptions
	httpClient *http.Client
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

func (c *OpenAICompatibleClient) Complete(ctx context.Context, messages []Message) (Result, error) {
	requestMessages := messages
	if c.options.ResponseSchema != nil {
		requestMessages = append([]Message{jsonSchemaInstruction(c.options.ResponseSchema.Schema)}, messages...)
	}

	payload, err := json.Marshal(completionRequest{
		Model:           c.model,
		Messages:        requestMessages,
		MaxTokens:       c.options.MaxTokens,
		Temperature:     c.options.Temperature,
		Thinking:        newThinkingOption(c.options.Thinking),
		ReasoningEffort: c.options.ReasoningEffort,
		StopSequences:   c.options.StopSequences,
		ResponseFormat:  newResponseFormat(c.options.ResponseSchema),
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Result{}, fmt.Errorf("API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result completionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return Result{}, fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		log.Printf("LLM API returned an empty answer; response body: %s", string(body))
		return Result{}, fmt.Errorf("API returned an empty answer")
	}
	return Result{Content: result.Choices[0].Message.Content, Usage: result.Usage}, nil
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
