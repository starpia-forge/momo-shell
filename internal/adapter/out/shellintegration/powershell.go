package shellintegration

import "strings"

// powershellHookTemplate mirrors bashHookTemplate for PowerShell 5.1+
// (verified against Windows PowerShell 5.1 in the S4 spike, doc 19 §3.2).
//
// This MUST end in "\r\n", not "\n" -- PSReadLine's Enter key binding
// fires on CR; a bare LF is inserted as a literal newline into the
// multi-line edit buffer instead of submitting the command (doc 19 §6).
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
const powershellHookTemplate = `Write-Host "momostart_@@NONCE@@"; function global:prompt { $__momo_ok = $?; $__momo_ec = if ($__momo_ok) { 0 } elseif ($LASTEXITCODE) { $LASTEXITCODE } else { 1 }; Write-Host -NoNewline ([char]27 + "]133;D;" + $__momo_ec + [char]7 + [char]27 + "]133;A" + [char]7); return "PS> " }; try { Set-PSReadLineKeyHandler -Key Enter -ScriptBlock { Write-Host -NoNewline ([char]27 + "]133;C" + [char]7); [Microsoft.PowerShell.PSConsoleReadLine]::AcceptLine() } } catch { }; try { Set-PSReadLineOption -AddToHistoryHandler { param($line) -not $line.StartsWith('$__momo_pc') } } catch { }; Write-Host -NoNewline ([char]27 + "]1337;momo;hookinstalled;@@NONCE@@" + [char]7)` + "\r\n"

func powershellHookScript(nonce string) []byte {
	return []byte(strings.ReplaceAll(powershellHookTemplate, "@@NONCE@@", nonce))
}
