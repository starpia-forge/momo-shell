package history

import "strings"

// maskSubstrings is a fixed v1 heuristic list (no settings UI exists yet to
// customize it) -- a case-insensitive match against the full command line
// means it's dropped from history entirely, never persisted.
var maskSubstrings = []string{
	"password=", "passwd=", "pwd=",
	"token=", "secret=", "apikey=", "api_key=", "api-key=",
	"sshpass -p",
}

// shouldMask reports whether command matches a sensitive-value heuristic.
func shouldMask(command string) bool {
	lower := strings.ToLower(command)
	for _, s := range maskSubstrings {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
