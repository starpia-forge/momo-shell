package out

// ShellParser is the driven port for static shell-command analysis.
// Implementations are pure (no I/O, no session access) -- the resolve
// service (internal/core/service/aicontrol/resolve) uses the returned
// plain data to classify risk; no shell-AST library type crosses this
// boundary (see internal/adapter/out/shparse, which wraps mvdan.cc/sh).
type ShellParser interface {
	// Analyze parses command under dialect and returns what resolve needs
	// to classify it. Returns an error for a parse failure or an
	// unrecognized dialect -- callers must treat that as "can't determine
	// safety statically" (degrade to requiring approval), never as "safe
	// to run".
	Analyze(command, dialect string) (CommandAnalysis, error)
}

// CommandAnalysis is the static-analysis result for one parsed command
// (which may itself be a pipeline/list of several simple commands, e.g.
// "cmd1 | cmd2 && cmd3").
type CommandAnalysis struct {
	// Commands is every simple command (verb + args + its own redirects)
	// found in the parse tree, in source order.
	Commands []SimpleCommand
	// ReferencedVars is the deduplicated set of variable names the command
	// expands (e.g. "X" for both "$X" and "${X:-default}") -- resolve
	// treats any reference here as a value it doesn't statically know,
	// forcing a safe degrade until VarQuery supplies a live value.
	ReferencedVars []string
	// Uncertain is true when the command's effect can't be statically
	// pinned down at all: command substitution ($(...) or `...`), or a
	// verb that isn't a fixed literal (e.g. "$CMD args", where the command
	// to run is itself a runtime value).
	Uncertain bool
	// IntegrityOps lists shell-state-mutating operations found (e.g.
	// "set", "trap", "alias", "export", "function") -- these change how
	// later commands in the same shell behave, and are an approval
	// category independent of the risk matrix.
	IntegrityOps []string
}

// SimpleCommand is one verb invocation (e.g. one stage of a pipeline).
type SimpleCommand struct {
	// Verb is the command name as a literal, or "" if it isn't one (e.g.
	// a dynamic "$CMD" invocation -- see CommandAnalysis.Uncertain).
	Verb      string
	Args      []Arg
	Redirects []Arg
}

// Arg is one word (a command argument or a redirect target).
type Arg struct {
	// Value is the word's literal text, or "" if the word isn't a pure
	// literal (HasVar is true in that case).
	Value string
	// HasVar is true if the word contains any variable or command
	// substitution -- its runtime value isn't statically known, including
	// the possibility it resolves to empty or unset.
	HasVar bool
}
