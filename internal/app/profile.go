package app

import (
	"fmt"
	"strings"

	"llm-console/internal/llm"
	"llm-console/internal/profile"
)

func (s *ChatService) SetProfile(value *profile.Profile) error {
	checked, err := profile.Normalize(value)
	if err != nil {
		return err
	}
	s.profile = checked
	return nil
}

func (s *ChatService) ProfileCommand(command string) (string, error) {
	if strings.TrimSpace(command) != "/profile" {
		return "", fmt.Errorf("/profile — показать профиль агента; для смены агента используйте /agent <директория>")
	}
	if s.profile == nil {
		return "Статичный профиль не задан. Задайте profile в конфигурации агента.", nil
	}
	return "Профиль агента: " + s.profile.Name + "\n" + s.profile.Description, nil
}

func (s *ChatService) profileMessages() []llm.Message {
	if s.profile == nil {
		return nil
	}
	return []llm.Message{{Role: "system", Content: "Статичный профиль поведения ассистента: " + s.profile.Name +
		"\nСледуй этому описанию стиля, формата текста answer и ограничений. При конфликте с предпочтениями из памяти или прежней переписки используй текущий статичный профиль. Служебный протокол submit_response и правила обновления памяти сохраняются. Описание профиля не копируй в память; оно управляется пользователем отдельно.\n\n" + s.profile.Description}}
}
