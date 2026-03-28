package ast

import "strings"

// EnrichCommands applies command-specific knowledge to extract subcommands
// from multi-level CLI tools (git, docker, kubectl, etc.).
// This runs after decomposition and unwrapping, before matching.
func EnrichCommands(cmds []SimpleCommand) {
	for i := range cmds {
		enrichCommand(&cmds[i])
	}
}

func enrichCommand(cmd *SimpleCommand) {
	switch cmd.Name {
	case "git":
		enrichSubcommand(cmd, gitGlobalFlagsWithArg)
	case "docker", "podman":
		enrichSubcommand(cmd, dockerGlobalFlagsWithArg)
	case "kubectl":
		enrichSubcommand(cmd, kubectlGlobalFlagsWithArg)
	case "terraform", "tofu":
		enrichSubcommand(cmd, terraformGlobalFlagsWithArg)
	case "helm":
		enrichSubcommand(cmd, helmGlobalFlagsWithArg)
	case "gh":
		enrichSubcommand(cmd, ghGlobalFlagsWithArg)
	case "restic":
		enrichSubcommand(cmd, resticGlobalFlagsWithArg)
	}
}

// enrichSubcommand walks Args, skips tokens consumed by known global flags,
// and extracts the first remaining token as the subcommand.
func enrichSubcommand(cmd *SimpleCommand, globalFlagsWithArg map[string]bool) {
	if len(cmd.Args) == 0 {
		return
	}

	// Build a set of global flags (with args) that appear in cmd.Flags.
	// For each one found, consume one token from Args.
	//
	// The decomposer puts flag-like tokens (starting with -) into Flags
	// and everything else into Args. A global flag like -C consumes the
	// NEXT token as its argument, but the decomposer doesn't know that,
	// so the argument lands in Args.
	//
	// Strategy: count how many global-flags-with-arg appear in Flags,
	// then skip that many leading Args (they're flag-arguments, not
	// positional args). The first non-skipped Arg is the subcommand.

	skipCount := 0
	for _, f := range cmd.Flags {
		// Handle --flag=value (already consumed, no arg in Args to skip).
		if strings.ContainsRune(f, '=') {
			continue
		}
		flagName := f
		// For combined short flags like -Cfoo, only check the first char.
		// But global flags are typically single-letter or long-form.
		if globalFlagsWithArg[flagName] {
			skipCount++
		}
	}

	if skipCount >= len(cmd.Args) {
		// All args consumed by global flags — no subcommand.
		return
	}

	// The subcommand is at Args[skipCount].
	cmd.Subcommand = cmd.Args[skipCount]

	// Remove the subcommand from Args.
	cmd.Args = append(cmd.Args[:skipCount], cmd.Args[skipCount+1:]...)
}

// ---------------------------------------------------------------------------
// Global flag tables — flags that consume the next token as their argument.
// Only flags that take a separate argument are listed. Flags using = syntax
// (--git-dir=/path) are handled automatically.
// ---------------------------------------------------------------------------

var gitGlobalFlagsWithArg = map[string]bool{
	"-C":             true,
	"-c":             true,
	"--git-dir":      true,
	"--work-tree":    true,
	"--namespace":    true,
	"--super-prefix": true,
	"--config-env":   true,
}

var dockerGlobalFlagsWithArg = map[string]bool{
	"-H":        true,
	"--host":    true,
	"--config":  true,
	"--context":  true,
	"-c":        true,
	"--log-level": true,
	"-l":        true,
}

var kubectlGlobalFlagsWithArg = map[string]bool{
	"-n":              true,
	"--namespace":     true,
	"--context":       true,
	"--cluster":       true,
	"--user":          true,
	"--kubeconfig":    true,
	"-s":              true,
	"--server":        true,
	"--token":         true,
	"--as":            true,
	"--as-group":      true,
	"--certificate-authority": true,
	"--client-certificate":    true,
	"--client-key":            true,
}

var terraformGlobalFlagsWithArg = map[string]bool{
	"-chdir": true,
}

var helmGlobalFlagsWithArg = map[string]bool{
	"-n":              true,
	"--namespace":     true,
	"--kube-context":  true,
	"--kubeconfig":    true,
	"--registry-config": true,
	"--repository-cache": true,
	"--repository-config": true,
}

var ghGlobalFlagsWithArg = map[string]bool{
	"-R":      true,
	"--repo":  true,
}

var resticGlobalFlagsWithArg = map[string]bool{
	"-r":                 true,
	"--repo":             true,
	"-o":                 true,
	"--option":           true,
	"--password-file":    true,
	"--password-command": true,
	"--cache-dir":        true,
	"--tls-client-cert":  true,
	"--key-hint":         true,
}
