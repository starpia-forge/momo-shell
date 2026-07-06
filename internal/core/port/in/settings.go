package in

// SettingsUseCase is the driving port for app-wide settings (appearance,
// terminal defaults), bound to the local Wails UI's settings page.
type SettingsUseCase interface {
	// GetAll returns every known setting key, with stored values overlaid
	// on defaults -- callers never need to know a key's default themselves.
	GetAll() (map[string]string, error)
	// Set validates value against key's expected type/range and persists
	// it. An unknown key or an out-of-range/malformed value is an error.
	Set(key, value string) error
}
