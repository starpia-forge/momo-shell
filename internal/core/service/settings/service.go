package settings

import (
	"fmt"
	"strconv"

	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

const (
	KeyTheme      = "appearance.theme"
	KeyAccent     = "appearance.accent"
	KeyFontSize   = "terminal.fontSize"
	KeyScrollback = "terminal.scrollback"
	KeyLanguage   = "general.language"
)

const (
	minFontSize = 8
	maxFontSize = 32

	minScrollback = 1000
	maxScrollback = 100000
)

// defaults holds every known key's default value -- GetAll always returns
// the full set, with stored values overlaid on top.
var defaults = map[string]string{
	KeyTheme:      "dark",
	KeyAccent:     "pink",
	KeyFontSize:   "14",
	KeyScrollback: "10000",
	KeyLanguage:   "system",
}

var validAccents = map[string]bool{
	"pink":   true,
	"orange": true,
	"purple": true,
	"blue":   true,
}

var validLanguages = map[string]bool{
	"system": true,
	"ko":     true,
	"en":     true,
	"zh":     true,
	"ja":     true,
}

// Service implements in.SettingsUseCase: key validation and default-merging
// on top of a flat out.SettingsRepository KV store.
type Service struct {
	repo out.SettingsRepository
}

var _ in.SettingsUseCase = (*Service)(nil)

func New(repo out.SettingsRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetAll() (map[string]string, error) {
	stored, err := s.repo.All()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(defaults))
	for key, value := range defaults {
		result[key] = value
	}
	for key, value := range stored {
		if _, known := defaults[key]; known {
			result[key] = value
		}
	}
	return result, nil
}

func (s *Service) Set(key, value string) error {
	if err := validate(key, value); err != nil {
		return err
	}
	return s.repo.Set(key, value)
}

func validate(key, value string) error {
	switch key {
	case KeyTheme:
		if value != "dark" && value != "light" {
			return fmt.Errorf("settings: invalid theme %q", value)
		}
	case KeyAccent:
		if !validAccents[value] {
			return fmt.Errorf("settings: invalid accent %q", value)
		}
	case KeyFontSize:
		return validateIntRange(key, value, minFontSize, maxFontSize)
	case KeyScrollback:
		return validateIntRange(key, value, minScrollback, maxScrollback)
	case KeyLanguage:
		if !validLanguages[value] {
			return fmt.Errorf("settings: invalid language %q", value)
		}
	default:
		return fmt.Errorf("settings: unknown key %q", key)
	}
	return nil
}

func validateIntRange(key, value string, min, max int) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("settings: %s must be an integer: %w", key, err)
	}
	if n < min || n > max {
		return fmt.Errorf("settings: %s must be between %d and %d, got %d", key, min, max, n)
	}
	return nil
}
