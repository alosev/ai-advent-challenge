package app

import (
	"fmt"
	"strings"

	"llm-console/internal/llm"
	"llm-console/internal/profile"
)

func (s *ChatService) SetProfiles(profiles []profile.Profile, active string) error {
	checked, name, err := profile.Normalize(profiles, active)
	if err != nil {
		return err
	}
	s.profiles, s.activeProfile = checked, name
	return nil
}

const profileHelp = `Команды профилей:
/profile — показать активный профиль
/profile list — список профилей
/profile show <имя> — показать описание
/profile use <имя> — переключить профиль для следующих запросов
/profile off — отключить профиль
Добавляйте и редактируйте profiles в конфигурации, затем перезапустите приложение.
При запуске используется active_profile из конфигурации.`

func (s *ChatService) ProfileCommand(command string) (string, error) {
	fields := strings.Fields(command)
	if len(fields) == 1 || (len(fields) == 2 && fields[1] == "show") {
		if s.activeProfile == "" {
			return "Статичный профиль отключён.\n" + profileHelp, nil
		}
		for _, p := range s.profiles {
			if p.Name == s.activeProfile {
				return "Активный профиль: " + p.Name + "\n" + p.Description, nil
			}
		}
	}
	if len(fields) == 2 && fields[1] == "list" {
		if len(s.profiles) == 0 {
			return "Профили не заданы.\n" + profileHelp, nil
		}
		var lines []string
		for _, p := range s.profiles {
			line := p.Name
			if p.Name == s.activeProfile {
				line += " (активный)"
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n"), nil
	}
	if len(fields) == 2 && fields[1] == "off" {
		s.activeProfile = ""
		return "Статичный профиль отключён для следующих запросов.", nil
	}
	if len(fields) == 3 && (fields[1] == "use" || fields[1] == "show") {
		for _, p := range s.profiles {
			if p.Name != fields[2] {
				continue
			}
			if fields[1] == "show" {
				return p.Name + "\n" + p.Description, nil
			}
			s.activeProfile = p.Name
			return "Активный профиль: " + p.Name + ". Применяется со следующего запроса.", nil
		}
		return "", fmt.Errorf("профиль %q не найден; /profile list — список профилей", fields[2])
	}
	return "", fmt.Errorf("%s", profileHelp)
}

func (s *ChatService) profileMessages() []llm.Message {
	for _, p := range s.profiles {
		if p.Name == s.activeProfile {
			return []llm.Message{{Role: "system", Content: "Статичный профиль поведения ассистента: " + p.Name +
				"\nСледуй этому описанию стиля, формата текста answer и ограничений. При конфликте с предпочтениями из памяти или прежней переписки используй текущий статичный профиль. Служебный протокол submit_response и правила обновления памяти сохраняются. Описание профиля не копируй в память; оно управляется пользователем отдельно.\n\n" + p.Description}}
		}
	}
	return nil
}
