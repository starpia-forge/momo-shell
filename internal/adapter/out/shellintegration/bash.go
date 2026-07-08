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

// bashProbeTemplate queries each of @@VARS@@ (a space-separated shell word
// list) for unset/set + value, base64-encoding the value so it can never
// contain OSC framing bytes (ESC, ], ;, BEL) -- see doc 19 §3.3/§4: this
// structurally eliminates sentinel-collision risk rather than escaping it.
// $? is saved into __momo_pc before the loop and restored via
// `( exit $__momo_pc )` in a subshell, so the probe itself never disturbs
// the caller's exit code.
const bashProbeTemplate = `__momo_pc=$?; __momo_out=""; for __momo_v in @@VARS@@; do if [ -n "${!__momo_v+x}" ]; then __momo_val=$(printf '%s' "${!__momo_v}" | base64 | tr -d '\n'); __momo_out="${__momo_out}${__momo_v}=1:${__momo_val};"; else __momo_out="${__momo_out}${__momo_v}=0:;"; fi; done; printf "\033]1337;momo;probe;@@NONCE@@;%s\007" "$__momo_out"; ( exit $__momo_pc )
`

func bashProbeScript(nonce string, vars []string) []byte {
	s := strings.ReplaceAll(bashProbeTemplate, "@@NONCE@@", nonce)
	s = strings.ReplaceAll(s, "@@VARS@@", strings.Join(vars, " "))
	return []byte(s)
}
