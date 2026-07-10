package shellintegration

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// powershellHookTemplate mirrors bashHookTemplate for PowerShell 5.1+
// (verified against Windows PowerShell 5.1 in the S4 spike, doc 19 §3.2).
//
// Local PowerShell sessions no longer use this template -- see
// powershellSpawnHookTemplate below. Typing this line into a running ConPTY
// PowerShell and suppressing its echo desyncs ConPTY's internal screen
// buffer from the terminal frontend's (ConPTY repaints via absolute cursor
// addressing; suppressed rows are still consumed in ConPTY's own buffer, so
// later output lands rows away from where it renders). It's kept only for
// Service.Reinject, which now refuses PowerShell sessions for exactly this
// reason (see shellintegration/service.go).
//
// This MUST end in "\r", not "\n" -- PSReadLine's Enter key binding fires
// on CR; a bare LF is inserted as a literal newline into the multi-line
// edit buffer instead of submitting the command. Doc 19 §3.2 only verified
// that CR is required to submit; it never tested a trailing LF *after* the
// CR. In practice "\r\n" submits the hook line via the CR, but the leftover
// LF byte then lands on the freshly-drawn (empty) next prompt and gets
// inserted as a literal newline there too -- putting the user's very first
// keystrokes into an unwanted PSReadLine continuation (">>") state. A lone
// "\r" avoids this; bash's hook (bash.go) is unaffected and stays LF-only.
//
// $? (not $LASTEXITCODE) is the primary exit signal: $LASTEXITCODE only
// reflects the last NATIVE executable and stays $null after a pure cmdlet
// like Write-Host, which produced a malformed "133;D;" marker (no code)
// before this was corrected during the spike (doc 19 §3.2).
//
// Set-PSReadLineKeyHandler (command-start marker) and
// Set-PSReadLineOption -AddToHistoryHandler (history exclusion) are both
// try/catch-guarded: older bundled PSReadLine (as on this system's
// Windows PowerShell 5.1) doesn't support -AddToHistoryHandler, so history
// non-contamination isn't guaranteed on every PowerShell install (doc 19
// §4.3) -- the prompt/exit-code hook still installs regardless.
const powershellHookTemplate = `Write-Host "momostart_@@NONCE@@"; function global:prompt { $__momo_ok = $?; $__momo_ec = if ($__momo_ok) { 0 } elseif ($LASTEXITCODE) { $LASTEXITCODE } else { 1 }; Write-Host -NoNewline ([char]27 + "]133;D;" + $__momo_ec + [char]7 + [char]27 + "]133;A" + [char]7); return "PS> " }; try { Set-PSReadLineKeyHandler -Key Enter -ScriptBlock { Write-Host -NoNewline ([char]27 + "]133;C" + [char]7); [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine() } } catch { }; try { Set-PSReadLineOption -AddToHistoryHandler { param($line) -not $line.StartsWith('$__momo_pc') } } catch { }; Write-Host -NoNewline ([char]27 + "]1337;momo;hookinstalled;@@NONCE@@" + [char]7)` + "\r"

func powershellHookScript(nonce string) []byte {
	return []byte(strings.ReplaceAll(powershellHookTemplate, "@@NONCE@@", nonce))
}

// powershellProbeTemplate mirrors bashProbeTemplate for PowerShell 5.1+
// (verified in the S4 spike, doc 19 §3.2): queries each of @@VARS@@ (a
// PowerShell array literal) for unset/set + value, base64-encoded via
// [Convert]::ToBase64String so the value can never contain OSC framing
// bytes. $LASTEXITCODE is saved into $__momo_pc before the loop and
// restored afterward, so the probe itself never disturbs the caller's exit
// code. Ends in "\r", not "\r\n", for the same reason as
// powershellHookTemplate -- CR alone submits via PSReadLine's Enter
// binding; a trailing LF after it would land on the next prompt and get
// inserted as a literal newline, leaving that prompt stuck in a spurious
// multi-line continuation.
const powershellProbeTemplate = `$__momo_pc = $LASTEXITCODE; $__momo_out = ""; foreach ($__momo_v in @@VARS@@) { if (Test-Path "variable:$__momo_v") { $__momo_val = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((Get-Variable -Name $__momo_v -ValueOnly).ToString())); $__momo_out += "$__momo_v=1:$__momo_val;" } else { $__momo_out += "$__momo_v=0:;" } }; Write-Host -NoNewline ([char]27 + "]1337;momo;probe;@@NONCE@@;" + $__momo_out + [char]7); $global:LASTEXITCODE = $__momo_pc` + "\r"

func powershellProbeScript(nonce string, vars []string) []byte {
	quoted := make([]string, len(vars))
	for i, v := range vars {
		quoted[i] = `"` + v + `"`
	}
	s := strings.ReplaceAll(powershellProbeTemplate, "@@NONCE@@", nonce)
	s = strings.ReplaceAll(s, "@@VARS@@", "@("+strings.Join(quoted, ",")+")")
	return []byte(s)
}

// powershellSpawnHookTemplate installs the same OSC133 hooks as
// powershellHookTemplate, but is passed via -EncodedCommand at process
// launch instead of typed into the running shell -- nothing is echoed, so
// there's nothing to suppress and no ConPTY/frontend desync is possible
// (see powershellHookTemplate's doc comment above). Consequently this has
// no momostart_ sentinel and no trailing CR: there's no typed-echo to find
// the start of, and no PSReadLine submission to trigger.
//
// $function:prompt captures the profile's prompt function (if any) into
// $global:__momo_p0 before overriding it, and the override chains to it --
// preserving the user's real prompt (path, git status, etc.) instead of
// hard-coding "PS> ". The guard `if (-not $global:__momo_p0)` makes this
// idempotent against an accidental double-install. Inside the override,
// $__momo_ec is computed from $?/$LASTEXITCODE *before* invoking the
// chained prompt -- the chained prompt may itself run native commands
// (e.g. a git-status prompt) that would otherwise clobber the exit code
// this hook is reporting.
const powershellSpawnHookTemplate = `if (-not $global:__momo_p0) { $global:__momo_p0 = $function:prompt }; function global:prompt { $__momo_ok = $?; $__momo_ec = if ($__momo_ok) { 0 } elseif ($LASTEXITCODE) { $LASTEXITCODE } else { 1 }; $__momo_txt = if ($global:__momo_p0) { & $global:__momo_p0 } else { "PS> " }; Write-Host -NoNewline ([char]27 + "]133;D;" + $__momo_ec + [char]7 + [char]27 + "]133;A" + [char]7); return $__momo_txt }; try { Set-PSReadLineKeyHandler -Key Enter -ScriptBlock { Write-Host -NoNewline ([char]27 + "]133;C" + [char]7); [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine() } } catch { }; try { Set-PSReadLineOption -AddToHistoryHandler { param($line) -not $line.StartsWith('$__momo_pc') } } catch { }; Write-Host -NoNewline ([char]27 + "]1337;momo;hookinstalled;@@NONCE@@" + [char]7)`

// powershellSpawnArgs returns the powershell.exe/pwsh arguments that launch
// with the spawn hook pre-installed. -NoLogo suppresses the copyright
// banner (deterministic across PS versions); -NoExit keeps the interactive
// prompt after -EncodedCommand's script body finishes; -EncodedCommand
// takes base64(UTF-16LE) and bypasses both quoting/escaping concerns and
// ExecutionPolicy (unlike -File or -Command).
func powershellSpawnArgs(nonce string) []string {
	s := strings.ReplaceAll(powershellSpawnHookTemplate, "@@NONCE@@", nonce)
	return []string{"-NoLogo", "-NoExit", "-EncodedCommand", encodePowerShellCommand(s)}
}

// encodePowerShellCommand base64-encodes script as raw UTF-16LE (no BOM),
// the format -EncodedCommand requires.
func encodePowerShellCommand(script string) string {
	units := utf16.Encode([]rune(script))
	b := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(b[i*2:], u)
	}
	return base64.StdEncoding.EncodeToString(b)
}
