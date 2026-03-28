package ast

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// BashDecomposer maps tree-sitter Bash node types to TW AST types.
type BashDecomposer struct {
	lang *gotreesitter.Language
}

// NewBashDecomposer creates a new BashDecomposer.
func NewBashDecomposer() *BashDecomposer {
	return &BashDecomposer{lang: grammars.BashLanguage()}
}

func (d *BashDecomposer) Decompose(root *gotreesitter.Node, input []byte) *CompoundCommand {
	cc := &CompoundCommand{}
	d.walkNode(root, input, cc, nil)
	return cc
}

func (d *BashDecomposer) nodeType(n *gotreesitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Type(d.lang)
}

// walkNode recursively processes a node and appends results to cc.
// If inner is non-nil, SimpleCommands are collected as inner commands
// (from command substitutions) rather than added as top-level statements.
func (d *BashDecomposer) walkNode(node *gotreesitter.Node, input []byte, cc *CompoundCommand, inner *[]SimpleCommand) {
	if node == nil {
		return
	}

	nt := d.nodeType(node)
	switch nt {
	case "program":
		d.walkNamedChildren(node, input, cc, inner)

	case "command":
		result := d.decomposeCommand(node, input)
		d.addResult(result, cc, inner)

	case "declaration_command":
		result := d.decomposeDeclaration(node, input)
		d.addResult(result, cc, inner)

	case "pipeline":
		if inner != nil {
			// Inside a substitution: flatten pipeline commands as inner.
			for i := 0; i < node.NamedChildCount(); i++ {
				d.walkNode(node.NamedChild(i), input, cc, inner)
			}
		} else {
			stmt := d.decomposePipeline(node, input)
			cc.Statements = append(cc.Statements, stmt)
		}

	case "list":
		d.decomposeList(node, input, cc, inner)

	case "subshell", "compound_statement", "do_group":
		d.walkNamedChildren(node, input, cc, inner)

	case "if_statement", "elif_clause", "else_clause":
		d.walkNamedChildren(node, input, cc, inner)

	case "while_statement", "for_statement", "c_style_for_statement":
		d.walkNamedChildren(node, input, cc, inner)

	case "case_statement", "case_item":
		d.walkNamedChildren(node, input, cc, inner)

	case "negated_command":
		d.walkNamedChildren(node, input, cc, inner)

	case "redirected_statement":
		d.decomposeRedirectedStatement(node, input, cc, inner)

	case "function_definition":
		// Skip — function definitions are not executed at definition time.

	case "command_substitution", "process_substitution":
		// Extract inner commands from substitutions.
		if inner != nil {
			d.walkNamedChildren(node, input, cc, inner)
		} else {
			d.walkNamedChildren(node, input, cc, nil)
		}

	default:
		// For unrecognized named nodes, recurse into children.
		if node.NamedChildCount() > 0 {
			d.walkNamedChildren(node, input, cc, inner)
		}
	}
}

func (d *BashDecomposer) walkNamedChildren(node *gotreesitter.Node, input []byte, cc *CompoundCommand, inner *[]SimpleCommand) {
	for i := 0; i < node.NamedChildCount(); i++ {
		d.walkNode(node.NamedChild(i), input, cc, inner)
	}
}

// addResult appends a decomposed command to either the compound command or inner list.
func (d *BashDecomposer) addResult(result commandResult, cc *CompoundCommand, inner *[]SimpleCommand) {
	if inner != nil {
		*inner = append(*inner, result.cmd)
		*inner = append(*inner, result.inner...)
	} else {
		stage := PipelineStage{
			Command: result.cmd,
			Inner:   result.inner,
		}
		cc.Statements = append(cc.Statements, Statement{
			Stages: []PipelineStage{stage},
		})
	}
}

type commandResult struct {
	cmd   SimpleCommand
	inner []SimpleCommand
}

func (d *BashDecomposer) decomposeCommand(node *gotreesitter.Node, input []byte) commandResult {
	var result commandResult
	result.cmd.Raw = buildRaw(node, input)

	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if !child.IsNamed() {
			continue
		}
		ct := d.nodeType(child)

		switch ct {
		case "command_name":
			name := d.resolveText(child, input)
			result.cmd.Name = normalizeCommandName(name)

		case "file_redirect":
			d.handleRedirect(child, input, &result.cmd)

		case "heredoc_redirect", "herestring_redirect":
			// Drop — not security-relevant for command evaluation.

		case "variable_assignment":
			// Skip env-var prefix assignments (VAR=value cmd ...).

		case "command_substitution", "process_substitution":
			// Direct substitution as an argument.
			innerCC := &CompoundCommand{}
			d.walkNamedChildren(child, input, innerCC, nil)
			result.inner = append(result.inner, innerCC.AllCommands()...)
			result.cmd.Args = append(result.cmd.Args, nodeText(child, input))

		default:
			text := d.resolveText(child, input)
			d.collectInnerCommands(child, input, &result.inner)
			if text != "" {
				if isFlag(text) {
					result.cmd.Flags = append(result.cmd.Flags, text)
				} else {
					result.cmd.Args = append(result.cmd.Args, text)
				}
			}
		}
	}

	return result
}

func (d *BashDecomposer) decomposeDeclaration(node *gotreesitter.Node, input []byte) commandResult {
	var result commandResult
	result.cmd.Raw = buildRaw(node, input)

	// The first child token is the declaration keyword (export, local, etc.).
	// It may be anonymous or named depending on grammar version.
	foundName := false
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)

		if !foundName {
			text := nodeText(child, input)
			if text != "" {
				result.cmd.Name = text
				foundName = true
			}
			continue
		}

		if !child.IsNamed() {
			continue
		}

		ct := d.nodeType(child)
		switch ct {
		case "variable_assignment":
			result.cmd.Args = append(result.cmd.Args, nodeText(child, input))
		default:
			text := d.resolveText(child, input)
			if text != "" {
				if isFlag(text) {
					result.cmd.Flags = append(result.cmd.Flags, text)
				} else {
					result.cmd.Args = append(result.cmd.Args, text)
				}
			}
		}
	}

	return result
}

func (d *BashDecomposer) decomposePipeline(node *gotreesitter.Node, input []byte) Statement {
	var stmt Statement
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		ct := d.nodeType(child)

		switch ct {
		case "command":
			result := d.decomposeCommand(child, input)
			stmt.Stages = append(stmt.Stages, PipelineStage{
				Command: result.cmd,
				Inner:   result.inner,
			})
		case "declaration_command":
			result := d.decomposeDeclaration(child, input)
			stmt.Stages = append(stmt.Stages, PipelineStage{
				Command: result.cmd,
				Inner:   result.inner,
			})
		default:
			// Recurse into other constructs (redirected_statement, etc.)
			innerCC := &CompoundCommand{}
			d.walkNode(child, input, innerCC, nil)
			for _, s := range innerCC.Statements {
				stmt.Stages = append(stmt.Stages, s.Stages...)
			}
		}
	}
	return stmt
}

func (d *BashDecomposer) decomposeList(node *gotreesitter.Node, input []byte, cc *CompoundCommand, inner *[]SimpleCommand) {
	var pendingOp string

	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)

		if !child.IsNamed() {
			text := nodeText(child, input)
			if text == "&&" || text == "||" {
				pendingOp = text
			}
			continue
		}

		// Process named child as a command/pipeline/construct.
		beforeLen := len(cc.Statements)
		d.walkNode(child, input, cc, inner)

		// Set operator on the statement BEFORE this one, but only if the child
		// actually added at least one new statement. If it added nothing, leave
		// pendingOp set so it can be applied to the next child that does add
		// statements — attaching it now would wrongly annotate a prior statement.
		if inner == nil && pendingOp != "" && len(cc.Statements) > beforeLen {
			cc.Statements[beforeLen-1].Operator = pendingOp
			pendingOp = ""
		}
	}
}

func (d *BashDecomposer) decomposeRedirectedStatement(node *gotreesitter.Node, input []byte, cc *CompoundCommand, inner *[]SimpleCommand) {
	// A redirected_statement wraps a command/pipeline with redirections.
	// Process the body; keep output redirect targets separate from Args so
	// write-sensitive matchers can inspect them without changing arg semantics.
	var stdinRedirects []redirectInfo
	var outputRedirects []string
	var strayArgs []string

	// Record how many statements exist before processing the body so we can
	// detect whether the body actually added any new statements.
	beforeLen := len(cc.Statements)

	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		ct := d.nodeType(child)

		switch ct {
		case "file_redirect":
			ri, ok := d.parseRedirect(child, input)
			if !ok {
				continue
			}
			if ri.isStdin {
				stdinRedirects = append(stdinRedirects, ri)
			} else if ri.target != "" {
				outputRedirects = append(outputRedirects, ri.target)
			}
			// Collect stray args that tree-sitter placed inside the redirect node.
			strayArgs = append(strayArgs, ri.strayArgs...)
		case "heredoc_redirect", "herestring_redirect":
			// Extract the content and try to parse it as shell commands.
			content := d.resolveText(child, input)
			if content != "" {
				ccInner := ParseShell([]byte(content), ShellBash)
				if ccInner != nil {
					// We'll attach these inner commands to all stages of the statement
					// (simpler for safety matching than trying to attribute to one stage).
					for _, cmd := range ccInner.AllCommands() {
						strayArgs = append(strayArgs, "INNER:"+cmd.Raw)
					}
				}
			}
		default:
			// This is the body (command, pipeline, etc.).
			d.walkNode(child, input, cc, inner)
		}
	}

	// Attach stdin redirects and stray args only if the body added at least one
	// new statement. If no new statements were added (e.g. the body was a
	// function definition or other no-op node), skip attaching to avoid
	// wrongly annotating a pre-existing statement.
	if inner == nil && len(cc.Statements) > beforeLen {
		lastStmt := &cc.Statements[len(cc.Statements)-1]
		if len(lastStmt.Stages) > 0 {
			// Stdin redirects (<) apply to the FIRST stage of a pipeline.
			firstStage := &lastStmt.Stages[0]
			for _, ri := range stdinRedirects {
				firstStage.Command.Flags = append(firstStage.Command.Flags, "<")
				firstStage.Command.Args = append(firstStage.Command.Args, ri.target)
			}

			// Stray args (words tree-sitter couldn't place) are usually
			// trailing, so keep them on the LAST stage and attach output
			// redirect targets separately for write-sensitive matching.
			lastStage := &lastStmt.Stages[len(lastStmt.Stages)-1]
			lastStage.Command.OutputRedirects = append(lastStage.Command.OutputRedirects, outputRedirects...)
			for _, arg := range strayArgs {
				if isFlag(arg) {
					lastStage.Command.Flags = append(lastStage.Command.Flags, arg)
				} else {
					lastStage.Command.Args = append(lastStage.Command.Args, arg)
				}
			}
		}
	}
}

type redirectInfo struct {
	isStdin   bool
	target    string
	strayArgs []string // extra word tokens tree-sitter placed inside the redirect node
}

func (d *BashDecomposer) parseRedirect(node *gotreesitter.Node, input []byte) (redirectInfo, bool) {
	var ri redirectInfo
	foundTarget := false
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if !child.IsNamed() {
			text := nodeText(child, input)
			if text == "<" {
				ri.isStdin = true
			}
		} else {
			text := d.resolveText(child, input)
			if !foundTarget {
				ri.target = text
				foundTarget = true
			} else if text != "" {
				ri.strayArgs = append(ri.strayArgs, text)
			}
		}
	}
	return ri, ri.target != ""
}

func (d *BashDecomposer) handleRedirect(node *gotreesitter.Node, input []byte, cmd *SimpleCommand) {
	ri, ok := d.parseRedirect(node, input)
	if !ok {
		return
	}
	if ri.isStdin {
		cmd.Flags = append(cmd.Flags, "<")
		cmd.Args = append(cmd.Args, ri.target)
	} else {
		cmd.OutputRedirects = append(cmd.OutputRedirects, ri.target)
	}
	// Recover any stray arguments that tree-sitter placed inside the
	// file_redirect node. For example,
	// "git >/dev/null reset --hard" produces a file_redirect node
	// containing [">", "/dev/null", "reset", "--hard"]. The first
	// named child is the redirect target; the rest are command args.
	for _, arg := range ri.strayArgs {
		if isFlag(arg) {
			cmd.Flags = append(cmd.Flags, arg)
		} else {
			cmd.Args = append(cmd.Args, arg)
		}
	}
}

// resolveText extracts the effective (unquoted) text of a node.
func (d *BashDecomposer) resolveText(node *gotreesitter.Node, input []byte) string {
	if node == nil {
		return ""
	}
	nt := d.nodeType(node)

	switch nt {
	case "word", "number":
		return nodeText(node, input)

	case "command_name":
		if node.NamedChildCount() > 0 {
			return d.resolveText(node.NamedChild(0), input)
		}
		return nodeText(node, input)

	case "raw_string":
		return trimQuotes(nodeText(node, input), '\'')

	case "string", "translated_string":
		return trimQuotes(nodeText(node, input), '"')

	case "ansi_c_string":
		text := nodeText(node, input)
		if strings.HasPrefix(text, "$'") && strings.HasSuffix(text, "'") {
			return text[2 : len(text)-1]
		}
		return text

	case "simple_expansion":
		return nodeText(node, input)

	case "expansion":
		return nodeText(node, input)

	case "concatenation":
		var sb strings.Builder
		for i := 0; i < node.NamedChildCount(); i++ {
			sb.WriteString(d.resolveText(node.NamedChild(i), input))
		}
		return sb.String()

	case "command_substitution":
		return nodeText(node, input)

	default:
		return nodeText(node, input)
	}
}

// collectInnerCommands recursively finds command/process substitutions
// within a node and extracts their commands.
func (d *BashDecomposer) collectInnerCommands(node *gotreesitter.Node, input []byte, inner *[]SimpleCommand) {
	if node == nil || inner == nil {
		return
	}
	nt := d.nodeType(node)
	if nt == "command_substitution" || nt == "process_substitution" {
		innerCC := &CompoundCommand{}
		d.walkNamedChildren(node, input, innerCC, nil)
		*inner = append(*inner, innerCC.AllCommands()...)
		return
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		d.collectInnerCommands(node.NamedChild(i), input, inner)
	}
}
