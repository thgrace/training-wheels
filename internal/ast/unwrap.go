package ast

import "strings"

const maxUnwrapIterations = 32

// Unwrap processes a CompoundCommand, stripping wrapper commands (sudo, env,
// command) and extracting inner commands from shell -c invocations.
func Unwrap(cc *CompoundCommand, shell ShellType) *CompoundCommand {
	if cc == nil {
		return nil
	}
	result := &CompoundCommand{}
	for _, stmt := range cc.Statements {
		newStmt := Statement{Operator: stmt.Operator}
		for _, stage := range stmt.Stages {
			main, extra := unwrapSimpleCommand(stage.Command, shell)
			newStage := PipelineStage{
				Command: main,
				Inner:   make([]SimpleCommand, 0, len(stage.Inner)+len(extra)),
			}
			newStage.Inner = append(newStage.Inner, stage.Inner...)
			newStage.Inner = append(newStage.Inner, extra...)
			// Recursively unwrap commands already placed in Inner (e.g., from
			// command substitutions parsed during AST construction).
			var enrichedInner []SimpleCommand
			for _, ic := range newStage.Inner {
				icMain, icInner := unwrapSimpleCommand(ic, shell)
				enrichedInner = append(enrichedInner, icMain)
				enrichedInner = append(enrichedInner, icInner...)
			}
			newStage.Inner = enrichedInner
			newStmt.Stages = append(newStmt.Stages, newStage)
		}
		result.Statements = append(result.Statements, newStmt)
	}
	return result
}

// unwrapSimpleCommand iteratively strips wrapper commands and extracts inner
// commands from shell -c invocations.
func unwrapSimpleCommand(cmd SimpleCommand, shell ShellType) (result SimpleCommand, inner []SimpleCommand) {
	var allInner []SimpleCommand

	for i := 0; i < maxUnwrapIterations; i++ {
		// Strip leading backslash (alias bypass: \rm → rm).
		cmd.Name = strings.TrimPrefix(cmd.Name, "\\")

		switch cmd.Name {
		case "sudo":
			next, ok := stripSudo(cmd.Raw)
			if !ok {
				return cmd, allInner
			}
			cc := ParseShell([]byte(next), shell)
			if cc == nil || len(cc.AllCommands()) == 0 {
				return cmd, allInner
			}
			cmd = cc.AllCommands()[0]
			allInner = append(allInner, cc.AllCommands()[1:]...)
			continue

		case "env":
			next, ok := stripEnv(cmd.Raw)
			if !ok {
				return cmd, allInner
			}
			cc := ParseShell([]byte(next), shell)
			if cc == nil || len(cc.AllCommands()) == 0 {
				return cmd, allInner
			}
			cmd = cc.AllCommands()[0]
			allInner = append(allInner, cc.AllCommands()[1:]...)
			continue

		case "xargs":
			next, ok := stripXargs(cmd.Raw)
			if !ok {
				return cmd, allInner
			}
			cc := ParseShell([]byte(next), shell)
			if cc == nil || len(cc.AllCommands()) == 0 {
				return cmd, allInner
			}
			cmd = cc.AllCommands()[0]
			allInner = append(allInner, cc.AllCommands()[1:]...)
			continue

		case "command":
			next, ok := stripCommandBuiltin(cmd.Raw)
			if !ok {
				return cmd, allInner
			}
			cc := ParseShell([]byte(next), shell)
			if cc == nil || len(cc.AllCommands()) == 0 {
				return cmd, allInner
			}
			cmd = cc.AllCommands()[0]
			allInner = append(allInner, cc.AllCommands()[1:]...)
			continue

		// Transparent wrappers: strip the prefix, re-parse remainder.
		case "nice", "time", "nohup", "watch", "timeout", "strace", "ltrace":
			next, ok := stripTransparentWrapper(cmd.Raw)
			if !ok {
				return cmd, allInner
			}
			cc := ParseShell([]byte(next), shell)
			if cc == nil || len(cc.AllCommands()) == 0 {
				return cmd, allInner
			}
			cmd = cc.AllCommands()[0]
			allInner = append(allInner, cc.AllCommands()[1:]...)
			continue

		case "bash", "sh", "zsh", "dash", "ksh":
			if inner := extractShellC(cmd, shell); len(inner) > 0 {
				allInner = append(allInner, inner...)
			}
			return cmd, allInner

		// PowerShell wrappers.
		case "powershell", "pwsh":
			if inner := extractPSCommand(cmd); len(inner) > 0 {
				allInner = append(allInner, inner...)
			}
			return cmd, allInner

		case "python", "python3", "python2":
			if inner := extractInterpreterInlineCode(cmd, shell, 'c'); len(inner) > 0 {
				allInner = append(allInner, inner...)
			}
			return cmd, allInner

		case "ruby", "perl", "node":
			if inner := extractInterpreterInlineCode(cmd, shell, 'e'); len(inner) > 0 {
				allInner = append(allInner, inner...)
			}
			return cmd, allInner

		case "invoke-expression":
			if inner := extractInvokeExpression(cmd); len(inner) > 0 {
				allInner = append(allInner, inner...)
			}
			return cmd, allInner

		default:
			return cmd, allInner
		}
	}
	return cmd, allInner
}

// ---------- wrapper strippers (operate on raw command text) ----------

// sudoFlagsWithArg are sudo flags that consume the next token as their argument.
var sudoFlagsWithArg = map[string]bool{
	"-u": true, "--user": true,
	"-g": true, "--group": true,
	"-p": true, "--prompt": true,
	"-C": true, "--close-from": true,
	"-D": true, "--chdir": true,
	"-R": true, "--role": true,
	"-T": true, "--type": true,
}

func stripSudo(raw string) (string, bool) {
	tokens := shellSplit(raw)
	if len(tokens) == 0 {
		return "", false
	}
	name := normalizeCommandName(unquoteSimple(tokens[0]))
	if name != "sudo" {
		return "", false
	}
	i := 1
	for i < len(tokens) {
		tok := unquoteSimple(tokens[i])
		if tok == "--" {
			i++
			break
		}
		if !isFlag(tok) {
			break
		}
		switch {
		case sudoFlagsWithArg[tok]:
			i += 2
		case tok[0] == '-' && tok[1] != '-' && len(tok) > 2:
			// Combined short flags like -nu: check if the last char takes an argument.
			if sudoFlagsWithArg["-"+string(tok[len(tok)-1])] {
				i += 2 // skip bundle and its argument (e.g., -nu root)
			} else {
				i++ // no argument consumed (e.g., -nE)
			}
		default:
			// No-arg flag or unknown flag — both advance by 1.
			i++
		}
	}
	if i >= len(tokens) {
		return "", false
	}
	return strings.Join(tokens[i:], " "), true
}

func stripEnv(raw string) (string, bool) {
	tokens := shellSplit(raw)
	if len(tokens) == 0 {
		return "", false
	}
	name := normalizeCommandName(unquoteSimple(tokens[0]))
	if name != "env" {
		return "", false
	}
	i := 1
	for i < len(tokens) {
		tok := unquoteSimple(tokens[i])
		if tok == "--" {
			i++
			break
		}
		// env flags
		switch tok {
		case "-i", "-0", "--null", "--ignore-environment":
			i++
			continue
		case "-u", "--unset", "-C", "--chdir":
			i += 2
			continue
		}
		if strings.HasPrefix(tok, "-u=") || strings.HasPrefix(tok, "--unset=") ||
			strings.HasPrefix(tok, "-C=") || strings.HasPrefix(tok, "--chdir=") {
			i++
			continue
		}
		// VAR=value assignment
		if strings.Contains(tok, "=") && !isFlag(tok) {
			i++
			continue
		}
		break
	}
	if i >= len(tokens) {
		return "", false
	}
	return strings.Join(tokens[i:], " "), true
}

func stripCommandBuiltin(raw string) (string, bool) {
	tokens := shellSplit(raw)
	if len(tokens) <= 1 {
		return "", false
	}
	i := 1
	for i < len(tokens) {
		tok := unquoteSimple(tokens[i])
		if tok == "-v" || tok == "-V" || tok == "-p" {
			i++
			continue
		}
		break
	}
	if i >= len(tokens) {
		return "", false
	}
	return strings.Join(tokens[i:], " "), true
}

var xargsFlagsWithArg = map[string]bool{
	"-a": true, "--arg-file": true,
	"-d": true, "--delimiter": true,
	"-E": true, "--eof": true,
	"-e": true,
	"-I": true, "--replace": true,
	"-i": true,
	"-L": true, "--max-lines": true,
	"-l": true,
	"-n": true, "--max-args": true,
	"-P": true, "--max-procs": true,
	"-s": true, "--max-chars": true,
}

func stripXargs(raw string) (string, bool) {
	tokens := shellSplit(raw)
	if len(tokens) <= 1 {
		return "", false
	}
	name := normalizeCommandName(unquoteSimple(tokens[0]))
	if name != "xargs" {
		return "", false
	}

	i := 1
	for i < len(tokens) {
		tok := unquoteSimple(tokens[i])
		if tok == "--" {
			i++
			break
		}
		if !isFlag(tok) {
			break
		}
		if xargsFlagsWithArg[tok] {
			i += 2
			continue
		}
		i++
	}

	if i >= len(tokens) {
		return "", false
	}
	return strings.Join(tokens[i:], " "), true
}

// stripTransparentWrapper strips a transparent prefix command (nice, time, etc.)
// that simply passes through to the next command. These wrappers may have their
// own flags, and some (like timeout) take a positional argument before the command.
func stripTransparentWrapper(raw string) (string, bool) {
	tokens := shellSplit(raw)
	if len(tokens) <= 1 {
		return "", false
	}
	wrapperName := normalizeCommandName(unquoteSimple(tokens[0]))

	// Skip the wrapper name and any flags.
	i := 1
	sawDashDash := false
	for i < len(tokens) {
		tok := unquoteSimple(tokens[i])
		if tok == "--" {
			sawDashDash = true
			i++
			break
		}
		if !isFlag(tok) {
			break
		}
		if transparentWrapperFlagsWithArg[tok] {
			i += 2
		} else {
			i++
		}
	}

	// Some wrappers take a positional argument before the command.
	// timeout: timeout [flags] DURATION command...
	// If -- was seen, all positional arguments are already consumed — do not skip further.
	if !sawDashDash {
		if positionalCount, ok := transparentWrapperPositionalArgs[wrapperName]; ok {
			i += positionalCount
		}
	}

	if i >= len(tokens) {
		return "", false
	}
	return strings.Join(tokens[i:], " "), true
}

// transparentWrapperPositionalArgs lists how many positional (non-flag)
// arguments each wrapper consumes before the actual command.
var transparentWrapperPositionalArgs = map[string]int{
	"timeout": 1, // timeout DURATION command...
}

// transparentWrapperFlagsWithArg lists flags for transparent wrappers that
// consume the next token as their argument.
var transparentWrapperFlagsWithArg = map[string]bool{
	"-n": true, "--adjustment": true, // nice -n, watch -n (both take arg)
	"-k": true, "--kill-after": true, // timeout
	"-s": true, "--signal": true,     // timeout
	"-d": true, "--differences": true, // watch
	"-e": true, // strace/ltrace expression filter
	"-o": true, // strace/ltrace output file
	"-p": true, // strace pid to attach
	"-P": true, // strace path filter
}

// ---------- shell -c extraction ----------

// extractShellC extracts inner commands from shell -c invocations.
// For `bash -c "rm -rf /"`, it returns the parsed commands from the -c argument.
// Handles intervening flags: `bash -O extglob -c "rm -rf /"` correctly
// finds the argument after -c by scanning the raw command.
func extractShellC(cmd SimpleCommand, shell ShellType) []SimpleCommand {
	if !hasShellCFlag(cmd) {
		return nil
	}
	// Find the -c argument from the raw command tokens, not Args[0].
	// This handles cases like `bash -O extglob -c "cmd"` where -O consumes
	// the next token and the -c argument is further along.
	cmdStr := findCArgFromRaw(cmd.Raw)
	if cmdStr == "" {
		return nil
	}
	cc := ParseShell([]byte(cmdStr), shell)
	if cc == nil {
		return nil
	}
	cmds := cc.AllCommands()
	var result []SimpleCommand
	for _, c := range cmds {
		main, inner := unwrapSimpleCommand(c, shell)
		result = append(result, main)
		result = append(result, inner...)
	}
	return result
}

// findCArgFromRaw scans the raw command tokens to find the argument
// immediately after -c (or a combined flag containing c).
func findCArgFromRaw(raw string) string {
	tokens := shellSplit(raw)
	if len(tokens) < 3 { // need at least: shell -c "cmd"
		return ""
	}
	// Skip the shell name (token 0).
	for i := 1; i < len(tokens); i++ {
		tok := unquoteSimple(tokens[i])
		if tok == "-c" {
			// Next token is the command string.
			if i+1 < len(tokens) {
				return unquoteSimple(tokens[i+1])
			}
			return ""
		}
		// Combined short flags: -xc, -Oc — if last char is 'c', next token is the arg.
		if len(tok) > 2 && tok[0] == '-' && tok[1] != '-' && tok[len(tok)-1] == 'c' {
			if i+1 < len(tokens) {
				return unquoteSimple(tokens[i+1])
			}
			return ""
		}
	}
	return ""
}

func hasShellCFlag(cmd SimpleCommand) bool {
	for _, f := range cmd.Flags {
		if f == "-c" {
			return true
		}
		// Combined short flags: -xc, etc. — 'c' must be last (its argument follows as next token).
		if len(f) > 2 && f[0] == '-' && f[1] != '-' && f[len(f)-1] == 'c' {
			return true
		}
	}
	return false
}

// ---------- interpreter -c/-e extraction (python, ruby, perl, node) ----------

// extractInterpreterInlineCode extracts inner commands from interpreter inline code invocations.
// For `python3 -c "import os; os.system('rm -rf /')"`, it tries to parse the -c argument
// as a shell command (it may contain shell commands via os.system/exec). If parsing fails,
// the raw argument is kept on the interpreter command for arg_contains matching.
func extractInterpreterInlineCode(cmd SimpleCommand, shell ShellType, flagChar byte) []SimpleCommand {
	// Check that the interpreter has the appropriate flag.
	flagStr := "-" + string(flagChar)
	hasFlag := false
	for _, f := range cmd.Flags {
		if f == flagStr {
			hasFlag = true
			break
		}
		// Combined short flags: e.g., -Bc where last char is the flag char.
		if len(f) > 2 && f[0] == '-' && f[1] != '-' && f[len(f)-1] == flagChar {
			hasFlag = true
			break
		}
	}
	if !hasFlag {
		return nil
	}

	// Find the flag argument from the raw command tokens.
	cmdStr := findFlagArgFromRaw(cmd.Raw, flagChar)
	if cmdStr == "" {
		return nil
	}

	// Try parsing the argument as a bash command. For interpreter inline code
	// (e.g., Python's os.system("rm -rf /")), this may not parse as bash,
	// but if it does we can extract inner commands.
	var result []SimpleCommand
	cc := ParseShell([]byte(cmdStr), shell)
	if cc != nil {
		for _, c := range cc.AllCommands() {
			main, inner := unwrapSimpleCommand(c, shell)
			result = append(result, main)
			result = append(result, inner...)
		}
	}

	// Also extract quoted strings from the code and try to parse each as bash.
	// Handles patterns like os.system('rm -rf /'), exec("git reset --hard"),
	// subprocess.run("rm -rf /", shell=True), etc.
	for _, q := range extractQuotedStrings(cmdStr) {
		qcc := ParseShell([]byte(q), shell)
		if qcc == nil {
			continue
		}
		for _, c := range qcc.AllCommands() {
			main, inner := unwrapSimpleCommand(c, shell)
			result = append(result, main)
			result = append(result, inner...)
		}
	}
	return result
}

// extractQuotedStrings extracts the contents of single- and double-quoted
// strings from a code string. Used to find shell commands embedded in
// interpreter inline code (e.g., Python's os.system('rm -rf /')).
func extractQuotedStrings(s string) []string {
	var results []string
	i := 0
	for i < len(s) {
		if s[i] == '\'' || s[i] == '"' {
			quote := s[i]
			i++
			start := i
			for i < len(s) && s[i] != quote {
				if s[i] == '\\' && quote == '"' && i+1 < len(s) {
					i++ // skip escaped char in double quotes
				}
				i++
			}
			if i < len(s) {
				content := s[start:i]
				if len(strings.TrimSpace(content)) > 0 {
					results = append(results, content)
				}
				i++ // skip closing quote
			}
		} else {
			i++
		}
	}
	return results
}

// findFlagArgFromRaw scans the raw command tokens to find the argument
// immediately after the given short flag character (e.g., 'c' for -c, 'e' for -e).
func findFlagArgFromRaw(raw string, flagChar byte) string {
	tokens := shellSplit(raw)
	if len(tokens) < 3 { // need at least: interpreter -flag "code"
		return ""
	}
	flagStr := "-" + string(flagChar)
	// Skip the interpreter name (token 0).
	for i := 1; i < len(tokens); i++ {
		tok := unquoteSimple(tokens[i])
		if tok == flagStr {
			// Next token is the code string.
			if i+1 < len(tokens) {
				return unquoteSimple(tokens[i+1])
			}
			return ""
		}
		// Combined short flags: e.g., -Bc where last char is the flag char.
		if len(tok) > 2 && tok[0] == '-' && tok[1] != '-' && tok[len(tok)-1] == flagChar {
			if i+1 < len(tokens) {
				return unquoteSimple(tokens[i+1])
			}
			return ""
		}
	}
	return ""
}

// ---------- tokenizer ----------

// shellSplit splits a command string into tokens respecting shell quoting.
// It preserves quote characters in the output so tokens can be re-joined
// and re-parsed by tree-sitter.
func shellSplit(s string) []string {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			current.WriteByte(ch)
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			current.WriteByte(ch)
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			current.WriteByte(ch)
			continue
		}

		if (ch == ' ' || ch == '\t') && !inSingle && !inDouble {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// unquoteSimple strips a single layer of matching outer quotes.
func unquoteSimple(s string) string {
	if len(s) < 2 {
		return s
	}
	if (s[0] == '\'' && s[len(s)-1] == '\'') ||
		(s[0] == '"' && s[len(s)-1] == '"') {
		return s[1 : len(s)-1]
	}
	return s
}

// ---------- PowerShell wrappers ----------

// extractPSCommand extracts inner commands from powershell/pwsh -Command invocations.
// e.g., `powershell -Command "Remove-Item -Recurse /tmp"` → parse the -Command argument.
func extractPSCommand(cmd SimpleCommand) []SimpleCommand {
	hasCommand := false
	for _, f := range cmd.Flags {
		fl := strings.ToLower(f)
		if fl == "-command" || fl == "-c" ||
			(len(fl) >= 4 && strings.HasPrefix("-command", fl)) {
			hasCommand = true
			break
		}
	}
	if !hasCommand {
		return nil
	}
	// Find the -Command argument from the raw command tokens, not Args[0].
	// This handles cases like `pwsh -NoProfile -Command "cmd"` where
	// intervening flags shift Args indices.
	cmdStr := findPSCommandArgFromRaw(cmd.Raw)
	if cmdStr == "" {
		return nil
	}
	cc := ParseShell([]byte(cmdStr), ShellPowerShell)
	if cc == nil {
		return nil
	}
	cmds := cc.AllCommands()
	var result []SimpleCommand
	for _, c := range cmds {
		main, inner := unwrapSimpleCommand(c, ShellPowerShell)
		result = append(result, main)
		result = append(result, inner...)
	}
	return result
}

// findPSCommandArgFromRaw scans the raw command tokens to find the argument
// immediately after -Command or -c (case-insensitive).
func findPSCommandArgFromRaw(raw string) string {
	tokens := shellSplit(raw)
	if len(tokens) < 3 { // need at least: powershell -Command "cmd"
		return ""
	}
	// Skip the shell name (token 0).
	for i := 1; i < len(tokens); i++ {
		tok := unquoteSimple(tokens[i])
		tokLower := strings.ToLower(tok)
		if tokLower == "-command" || tokLower == "-c" ||
			(len(tokLower) >= 4 && strings.HasPrefix("-command", tokLower)) {
			// Next token is the command string.
			if i+1 < len(tokens) {
				return unquoteSimple(tokens[i+1])
			}
			return ""
		}
	}
	return ""
}

// extractInvokeExpression extracts inner commands from Invoke-Expression calls.
// e.g., `Invoke-Expression "rm -rf /"` → parse the first argument.
func extractInvokeExpression(cmd SimpleCommand) []SimpleCommand {
	if len(cmd.Args) == 0 {
		return nil
	}
	cmdStr := cmd.Args[0]
	// Invoke-Expression content could be PowerShell or POSIX.
	// Try PowerShell first; the structural matcher will evaluate either way.
	cc := ParseShell([]byte(cmdStr), ShellPowerShell)
	if cc == nil || len(cc.AllCommands()) == 0 {
		// Fall back to Bash parse for POSIX commands.
		cc = ParseShell([]byte(cmdStr), ShellBash)
	}
	if cc == nil {
		return nil
	}
	return cc.AllCommands()
}
