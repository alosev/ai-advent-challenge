package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"llm-console/internal/history"
	"llm-console/internal/llm"
)

type asker interface {
	Ask(context.Context, string) (llm.Result, error)
	MemoryCommand(string) (string, error)
	ProfileCommand(string) (string, error)
	AgentCommand(string) (string, error)
	TaskCommand(string) (string, error)
	InvariantCommand(string) (string, error)
	Clear() error
	TotalUsage() *llm.Usage
	History() history.State
}

type console struct {
	out   io.Writer
	color bool
}

func newConsole(out *os.File) console {
	return console{out: out, color: isTerminal(out) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"}
}

func (c console) observeTools(client *llm.OpenAICompatibleClient) {
	client.SetToolCallObserver(c.toolCall)
	client.SetToolResultObserver(func(call llm.ToolCall, output string) {
		c.toolResult(call.Function.Name, output)
	})
}

func (c console) toolCall(call llm.ToolCall) {
	c.block("ВЫЗОВ ИНСТРУМЕНТА · "+call.Function.Name, call.Function.Arguments, "33")
}

func (c console) toolResult(name, output string) {
	c.block("РЕЗУЛЬТАТ ИНСТРУМЕНТА · "+name, output, "35")
}

func (c console) history(state history.State) {
	toolNames := make(map[string]string)
	var displayedTotal *llm.Usage
	for _, message := range state.Messages {
		switch message.Role {
		case "user":
			c.heading("ВЫ", "1;36")
			fmt.Fprintf(c.out, "> %s\n", message.Content)
		case "assistant":
			if len(message.ToolCalls) == 0 {
				c.answer(llm.Result{Content: message.Content, Usage: message.Usage, MemorySaved: message.MemorySaved})
				c.totalUsage(message.TotalUsage)
				displayedTotal = message.TotalUsage
				fmt.Fprintln(c.out)
			} else if strings.TrimSpace(message.Content) != "" {
				c.block("АССИСТЕНТ", message.Content, "1;32")
			}
			for _, call := range message.ToolCalls {
				toolNames[call.ID] = call.Function.Name
				c.toolCall(call)
			}
		case "tool":
			name := toolNames[message.ToolCallID]
			if name == "" {
				name = message.ToolCallID
			}
			c.toolResult(name, message.Content)
		}
	}
	// A failed turn can add token usage without adding an assistant message.
	if state.Usage != nil && (displayedTotal == nil || *displayedTotal != *state.Usage) {
		fmt.Fprintln(c.out)
		c.totalUsage(state.Usage)
		fmt.Fprintln(c.out)
	}
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
	if saved := result.MemorySaved; saved != nil && (saved.Working || saved.LongTerm) {
		fmt.Fprintln(c.out)
		if saved.Working {
			fmt.Fprintln(c.out, "📝 Рабочая память обновлена")
		}
		if saved.LongTerm {
			fmt.Fprintln(c.out, "🧠 Долговременная память обновлена")
		}
	}
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
	fmt.Fprintln(ui.out, "Диалог · Enter — отправить · /multi — многострочный ввод · /clear — очистить историю · /exit — выход")
	fmt.Fprintln(ui.out, "Диалог и память обновляются автоматически. /memory — просмотр и ручное редактирование памяти.")
	fmt.Fprintln(ui.out, "/agent — текущий агент · /agent <директория> — переключить · /profile — профиль агента")
	fmt.Fprintln(ui.out, "/task — необязательная задача · /task list — процессы · /approve — подтвердить текущий шаг")
	fmt.Fprintln(ui.out, "/invariant — правила из invariants.json · /invariant history — журнал изменений")
	agentOutput, _ := chat.AgentCommand("/agent")
	ui.block("АГЕНТ", agentOutput, "34")
	profileOutput, profileErr := chat.ProfileCommand("/profile")
	if profileErr == nil {
		ui.block("ПРОФИЛЬ", profileOutput, "34")
	}
	ui.history(chat.History())
	if rules, rulesErr := chat.InvariantCommand("/invariant"); rulesErr == nil {
		ui.block("ДЕЙСТВУЮЩИЕ ИНВАРИАНТЫ", rules, "34")
	}
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
		if fields := strings.Fields(prompt); len(fields) > 0 && fields[0] == "/invariant" {
			output, commandErr := chat.InvariantCommand(prompt)
			if commandErr != nil {
				ui.block("ОШИБКА ИНВАРИАНТОВ", commandErr.Error(), "1;31")
			} else {
				ui.block("ИНВАРИАНТЫ", output, "34")
			}
			if err == io.EOF {
				return nil
			}
			continue
		}
		if fields := strings.Fields(prompt); len(fields) > 0 && (fields[0] == "/task" || fields[0] == "/approve") {
			output, taskErr := chat.TaskCommand(prompt)
			if taskErr != nil {
				ui.block("ОШИБКА ЗАДАЧИ", taskErr.Error(), "1;31")
			} else {
				ui.block("ЗАДАЧА", output, "34")
			}
			if err == io.EOF {
				return nil
			}
			continue
		}
		if fields := strings.Fields(prompt); len(fields) > 0 && fields[0] == "/agent" {
			output, switchErr := chat.AgentCommand(prompt)
			if switchErr != nil {
				ui.block("ОШИБКА ПЕРЕКЛЮЧЕНИЯ", switchErr.Error(), "1;31")
			} else {
				ui.block("АГЕНТ", output, "34")
				profileOutput, _ := chat.ProfileCommand("/profile")
				ui.block("ПРОФИЛЬ", profileOutput, "34")
				ui.history(chat.History())
				if rules, rulesErr := chat.InvariantCommand("/invariant"); rulesErr == nil {
					ui.block("ДЕЙСТВУЮЩИЕ ИНВАРИАНТЫ", rules, "34")
				}
			}
			if err == io.EOF {
				return nil
			}
			continue
		}
		if fields := strings.Fields(prompt); len(fields) > 0 && fields[0] == "/profile" {
			output, profileErr := chat.ProfileCommand(prompt)
			if profileErr != nil {
				ui.block("ОШИБКА ПРОФИЛЯ", profileErr.Error(), "1;31")
			} else {
				ui.block("ПРОФИЛЬ", output, "34")
			}
			if err == io.EOF {
				return nil
			}
			continue
		}
		if fields := strings.Fields(prompt); len(fields) > 0 && fields[0] == "/memory" {
			output, memoryErr := chat.MemoryCommand(prompt)
			if memoryErr != nil {
				ui.block("ОШИБКА ПАМЯТИ", memoryErr.Error(), "1;31")
			} else {
				ui.block("ПАМЯТЬ", output, "34")
			}
			if err == io.EOF {
				return nil
			}
			continue
		}
		if prompt == "/clear" {
			if err := chat.Clear(); err != nil {
				ui.block("ОШИБКА ОЧИСТКИ", err.Error(), "1;31")
			} else {
				ui.block("ИСТОРИЯ ОЧИЩЕНА", "Переписка и саммари удалены, счётчик токенов сброшен. Рабочая и долговременная память сохранены.", "34")
			}
			if err == io.EOF {
				return nil
			}
			continue
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
			if answer.Content != "" {
				ui.answer(answer)
			} else {
				ui.usage(answer.Usage)
			}
			if askErr != nil {
				ui.block("ОШИБКА", askErr.Error(), "1;31")
			}
			ui.totalUsage(chat.TotalUsage())
			fmt.Fprintln(ui.out)
		}
		if err == io.EOF {
			return nil
		}
	}
}
