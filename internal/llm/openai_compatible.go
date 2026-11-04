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

// CompletionOptions contains optional controls supported by the OpenAI chat
// completions API.
type CompletionOptions struct {
	MaxTokens      int
	StopSequences  []string
	ResponseSchema *ResponseSchema
}

// ResponseSchema describes the desired JSON response.
type ResponseSchema struct {
	Schema json.RawMessage
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

func (c *OpenAICompatibleClient) Complete(ctx context.Context, messages []Message) (string, error) {
	requestMessages := messages
	if c.options.ResponseSchema != nil {
		requestMessages = append([]Message{jsonSchemaInstruction(c.options.ResponseSchema.Schema)}, messages...)
	}

	payload, err := json.Marshal(struct {
		Model          string          `json:"model"`
		Messages       []Message       `json:"messages"`
		MaxTokens      int             `json:"max_tokens,omitempty"`
		StopSequences  []string        `json:"stop,omitempty"`
		ResponseFormat *responseFormat `json:"response_format,omitempty"`
	}{
		Model:          c.model,
		Messages:       requestMessages,
		MaxTokens:      c.options.MaxTokens,
		StopSequences:  c.options.StopSequences,
		ResponseFormat: newResponseFormat(c.options.ResponseSchema),
	})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		log.Printf("LLM API returned an empty answer; response body: %s", string(body))
		return "", fmt.Errorf("API returned an empty answer")
	}
	return result.Choices[0].Message.Content, nil
}

type responseFormat struct {
	Type string `json:"type"`
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
