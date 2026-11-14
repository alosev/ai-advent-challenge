package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"llm-console/internal/app"
	"llm-console/internal/config"
	"llm-console/internal/llm"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	configPath, err := parseArgs(args, os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ошибка аргументов: %w", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("ошибка конфигурации: %w", err)
	}

	options := llm.CompletionOptions{
		MaxTokens:       cfg.MaxTokens,
		Temperature:     cfg.Temperature,
		Thinking:        cfg.Thinking,
		ReasoningEffort: cfg.ReasoningEffort,
		StopSequences:   cfg.StopSequences,
	}
	if len(cfg.ResponseSchema) > 0 {
		options.ResponseSchema = &llm.ResponseSchema{Schema: cfg.ResponseSchema}
	}
	client := llm.NewOpenAICompatibleClient(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.Timeout, options)
	chat := app.NewChatService(client)
	ui := newConsole(os.Stdout)
	ui.observeTools(client)
	if err := runChat(context.Background(), os.Stdin, ui, chat); err != nil {
		return fmt.Errorf("ошибка чтения: %w", err)
	}
	return nil
}
