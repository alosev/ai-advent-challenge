package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"llm-console/internal/app"
	"llm-console/internal/config"
	"llm-console/internal/history"
	"llm-console/internal/llm"
	"llm-console/internal/memory"
)

type agentSession struct {
	*app.ChatService
	logger *llm.ResponseLogger
}

func (s *agentSession) close() {
	if s.logger != nil {
		if err := s.logger.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка закрытия лога API:", err)
		}
	}
}

func loadSession(configPath string, ui console) (*agentSession, error) {
	session := &agentSession{}
	defer func() {
		if session.ChatService == nil {
			session.close()
		}
	}()
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка конфигурации: %w", err)
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
			return nil, fmt.Errorf("ошибка открытия лога API: %w", err)
		}
		session.logger = logger
		client.SetResponseObserver(func(record llm.ResponseRecord) {
			if err := logger.Write(record); err != nil {
				fmt.Fprintln(os.Stderr, "Не удалось записать ответ в лог API:", err)
			}
		})
	}
	chat, err := app.NewChatServiceWithHistory(app.NewMemoryClient(client), history.NewJSONStore(cfg.HistoryPath))
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки истории (файл оставлен без изменений): %w", err)
	}
	chat.SetSummaryClient(client.SummaryClient())
	if err := chat.SetProfile(cfg.Profile); err != nil {
		return nil, fmt.Errorf("ошибка профилей: %w", err)
	}
	if err := chat.SetMemoryStores(memory.NewJSONStore(cfg.WorkingMemoryPath), memory.NewJSONStore(cfg.LongTermMemoryPath)); err != nil {
		return nil, fmt.Errorf("ошибка загрузки памяти (файлы оставлены без изменений): %w", err)
	}
	ui.observeTools(client)
	session.ChatService = chat
	return session, nil
}

// Each loaded agent retains its own service, including unsaved in-memory state.
type agentSessions struct {
	*agentSession
	sessions map[string]*agentSession
	current  string
	baseDir  string
	ui       console
}

func newAgentSessions(options launchOptions, ui console) (*agentSessions, error) {
	base, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sessions := &agentSessions{sessions: make(map[string]*agentSession), baseDir: base, ui: ui}
	if err := sessions.use(options.dir); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (s *agentSessions) use(directory string) error {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	// A directory identifies an agent, including when entered through an alias.
	directory, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return fmt.Errorf("не удалось открыть директорию агента: %w", err)
	}
	session := s.sessions[directory]
	if session == nil {
		session, err = loadSession(filepath.Join(directory, "config.json"), s.ui)
		if err != nil {
			return err
		}
		s.sessions[directory] = session
	}
	s.agentSession, s.current = session, directory
	return nil
}

func (s *agentSessions) AgentCommand(command string) (string, error) {
	directory := strings.TrimSpace(strings.TrimPrefix(command, "/agent"))
	if directory != "" {
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(s.baseDir, directory)
		}
		if err := s.use(directory); err != nil {
			return "", err
		}
	}
	path, err := filepath.Rel(s.baseDir, s.current)
	if err != nil {
		path = s.current
	}
	return "Директория агента: " + path + "\n/agent <директория> — переключить; путь относительно каталога запуска.", nil
}

func (s *agentSessions) close() {
	for _, session := range s.sessions {
		session.close()
	}
}
