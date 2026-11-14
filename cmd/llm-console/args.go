package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

func parseArgs(args []string, out io.Writer) (string, error) {
	flags := flag.NewFlagSet("llm-console", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "config.json", "файл конфигурации")
	flags.Usage = func() {
		fmt.Fprintln(out, "Запуск: llm-console [--config config.json]")
		fmt.Fprintln(out, "Диалог с LLM. Введите сообщение после запуска; /exit — выход.")
	}
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() > 0 {
		return "", fmt.Errorf("сообщения вводятся в чате после запуска; допустим только --config")
	}
	if strings.TrimSpace(*configPath) == "" {
		return "", fmt.Errorf("после --config укажите путь к файлу")
	}
	return *configPath, nil
}
