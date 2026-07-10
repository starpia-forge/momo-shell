package shellintegration

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"

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
	if !strings.HasSuffix(s, "\r") || strings.HasSuffix(s, "\r\n") {
		t.Fatalf("expected script to end in a lone CR (PSReadLine requires CR to submit; a trailing LF after it corrupts the next prompt), got %q", s)
	}
}

func TestHookScript_UnknownDialect(t *testing.T) {
	b := New()
	if _, err := b.HookScript(out.DialectUnknown, "n"); err == nil {
		t.Fatal("expected an error for an unsupported dialect, got nil")
	}
}

func TestProbeScript_Bash(t *testing.T) {
	b := New()
	script, err := b.ProbeScript(out.DialectBash, "abc123", []string{"FOO", "BAR"})
	if err != nil {
		t.Fatalf("ProbeScript failed: %v", err)
	}
	s := string(script)

	if !strings.HasPrefix(s, "__momo_pc=$?; ") {
		t.Fatalf("expected script to capture $? first, got %q", s)
	}
	if !strings.Contains(s, "for __momo_v in FOO BAR; do") {
		t.Fatalf("expected the variable list interpolated as a bash word list, got %q", s)
	}
	if !strings.Contains(s, `${!__momo_v+x}`) {
		t.Fatalf("expected the unset-vs-set existence test, got %q", s)
	}
	if !strings.Contains(s, `"\033]1337;momo;probe;abc123;%s\007"`) {
		t.Fatalf("expected the probe marker with the nonce interpolated, got %q", s)
	}
	if !strings.Contains(s, "( exit $__momo_pc )") {
		t.Fatalf("expected $? to be restored via a subshell exit, got %q", s)
	}
}

func TestProbeScript_PowerShell(t *testing.T) {
	b := New()
	script, err := b.ProbeScript(out.DialectPowerShell, "xyz789", []string{"FOO", "BAR"})
	if err != nil {
		t.Fatalf("ProbeScript failed: %v", err)
	}
	s := string(script)

	if !strings.HasPrefix(s, `$__momo_pc = $LASTEXITCODE; `) {
		t.Fatalf("expected script to capture $LASTEXITCODE first, got %q", s)
	}
	if !strings.Contains(s, `foreach ($__momo_v in @("FOO","BAR"))`) {
		t.Fatalf("expected the variable list interpolated as a PowerShell array literal, got %q", s)
	}
	if !strings.Contains(s, `Test-Path "variable:$__momo_v"`) {
		t.Fatalf("expected the unset-vs-set existence test, got %q", s)
	}
	if !strings.Contains(s, `"]1337;momo;probe;xyz789;"`) {
		t.Fatalf("expected the probe marker with the nonce interpolated, got %q", s)
	}
	if !strings.Contains(s, "$global:LASTEXITCODE = $__momo_pc") {
		t.Fatalf("expected $LASTEXITCODE to be restored, got %q", s)
	}
	if !strings.HasSuffix(s, "\r") || strings.HasSuffix(s, "\r\n") {
		t.Fatalf("expected script to end in a lone CR (PSReadLine requires CR to submit; a trailing LF after it corrupts the next prompt), got %q", s)
	}
}

func TestProbeScript_UnknownDialect(t *testing.T) {
	b := New()
	if _, err := b.ProbeScript(out.DialectUnknown, "n", []string{"X"}); err == nil {
		t.Fatal("expected an error for an unsupported dialect, got nil")
	}
}

func TestSpawnArgs_PowerShell(t *testing.T) {
	b := New()
	args, err := b.SpawnArgs(out.DialectPowerShell, "xyz789")
	if err != nil {
		t.Fatalf("SpawnArgs failed: %v", err)
	}
	if len(args) != 4 || args[0] != "-NoLogo" || args[1] != "-NoExit" || args[2] != "-EncodedCommand" {
		t.Fatalf("expected [-NoLogo -NoExit -EncodedCommand <b64>], got %v", args)
	}

	raw, err := base64.StdEncoding.DecodeString(args[3])
	if err != nil {
		t.Fatalf("-EncodedCommand payload isn't valid base64: %v", err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("expected an even number of bytes (UTF-16LE code units), got %d", len(raw))
	}
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE {
		t.Fatal("expected no UTF-16LE BOM -- -EncodedCommand wants bare UTF-16LE")
	}

	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	s := string(utf16.Decode(units))

	if !strings.Contains(s, "function global:prompt {") {
		t.Fatalf("expected the prompt override, got %q", s)
	}
	if !strings.Contains(s, "$global:__momo_p0 = $function:prompt") {
		t.Fatalf("expected the original prompt captured for chaining (preserves the user's real prompt), got %q", s)
	}
	if !strings.Contains(s, "hookinstalled;xyz789") {
		t.Fatalf("expected the hookinstalled marker with the nonce interpolated, got %q", s)
	}
	if strings.Contains(s, "momostart_") {
		t.Fatalf("expected no momostart_ sentinel -- nothing is typed/echoed for spawn-time injection, got %q", s)
	}
	if strings.HasSuffix(s, "\r") || strings.HasSuffix(s, "\n") {
		t.Fatalf("expected no trailing CR/LF -- there's no PSReadLine submission to trigger, got %q", s)
	}
}

func TestSpawnArgs_NonPowerShell(t *testing.T) {
	b := New()
	if _, err := b.SpawnArgs(out.DialectBash, "n"); err == nil {
		t.Fatal("expected an error for bash (no spawn-time injection strategy)")
	}
	if _, err := b.SpawnArgs(out.DialectUnknown, "n"); err == nil {
		t.Fatal("expected an error for an unknown dialect")
	}
}
