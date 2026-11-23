package profile

import (
	"fmt"
	"strings"
)

// Profile is a static instruction edited only by the user in configuration.
type Profile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Normalize validates a copy so callers never retain partially checked profiles.
func Normalize(profiles []Profile, active string) ([]Profile, string, error) {
	result := append([]Profile(nil), profiles...)
	seen := make(map[string]bool, len(result))
	for i := range result {
		p := &result[i]
		p.Name = strings.TrimSpace(p.Name)
		p.Description = strings.TrimSpace(p.Description)
		if p.Name == "" || strings.ContainsAny(p.Name, " \t\r\n") || len(strings.Fields(p.Name)) != 1 {
			return nil, "", fmt.Errorf("имя профиля должно быть непустым и без пробелов")
		}
		if p.Description == "" {
			return nil, "", fmt.Errorf("у профиля %q должно быть непустое описание", p.Name)
		}
		if seen[p.Name] {
			return nil, "", fmt.Errorf("имя профиля %q повторяется", p.Name)
		}
		seen[p.Name] = true
	}
	active = strings.TrimSpace(active)
	if active != "" && !seen[active] {
		return nil, "", fmt.Errorf("активный профиль %q отсутствует в profiles", active)
	}
	return result, active, nil
}
