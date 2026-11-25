package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

type launchOptions struct{ dir string }

func parseArgs(args []string, out io.Writer) (launchOptions, error) {
	flags := flag.NewFlagSet("llm-console", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dir := flags.String("dir", ".", "директория агента")
	flags.Usage = func() {
		fmt.Fprintln(out, "Запуск: llm-console [--dir директория]")
		fmt.Fprintln(out, "Диалог с LLM. Введите сообщение после запуска; /exit — выход.")
	}
	if err := flags.Parse(args); err != nil {
		return launchOptions{}, err
	}
	if flags.NArg() > 0 {
		return launchOptions{}, fmt.Errorf("сообщения вводятся в чате после запуска; допустим только --dir")
	}
	if strings.TrimSpace(*dir) == "" {
		return launchOptions{}, fmt.Errorf("после --dir укажите директорию")
	}
	return launchOptions{dir: *dir}, nil
}
