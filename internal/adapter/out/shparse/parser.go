// Package shparse implements out.ShellParser by wrapping mvdan.cc/sh/v3's
// syntax package (an AST parser, not a regex -- design doc 17 §6.1 step 1
// forbids regex for command safety analysis). mvdan.cc/sh is the only
// place in this codebase that imports the shell-AST library; every other
// package only sees the plain-data out.CommandAnalysis this adapter
// produces.
package shparse

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"momo-shell/internal/core/port/out"
)

type Parser struct{}

func New() *Parser { return &Parser{} }

var _ out.ShellParser = (*Parser)(nil)

// destructiveVerbs are commands whose effect is inherently destructive
// regardless of arguments (design doc 17 §6.1 step 3's risk matrix).
// chmod/chown are destructive only with a recursive flag -- see
// isRecursive.
var destructiveVerbs = map[string]bool{
	"rm": true, "dd": true, "mkfs": true,
	"chmod": true, "chown": true,
}

// shellIntegrityVerbs mutate shell state for the rest of the session (doc
// 17 §6.1 step 5) -- an approval category independent of the risk matrix.
// "export"/"declare"/"local"/"readonly"/"typeset"/"nameref" are usually
// parsed as *syntax.DeclClause rather than a plain call (see visitor.visit)
// and are handled there; they're also listed here as a safety net for any
// dialect/context where the parser instead emits a plain CallExpr.
var shellIntegrityVerbs = map[string]bool{
	"set": true, "trap": true, "alias": true, "unalias": true,
	"export": true, "declare": true, "local": true, "readonly": true, "typeset": true,
}

// outputRedirectOps are operators that write TO their target -- the ones
// relevant to "redirect targeting a critical path" (doc 17 §6.1 step 3's
// ">device"). Input redirects (<, <<, <<<) and fd-duplication (>&, <&)
// don't write into the filesystem target and are excluded.
var outputRedirectOps = map[syntax.RedirOperator]bool{
	syntax.RdrOut: true, syntax.AppOut: true,
	syntax.RdrClob: true, syntax.AppClob: true,
	syntax.RdrAll: true, syntax.RdrAllClob: true,
	syntax.AppAll: true, syntax.AppAllClob: true,
}

func (p *Parser) Analyze(command, dialect string) (out.CommandAnalysis, error) {
	variant, ok := langVariant(dialect)
	if !ok {
		return out.CommandAnalysis{}, fmt.Errorf("shparse: unsupported dialect %q", dialect)
	}

	file, err := syntax.NewParser(syntax.Variant(variant)).Parse(strings.NewReader(command), "")
	if err != nil {
		return out.CommandAnalysis{}, fmt.Errorf("shparse: parse: %w", err)
	}

	v := &visitor{varSeen: make(map[string]bool), integritySeen: make(map[string]bool)}
	syntax.Walk(file, v.visit)
	return v.result(), nil
}

// langVariant maps this project's dialect strings (matching
// out.ShellDialect's values plus "sh"/"posix") to a supported
// syntax.LangVariant. zsh is deliberately excluded -- mvdan.cc/sh's zsh
// support is upstream-documented as experimental/incomplete, and this
// project's shell-integration scope (S4/F3) is bash+PowerShell only; an
// unrecognized dialect here safely degrades to "can't analyze" rather than
// risking an inaccurate zsh parse.
func langVariant(dialect string) (syntax.LangVariant, bool) {
	switch dialect {
	case "bash":
		return syntax.LangBash, true
	case "sh", "posix":
		return syntax.LangPOSIX, true
	default:
		return 0, false
	}
}

type visitor struct {
	commands      []out.SimpleCommand
	varOrder      []string
	varSeen       map[string]bool
	uncertain     bool
	integrityOps  []string
	integritySeen map[string]bool
}

func (v *visitor) result() out.CommandAnalysis {
	return out.CommandAnalysis{
		Commands:       v.commands,
		ReferencedVars: v.varOrder,
		Uncertain:      v.uncertain,
		IntegrityOps:   v.integrityOps,
	}
}

func (v *visitor) addVar(name string) {
	if name == "" || v.varSeen[name] {
		return
	}
	v.varSeen[name] = true
	v.varOrder = append(v.varOrder, name)
}

func (v *visitor) addIntegrityOp(op string) {
	if v.integritySeen[op] {
		return
	}
	v.integritySeen[op] = true
	v.integrityOps = append(v.integrityOps, op)
}

func (v *visitor) visit(node syntax.Node) bool {
	switch n := node.(type) {
	case *syntax.Stmt:
		if call, ok := n.Cmd.(*syntax.CallExpr); ok {
			v.visitCall(n, call)
		}
	case *syntax.ParamExp:
		if n.Param != nil {
			v.addVar(n.Param.Value)
		}
	case *syntax.CmdSubst:
		// $(...) and `...` (Backquotes) both land here -- doc 17 §6.1
		// step 4: command substitution is always uncertain.
		v.uncertain = true
	case *syntax.FuncDecl:
		v.addIntegrityOp("function")
	case *syntax.DeclClause:
		if n.Variant != nil {
			v.addIntegrityOp(n.Variant.Value)
		}
	}
	return true
}

func (v *visitor) visitCall(stmt *syntax.Stmt, call *syntax.CallExpr) {
	cmd := out.SimpleCommand{}

	if len(call.Args) > 0 {
		verbWord := call.Args[0]
		cmd.Verb = verbWord.Lit()
		if cmd.Verb == "" && wordHasVar(verbWord) {
			// A dynamic verb ("$CMD args") means the command that will
			// actually run isn't known statically -- doc 17 §6.1 step 4's
			// "미지 원격 함수" (unknown function) principle applied to the
			// cheaply-detectable case: the verb itself is a runtime value.
			v.uncertain = true
		}
		for _, w := range call.Args[1:] {
			cmd.Args = append(cmd.Args, wordToArg(w))
		}
		if shellIntegrityVerbs[cmd.Verb] {
			v.addIntegrityOp(cmd.Verb)
		}
		if cmd.Verb == "eval" {
			// doc 17 §6.1 step 4: eval's effect depends on its argument's
			// runtime value, which is never statically known.
			v.uncertain = true
		}
	}

	for _, r := range stmt.Redirs {
		if outputRedirectOps[r.Op] {
			cmd.Redirects = append(cmd.Redirects, wordToArg(r.Word))
		}
	}

	v.commands = append(v.commands, cmd)
}

func wordToArg(w *syntax.Word) out.Arg {
	return out.Arg{Value: w.Lit(), HasVar: wordHasVar(w)}
}

// wordHasVar reports whether w contains any variable or command
// substitution anywhere in its structure (including nested inside double
// quotes), e.g. "/home/$USER", "${X:-def}", "prefix$(cmd)suffix".
func wordHasVar(w *syntax.Word) bool {
	found := false
	syntax.Walk(w, func(n syntax.Node) bool {
		if found {
			return false
		}
		switch n.(type) {
		case *syntax.ParamExp, *syntax.CmdSubst:
			found = true
			return false
		}
		return true
	})
	return found
}
