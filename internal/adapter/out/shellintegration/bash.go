package shellintegration

import "strings"

// bashHookTemplate installs OSC133 prompt/command/exit hooks purely at
// runtime (no rc/profile edits) -- verified against real bash (Git-Bash
// MSYS + WSL Ubuntu) in the S4 spike (doc 19 §3.1). The leading
// "momostart_<nonce>" sentinel lets the middleware distinguish this
// injection's own echo+execution noise from real shell output it must not
// suppress (see shellintegration.Service's bootstrap phase).
//
// $? is captured into __momo_ec immediately on each PROMPT_COMMAND firing
// and restored via the printf argument (not re-evaluated), so emitting the
// OSC133 markers never disturbs the exit code the next command would see.
// HISTCONTROL=ignorespace plus this line's own self-deletion
// (`history -d $(history 1)`) keeps the injection out of the user's shell
// history.
const bashHookTemplate = `printf 'momostart_@@NONCE@@\n'; HISTCONTROL=ignorespace${HISTCONTROL:+:$HISTCONTROL}; PROMPT_COMMAND='__momo_ec=$?; printf "\033]133;D;%s\007\033]133;A\007" "$__momo_ec"'; PS0='\033]133;C\007'; history -d $(history 1) 2>/dev/null; printf "\033]1337;momo;hookinstalled;@@NONCE@@\007"
`

func bashHookScript(nonce string) []byte {
	return []byte(strings.ReplaceAll(bashHookTemplate, "@@NONCE@@", nonce))
}
