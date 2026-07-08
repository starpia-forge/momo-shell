package secretscan

import "testing"

func TestScan_CuratedRules(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantType string
		wantVal  string
	}{
		{
			name:     "AWS access key ID",
			text:     "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			wantType: "aws-access-key-id",
			wantVal:  "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:     "private key block header",
			text:     "before -----BEGIN RSA PRIVATE KEY----- after",
			wantType: "private-key-block",
			wantVal:  "-----BEGIN RSA PRIVATE KEY-----",
		},
		{
			name:     "GitHub token",
			text:     "token: ghp_abcdefghijklmnopqrstuvwxyz1234567890",
			wantType: "github-token",
			wantVal:  "ghp_abcdefghijklmnopqrstuvwxyz1234567890",
		},
		{
			name:     "Slack token",
			text:     "SLACK_TOKEN=xoxb-123456789012-abcdefghij",
			wantType: "slack-token",
			wantVal:  "xoxb-123456789012-abcdefghij",
		},
		{
			name:     "JWT",
			text:     "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			wantType: "jwt",
			wantVal:  "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		},
	}

	s := New()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hits := s.Scan([]byte(c.text))
			var found *hitMatch
			for _, h := range hits {
				if h.Type == c.wantType {
					found = &hitMatch{h.Start, h.End, h.Value}
					break
				}
			}
			if found == nil {
				t.Fatalf("Scan(%q) = %+v, want a %s hit", c.text, hits, c.wantType)
			}
			if found.value != c.wantVal {
				t.Errorf("hit.Value = %q, want %q", found.value, c.wantVal)
			}
			if c.text[found.start:found.end] != c.wantVal {
				t.Errorf("text[%d:%d] = %q, want %q (offset mismatch)", found.start, found.end, c.text[found.start:found.end], c.wantVal)
			}
		})
	}
}

type hitMatch struct {
	start, end int
	value      string
}

func TestScan_SafeTextHasNoHits(t *testing.T) {
	s := New()
	hits := s.Scan([]byte("ls -la /home/user && echo done"))
	if len(hits) != 0 {
		t.Errorf("Scan(safe text) = %+v, want no hits", hits)
	}
}

func TestScan_EntropySignal(t *testing.T) {
	s := New()

	t.Run("high-entropy random-looking blob is flagged", func(t *testing.T) {
		// A base64-looking string with no repeating structure -- picked to
		// score above the default threshold; verified empirically via this
		// test (entropy has no closed-form "obviously high" literal).
		blob := "K7gNU3sd9oQL0zNhqoVWhr3g6s1xYv72olZpeYUnbXcMdRj4"
		hits := s.Scan([]byte("token: " + blob)) // space breaks the candidate boundary before blob
		found := false
		for _, h := range hits {
			if h.Type == "generic-high-entropy" && h.Value == blob {
				found = true
			}
		}
		if !found {
			t.Errorf("Scan() = %+v, want a generic-high-entropy hit for %q", hits, blob)
		}
	})

	t.Run("low-entropy repeated pattern is not flagged", func(t *testing.T) {
		hits := s.Scan([]byte("abcabcabcabcabcabcabcabcabcabcabcabc"))
		for _, h := range hits {
			if h.Type == "generic-high-entropy" {
				t.Errorf("Scan(repeated pattern) = %+v, want no generic-high-entropy hit", hits)
			}
		}
	})

	t.Run("single repeated character is not flagged", func(t *testing.T) {
		hits := s.Scan([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
		for _, h := range hits {
			if h.Type == "generic-high-entropy" {
				t.Errorf("Scan(all-same-char) = %+v, want no generic-high-entropy hit", hits)
			}
		}
	})

	t.Run("short candidate below length floor is never checked", func(t *testing.T) {
		hits := s.Scan([]byte("aK9x2Q")) // 6 chars, well under the 20-char candidate floor
		if len(hits) != 0 {
			t.Errorf("Scan(short token) = %+v, want no hits", hits)
		}
	})
}

func TestScan_RuleHitSuppressesOverlappingEntropyHit(t *testing.T) {
	s := New()
	// AKIAIOSFODNN7EXAMPLE is exactly 20 chars -- long enough to also match
	// entropyCandidate. It must be reported once (as the rule hit), not
	// twice.
	hits := s.Scan([]byte("AKIAIOSFODNN7EXAMPLE"))

	var ruleHits, entropyHits int
	for _, h := range hits {
		switch h.Type {
		case "aws-access-key-id":
			ruleHits++
		case "generic-high-entropy":
			entropyHits++
		}
	}
	if ruleHits != 1 {
		t.Errorf("got %d aws-access-key-id hits, want 1", ruleHits)
	}
	if entropyHits != 0 {
		t.Errorf("got %d generic-high-entropy hits overlapping the rule match, want 0", entropyHits)
	}
}

func TestNewWithEntropyThreshold_OverridesDefault(t *testing.T) {
	blob := "K7gNU3sd9oQL0zNhqoVWhr3g6s1xYv72olZpeYUnbXcMdRj4"

	t.Run("very high threshold suppresses a hit the default would flag", func(t *testing.T) {
		s := NewWithEntropyThreshold(7.9) // above log2(256) is impossible to reach -- always suppresses
		hits := s.Scan([]byte(blob))
		for _, h := range hits {
			if h.Type == "generic-high-entropy" {
				t.Errorf("Scan() with threshold 7.9 = %+v, want no generic-high-entropy hit", hits)
			}
		}
	})

	t.Run("very low threshold flags a candidate the default would not", func(t *testing.T) {
		s := NewWithEntropyThreshold(0.1)
		hits := s.Scan([]byte("abcabcabcabcabcabcabcabcabcabcabcabc")) // low-entropy but non-empty
		found := false
		for _, h := range hits {
			if h.Type == "generic-high-entropy" {
				found = true
			}
		}
		if !found {
			t.Errorf("Scan() with threshold 0.1 = %+v, want a generic-high-entropy hit", hits)
		}
	})
}

func TestScan_MultipleHitsInOneText(t *testing.T) {
	s := New()
	text := "AKIAIOSFODNN7EXAMPLE and also ghp_abcdefghijklmnopqrstuvwxyz1234567890"
	hits := s.Scan([]byte(text))

	var gotAWS, gotGitHub bool
	for _, h := range hits {
		switch h.Type {
		case "aws-access-key-id":
			gotAWS = true
		case "github-token":
			gotGitHub = true
		}
	}
	if !gotAWS || !gotGitHub {
		t.Errorf("Scan(%q) = %+v, want both an aws-access-key-id and a github-token hit", text, hits)
	}
}
