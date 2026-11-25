package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-console/internal/tools"
)

// OpenAICompatibleClient supports providers exposing POST /chat/completions.
type OpenAICompatibleClient struct {
	disableTools    bool
	baseURL         string
	apiKey          string
	model           string
	options         CompletionOptions
	httpClient      *http.Client
	onToolCall      func(ToolCall)
	onToolResult    func(ToolCall, string)
	onResponse      func(ResponseRecord)
	additionalTools []tools.Definition
	executeTool     func(context.Context, string, string) (string, error)
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

// SummaryClient shares only connection settings and the model. Generation
// options from the main chat are not used for summarization.
func (c *OpenAICompatibleClient) SummaryClient() Client {
	return &OpenAICompatibleClient{
		baseURL: c.baseURL, apiKey: c.apiKey, model: c.model,
		httpClient: c.httpClient, disableTools: true,
		onResponse: c.onResponse,
	}
}

// SetResponseObserver observes problematic HTTP responses. Configure it before
// creating the summary client. Request settings are never included.
func (c *OpenAICompatibleClient) SetResponseObserver(observer func(ResponseRecord)) {
	c.onResponse = observer
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

// SetToolExecutor installs application tools and a mandatory execution gate.
// The callback also handles ordinary tools; it is not copied to SummaryClient.
func (c *OpenAICompatibleClient) SetToolExecutor(additional []tools.Definition, execute func(context.Context, string, string) (string, error)) {
	c.additionalTools = append([]tools.Definition(nil), additional...)
	c.executeTool = execute
}

func (c *OpenAICompatibleClient) Complete(ctx context.Context, messages []Message) (Result, error) {
	requestMessages := append([]Message(nil), messages...)
	generatedStart := len(requestMessages)
	var total *Usage
	var lastResponse completionResponse
	for round := 0; round < 10; round++ {
		response, err := c.completeOnce(ctx, requestMessages)
		if err != nil {
			return Result{Usage: total}, err
		}
		lastResponse = response
		if response.Usage != nil {
			if total == nil {
				total = &Usage{}
			}
			total.PromptTokens += response.Usage.PromptTokens
			total.CompletionTokens += response.Usage.CompletionTokens
			total.TotalTokens += response.Usage.TotalTokens
		}
		if len(response.Choices) == 0 {
			return c.protocolError(response, total, "no_choices", fmt.Errorf("API returned no choices"))
		}
		choice := response.Choices[0]
		message := choice.Message
		if choice.FinishReason == "length" {
			return c.protocolError(response, total, "truncated_answer", fmt.Errorf("API обрезал ответ (finish_reason=length, max_tokens=%d); увеличьте max_tokens или задайте 0", c.options.MaxTokens))
		}
		switch choice.FinishReason {
		case "content_filter", "insufficient_system_resource", "aborted":
			return c.protocolError(response, total, "incomplete_response", fmt.Errorf("API прервал ответ: finish_reason=%s", choice.FinishReason))
		}
		if c.disableTools && len(message.ToolCalls) > 0 {
			return c.protocolError(response, total, "unexpected_tool_calls", fmt.Errorf("API returned tool calls during summarization"))
		}
		if final := c.options.FinalTool; final != nil {
			for _, call := range message.ToolCalls {
				if call.Function.Name != final.Function.Name {
					continue
				}
				// Reject a mixed batch before executing any ordinary tools. The
				// final answer must use results already available to the model.
				if len(message.ToolCalls) != 1 || call.Type != "function" || call.ID == "" {
					return c.protocolError(response, total, "invalid_final_tool", fmt.Errorf("%s должен быть единственным вызовом в ответе, с типом function и непустым ID", final.Function.Name))
				}
				// An HTTP response can be complete while its function arguments
				// are truncated, even with finish_reason=tool_calls.
				var arguments json.RawMessage
				decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
				if err := decoder.Decode(&arguments); err != nil {
					if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
						return c.protocolError(response, total, "incomplete_final_arguments", fmt.Errorf("API вернул незавершённый JSON аргументов %s (finish_reason=%s); ответ не выдан, память не изменена; повторный запрос не выполнялся", final.Function.Name, choice.FinishReason))
					}
					return c.protocolError(response, total, "invalid_final_arguments", fmt.Errorf("API вернул некорректный JSON аргументов %s: %w; память не изменена", final.Function.Name, err))
				}
				if err := decoder.Decode(new(any)); err != io.EOF {
					return c.protocolError(response, total, "invalid_final_arguments", fmt.Errorf("API вернул лишние данные после JSON аргументов %s; память не изменена", final.Function.Name))
				}
				// Consume the terminal call locally. No tool result or additional
				// request is needed, and no unmatched call enters chat history.
				terminal := Message{Role: "assistant", Content: call.Function.Arguments, ReasoningContent: message.ReasoningContent}
				generated := append(requestMessages[generatedStart:], terminal)
				return c.finalResult(response, total, terminal.Content, generated), nil
			}
			if len(message.ToolCalls) == 0 {
				return c.protocolError(response, total, "missing_final_tool", fmt.Errorf("API не вызвал обязательный инструмент %s; повторный запрос не выполнялся", final.Function.Name))
			}
		}
		if len(message.ToolCalls) == 0 {
			if strings.TrimSpace(message.Content) == "" {
				return c.protocolError(response, total, "empty_answer", c.emptyAnswerError(choice))
			}
			generated := append(requestMessages[generatedStart:], message)
			return c.finalResult(response, total, message.Content, generated), nil
		}
		seen := make(map[string]bool, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			if call.ID == "" || seen[call.ID] || call.Type != "function" {
				return c.protocolError(response, total, "invalid_tool_call", fmt.Errorf("API вернул некорректный вызов инструмента: нужен уникальный непустой ID и тип function"))
			}
			seen[call.ID] = true
		}
		requestMessages = append(requestMessages, message)
		for _, call := range message.ToolCalls {
			if c.onToolCall != nil {
				c.onToolCall(call)
			}
			var output string
			var callErr error
			if c.executeTool != nil {
				output, callErr = c.executeTool(ctx, call.Function.Name, call.Function.Arguments)
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
	return c.protocolError(lastResponse, total, "tool_call_limit", fmt.Errorf("tool call limit exceeded"))
}

func (c *OpenAICompatibleClient) protocolError(response completionResponse, usage *Usage, category string, err error) (Result, error) {
	if c.onResponse != nil {
		record := response.diagnostic
		record.Error = category
		c.onResponse(record)
	}
	return Result{Usage: usage}, err
}

func (c *OpenAICompatibleClient) finalResult(response completionResponse, usage *Usage, content string, generated []Message) Result {
	result := Result{Content: content, Usage: usage, Messages: generated}
	if c.onResponse != nil {
		record := response.diagnostic
		result.LogInvalidResponse = func() {
			record.Error = "invalid_final_arguments"
			c.onResponse(record)
		}
	}
	return result
}

func (c *OpenAICompatibleClient) completeOnce(ctx context.Context, messages []Message) (result completionResponse, callErr error) {
	record := ResponseRecord{Time: time.Now().UTC()}
	defer func() {
		if c.onResponse == nil || record.HTTPStatus == 0 {
			return
		}
		if callErr != nil && record.Error == "" {
			record.Error = "invalid_response"
		}
		if record.Error != "" {
			c.onResponse(record)
		}
	}()
	definitions := tools.Definitions()
	definitions = append(definitions, c.additionalTools...)
	if c.disableTools {
		definitions = nil
	}
	toolChoice := ""
	thinking, reasoningEffort := c.options.Thinking, c.options.ReasoningEffort
	if c.disableTools {
		// Summaries never inherit the main chat's reasoning configuration.
		thinking, reasoningEffort = "disabled", ""
	}
	if c.options.FinalTool != nil && !c.disableTools {
		definitions = append(definitions, *c.options.FinalTool)
		toolChoice = "required"
		// Required tool choice is incompatible with thinking mode.
		thinking, reasoningEffort = "disabled", ""
	}
	payload, err := json.Marshal(completionRequest{
		Tools:           definitions,
		Model:           c.model,
		Messages:        messages,
		MaxTokens:       c.options.MaxTokens,
		Temperature:     c.options.Temperature,
		Thinking:        newThinkingOption(thinking),
		ReasoningEffort: reasoningEffort,
		ToolChoice:      toolChoice,
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
	record.HTTPStatus = resp.StatusCode
	body, err := io.ReadAll(resp.Body)
	record.Body = string(body)
	if err != nil {
		record.Error = "read_response"
		return completionResponse{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		record.Error = "http_error"
		return completionResponse{}, fmt.Errorf("API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &result); err != nil {
		record.Error = "invalid_json"
		return completionResponse{}, fmt.Errorf("decode response: %w", err)
	}
	result.diagnostic = record
	return result, nil
}

func (c *OpenAICompatibleClient) emptyAnswerError(choice completionChoice) error {
	hint := "провайдер не вернул текст ответа"
	switch choice.FinishReason {
	case "length":
		hint = "достигнут лимит генерации; увеличьте max_tokens или задайте 0"
	case "content_filter":
		hint = "ответ остановлен фильтром провайдера"
	default:
		if choice.Message.ReasoningContent != "" {
			hint = "получены рассуждения без финального ответа"
		}
	}
	return fmt.Errorf("API returned an empty answer (finish_reason=%q, content_bytes=%d, reasoning_bytes=%d, max_tokens=%d): %s",
		choice.FinishReason, len(choice.Message.Content), len(choice.Message.ReasoningContent), c.options.MaxTokens, hint)
}

func newThinkingOption(value string) *thinkingOption {
	if value == "" {
		return nil
	}
	return &thinkingOption{Type: value}
}
