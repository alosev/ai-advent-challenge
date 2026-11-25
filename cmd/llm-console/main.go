package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	options, err := parseArgs(args, os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ошибка аргументов: %w", err)
	}
	ui := newConsole(os.Stdout)
	sessions, err := newAgentSessions(options, ui)
	if err != nil {
		return err
	}
	defer sessions.close()
	if err := runChat(context.Background(), os.Stdin, ui, sessions); err != nil {
		return fmt.Errorf("ошибка чтения: %w", err)
	}
	return nil
}
