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

func Normalize(value *Profile) (*Profile, error) {
	if value == nil {
		return nil, nil
	}
	result := *value
	result.Name = strings.TrimSpace(result.Name)
	result.Description = strings.TrimSpace(result.Description)
	if result.Name == "" || len(strings.Fields(result.Name)) != 1 {
		return nil, fmt.Errorf("имя профиля должно быть непустым и без пробелов")
	}
	if result.Description == "" {
		return nil, fmt.Errorf("у профиля %q должно быть непустое описание", result.Name)
	}
	return &result, nil
}
