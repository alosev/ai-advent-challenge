package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"llm-console/internal/app"
	"llm-console/internal/config"
	"llm-console/internal/history"
	"llm-console/internal/llm"
	"llm-console/internal/memory"
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
		FinalTool:       app.SubmitResponseTool(),
		MaxTokens:       cfg.MaxTokens,
		Temperature:     cfg.Temperature,
		Thinking:        cfg.Thinking,
		ReasoningEffort: cfg.ReasoningEffort,
	}
	client := llm.NewOpenAICompatibleClient(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.Timeout, options)
	if cfg.APIResponseLogPath != "-" {
		logger, err := llm.NewResponseLogger(cfg.APIResponseLogPath)
		if err != nil {
			return fmt.Errorf("ошибка открытия лога API: %w", err)
		}
		defer logger.Close()
		client.SetResponseObserver(func(record llm.ResponseRecord) {
			if err := logger.Write(record); err != nil {
				fmt.Fprintln(os.Stderr, "Не удалось записать ответ в лог API:", err)
			}
		})
	}
	chat, err := app.NewChatServiceWithHistory(app.NewMemoryClient(client), history.NewJSONStore(cfg.HistoryPath))
	if err != nil {
		return fmt.Errorf("ошибка загрузки истории (файл оставлен без изменений): %w", err)
	}
	chat.SetSummaryClient(client.SummaryClient())
	if err := chat.SetMemoryStores(memory.NewJSONStore(cfg.WorkingMemoryPath), memory.NewJSONStore(cfg.LongTermMemoryPath)); err != nil {
		return fmt.Errorf("ошибка загрузки памяти (файлы оставлены без изменений): %w", err)
	}
	ui := newConsole(os.Stdout)
	ui.observeTools(client)
	if err := runChat(context.Background(), os.Stdin, ui, chat); err != nil {
		return fmt.Errorf("ошибка чтения: %w", err)
	}
	return nil
}
