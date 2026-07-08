package shellintegration

import (
	"strings"
	"testing"

	"momo-shell/internal/core/port/out"
)

func TestHookScript_Bash(t *testing.T) {
	b := New()
	script, err := b.HookScript(out.DialectBash, "abc123")
	if err != nil {
		t.Fatalf("HookScript failed: %v", err)
	}
	s := string(script)

	if !strings.HasPrefix(s, "printf 'momostart_abc123\\n'; ") {
		t.Fatalf("expected script to start with the momostart sentinel, got %q", s)
	}
	if !strings.Contains(s, `PROMPT_COMMAND='__momo_ec=$?;`) {
		t.Fatalf("expected PROMPT_COMMAND hook, got %q", s)
	}
	if !strings.Contains(s, `PS0='\033]133;C\007'`) {
		t.Fatalf("expected PS0 command-start hook, got %q", s)
	}
	if !strings.Contains(s, "HISTCONTROL=ignorespace") {
		t.Fatalf("expected HISTCONTROL=ignorespace for history non-contamination, got %q", s)
	}
	if !strings.HasSuffix(s, `hookinstalled;abc123\007"`+"\n") {
		t.Fatalf("expected script to end with the hookinstalled marker + newline, got %q", s)
	}
}

func TestHookScript_PowerShell(t *testing.T) {
	b := New()
	script, err := b.HookScript(out.DialectPowerShell, "xyz789")
	if err != nil {
		t.Fatalf("HookScript failed: %v", err)
	}
	s := string(script)

	if !strings.HasPrefix(s, `Write-Host "momostart_xyz789"; `) {
		t.Fatalf("expected script to start with the momostart sentinel, got %q", s)
	}
	if !strings.Contains(s, "function global:prompt {") {
		t.Fatalf("expected prompt function override, got %q", s)
	}
	if !strings.Contains(s, "$__momo_ok = $?") {
		t.Fatalf("expected $? as the primary exit signal, got %q", s)
	}
	if !strings.HasSuffix(s, "\r\n") {
		t.Fatalf("expected script to end in CRLF (PSReadLine requires CR to submit), got %q", s)
	}
}

func TestHookScript_UnknownDialect(t *testing.T) {
	b := New()
	if _, err := b.HookScript(out.DialectUnknown, "n"); err == nil {
		t.Fatal("expected an error for an unsupported dialect, got nil")
	}
}
