package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"llm-console/internal/app"
	"llm-console/internal/config"
	"llm-console/internal/llm"
)

func main() {
	configPath, promptArgs, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка аргументов:", err)
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка конфигурации:", err)
		os.Exit(1)
	}

	options := llm.CompletionOptions{
		MaxTokens:       cfg.MaxTokens,
		Temperature:     cfg.Temperature,
		Thinking:        cfg.Thinking,
		ReasoningEffort: cfg.ReasoningEffort,
		StopSequences:   cfg.StopSequences,
	}
	if len(cfg.ResponseSchema) > 0 {
		options.ResponseSchema = &llm.ResponseSchema{
			Schema: cfg.ResponseSchema,
		}
	}
	client := llm.NewOpenAICompatibleClient(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.Timeout, options)
	client.SetToolCallObserver(func(call llm.ToolCall) {
		fmt.Fprintf(os.Stderr, "Вызов инструмента: %s\nАргументы: %s\n", call.Function.Name, call.Function.Arguments)
	})
	chat := app.NewChatService(client)

	prompt, err := readPrompt(os.Stdin, promptArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка чтения запроса:", err)
		os.Exit(1)
	}

	answer, err := chat.Ask(context.Background(), prompt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка запроса к LLM:", err)
		os.Exit(1)
	}
	fmt.Println(answer.Content)
	if answer.Usage != nil {
		fmt.Fprintf(os.Stderr, "Токены: всего %d (вход: %d, выход: %d)\n", answer.Usage.TotalTokens, answer.Usage.PromptTokens, answer.Usage.CompletionTokens)
	} else {
		fmt.Fprintln(os.Stderr, "Данные о расходе токенов недоступны")
	}
}

func parseArgs(args []string) (configPath string, promptArgs []string, err error) {
	configPath = "config.json"
	for len(args) > 0 {
		if args[0] != "--config" {
			return configPath, args, nil
		}
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return "", nil, fmt.Errorf("после --config укажите путь к файлу")
		}
		configPath = args[1]
		args = args[2:]
	}
	return configPath, args, nil
}

func readPrompt(in io.Reader, args []string) (string, error) {
	if len(args) > 0 {
		prompt := strings.TrimSpace(strings.Join(args, " "))
		if prompt == "" {
			return "", fmt.Errorf("запрос не должен быть пустым")
		}
		return prompt, nil
	}

	fmt.Fprintln(os.Stderr, "Введите запрос. Для отправки нажмите Ctrl+D на новой строке (Windows: Ctrl+Z, затем Enter):")
	data, err := io.ReadAll(in)
	if err != nil {
		return "", err
	}
	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", fmt.Errorf("запрос не должен быть пустым")
	}
	return prompt, nil
}
