package out

// SettingsRepository persists app-wide settings (appearance, terminal
// defaults) as a flat key-value store. Unlike ShareSettings, this port
// exposes the raw KV surface directly -- validation and default-merging
// live in core/service/settings, not the repo.
type SettingsRepository interface {
	Get(key string) (value string, found bool, err error)
	Set(key, value string) error
	All() (map[string]string, error)
}
