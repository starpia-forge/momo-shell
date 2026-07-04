package out

// SecretStore holds secret material (passwords, key passphrases) outside
// of the Host record, referenced only by an opaque ref string (e.g.
// "host:{id}"). Adapters choose the backend (OS keychain, encrypted file).
type SecretStore interface {
	Set(ref string, secret []byte) error
	Get(ref string) ([]byte, error)
	Delete(ref string) error
}
