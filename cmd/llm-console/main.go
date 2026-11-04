package main

import (
	"bufio"
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
		MaxTokens:     cfg.MaxTokens,
		StopSequences: cfg.StopSequences,
	}
	if len(cfg.ResponseSchema) > 0 {
		options.ResponseSchema = &llm.ResponseSchema{
			Schema: cfg.ResponseSchema,
		}
	}
	client := llm.NewOpenAICompatibleClient(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.Timeout, options)
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
	fmt.Println(answer)
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
		return strings.TrimSpace(strings.Join(args, " ")), nil
	}

	fmt.Print("Ваш вопрос: ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	prompt := strings.TrimSpace(line)
	if prompt == "" {
		return "", fmt.Errorf("запрос не должен быть пустым")
	}
	return prompt, nil
}
