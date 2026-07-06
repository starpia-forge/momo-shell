package wails

import "momo-shell/internal/core/port/in"

// AppInfoDTO is the JSON-facing response DTO for the settings page's
// "정보" tab.
type AppInfoDTO struct {
	Version string `json:"version"`
}

// SettingsService is the Wails-bound facade over in.SettingsUseCase. It
// only converts between JSON-facing DTOs and domain types -- no business
// logic.
type SettingsService struct {
	uc      in.SettingsUseCase
	version string
}

func NewSettingsService(uc in.SettingsUseCase, version string) *SettingsService {
	return &SettingsService{uc: uc, version: version}
}

func (s *SettingsService) GetAll() (map[string]string, error) {
	return s.uc.GetAll()
}

func (s *SettingsService) Set(key string, value string) error {
	return s.uc.Set(key, value)
}

func (s *SettingsService) AppInfo() AppInfoDTO {
	return AppInfoDTO{Version: s.version}
}
