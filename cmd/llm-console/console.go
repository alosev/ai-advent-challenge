package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"llm-console/internal/llm"
)

type asker interface {
	Ask(context.Context, string) (llm.Result, error)
}

type console struct {
	out   io.Writer
	color bool
}

func newConsole(out *os.File) console {
	return console{out: out, color: isTerminal(out) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"}
}

func (c console) observeTools(client *llm.OpenAICompatibleClient) {
	client.SetToolCallObserver(func(call llm.ToolCall) {
		c.block("ВЫЗОВ ИНСТРУМЕНТА · "+call.Function.Name, call.Function.Arguments, "33")
	})
	client.SetToolResultObserver(func(call llm.ToolCall, output string) {
		c.block("РЕЗУЛЬТАТ ИНСТРУМЕНТА · "+call.Function.Name, output, "35")
	})
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func (c console) heading(label, color string) {
	if c.color {
		fmt.Fprintf(c.out, "\033[%sm── %s ──\033[0m\n", color, label)
	} else {
		fmt.Fprintf(c.out, "── %s ──\n", label)
	}
}

func (c console) block(label, content, color string) {
	fmt.Fprintln(c.out)
	c.heading(label, color)
	fmt.Fprintln(c.out, content)
}

func (c console) answer(result llm.Result) {
	c.block("АССИСТЕНТ", result.Content, "1;32")
	c.usage(result.Usage)
}

func (c console) usage(usage *llm.Usage) {
	fmt.Fprintln(c.out)
	c.usageLine("Токены за ответ", usage)
}

func (c console) totalUsage(total *llm.Usage) {
	c.usageLine("Токены за диалог (по данным API)", total)
}

func (c console) usageLine(label string, usage *llm.Usage) {
	if c.color {
		fmt.Fprint(c.out, "\033[34m")
	}
	if usage == nil {
		fmt.Fprintf(c.out, ">>> %s: данные недоступны", label)
	} else {
		fmt.Fprintf(c.out, ">>> %s: %d (вход: %d, выход: %d)",
			label, usage.TotalTokens, usage.PromptTokens, usage.CompletionTokens)
	}
	if c.color {
		fmt.Fprint(c.out, "\033[0m")
	}
	fmt.Fprintln(c.out)
}

// runChat uses line-oriented input; multiline messages end with /send.
func runChat(ctx context.Context, in io.Reader, ui console, chat asker) error {
	reader := bufio.NewReader(in)
	var total *llm.Usage
	fmt.Fprintln(ui.out, "Диалог · Enter — отправить · /multi — многострочный ввод · /exit — выход")
	fmt.Fprintln(ui.out, "Контекст сохраняется только до выхода из программы.")
	for {
		ui.heading("ВЫ", "1;36")
		fmt.Fprint(ui.out, "> ")
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		prompt := strings.TrimSpace(line)
		if prompt == "/exit" || prompt == "/quit" {
			return nil
		}
		if prompt == "/multi" {
			fmt.Fprintln(ui.out, "Введите сообщение; /send на отдельной строке — отправить, /cancel — отменить.")
			var lines []string
			for {
				line, err = reader.ReadString('\n')
				if err != nil && err != io.EOF {
					return err
				}
				command := strings.TrimSpace(line)
				if command == "/cancel" {
					lines = nil
					break
				}
				if command == "/send" {
					break
				}
				if err == io.EOF {
					return nil
				}
				lines = append(lines, strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"))
			}
			prompt = strings.TrimSpace(strings.Join(lines, "\n"))
		}
		if prompt != "" {
			answer, askErr := chat.Ask(ctx, prompt)
			if answer.Usage != nil {
				if total == nil {
					total = &llm.Usage{}
				}
				total.TotalTokens += answer.Usage.TotalTokens
				total.PromptTokens += answer.Usage.PromptTokens
				total.CompletionTokens += answer.Usage.CompletionTokens
			}
			if askErr != nil {
				ui.block("ОШИБКА", askErr.Error(), "1;31")
				ui.usage(answer.Usage)
			} else {
				ui.answer(answer)
			}
			ui.totalUsage(total)
			fmt.Fprintln(ui.out)
		}
		if err == io.EOF {
			return nil
		}
	}
}
