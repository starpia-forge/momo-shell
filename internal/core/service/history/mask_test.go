package history

import "testing"

func TestShouldMask(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"ls -la", false},
		{"echo hello", false},
		{"export DB_PASSWORD=hunter2", true},
		{"curl -u user:hunter2 https://example.com", false},
		{"curl -H \"Authorization: token=abc123\" https://api.example.com", true},
		{"export MY_SECRET=xyz", true},
		{"export API_KEY=xyz", true},
		{"sshpass -p hunter2 ssh user@host", true},
		{"git commit -m 'fix pwd=typo in docs'", true}, // heuristic false positive, accepted for v1
	}

	for _, tc := range cases {
		if got := shouldMask(tc.command); got != tc.want {
			t.Errorf("shouldMask(%q) = %v, want %v", tc.command, got, tc.want)
		}
	}
}
