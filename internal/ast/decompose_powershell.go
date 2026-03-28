package ast

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// PowerShellDecomposer maps tree-sitter PowerShell node types to TW AST types.
type PowerShellDecomposer struct {
	lang *gotreesitter.Language
}

// NewPowerShellDecomposer creates a new PowerShellDecomposer.
func NewPowerShellDecomposer() *PowerShellDecomposer {
	return &PowerShellDecomposer{lang: grammars.PowershellLanguage()}
}

func (d *PowerShellDecomposer) Decompose(root *gotreesitter.Node, input []byte) *CompoundCommand {
	cc := &CompoundCommand{}
	d.walkNode(root, input, cc)
	return cc
}

func (d *PowerShellDecomposer) nodeType(n *gotreesitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Type(d.lang)
}

func (d *PowerShellDecomposer) walkNode(node *gotreesitter.Node, input []byte, cc *CompoundCommand) {
	if node == nil {
		return
	}

	nt := d.nodeType(node)
	switch nt {
	case "program", "statement_list":
		d.walkNamedChildren(node, input, cc)

	case "pipeline":
		d.walkNamedChildren(node, input, cc)

	case "pipeline_chain":
		d.decomposePipelineChain(node, input, cc)

	case "command":
		result := d.decomposeCommand(node, input)
		cc.Statements = append(cc.Statements, Statement{
			Stages: []PipelineStage{{Command: result.cmd, Inner: result.inner}},
		})

	case "if_statement":
		d.walkNamedChildren(node, input, cc)

	case "foreach_statement", "for_statement", "while_statement", "do_while_statement":
		d.walkNamedChildren(node, input, cc)

	case "switch_statement":
		d.walkNamedChildren(node, input, cc)

	case "try_statement":
		d.walkNamedChildren(node, input, cc)

	case "statement_block":
		d.walkNamedChildren(node, input, cc)

	case "function_definition", "function_statement":
		// Skip — not executed at definition time.

	case "empty_statement":
		// Skip semicolons.

	case "assignment_expression":
		// Recurse into the RHS which may contain a pipeline/command.
		d.walkNamedChildren(node, input, cc)

	default:
		// For expression wrappers and unknown nodes, recurse into children.
		if node.NamedChildCount() > 0 {
			d.walkNamedChildren(node, input, cc)
		}
	}
}

func (d *PowerShellDecomposer) walkNamedChildren(node *gotreesitter.Node, input []byte, cc *CompoundCommand) {
	for i := 0; i < node.NamedChildCount(); i++ {
		d.walkNode(node.NamedChild(i), input, cc)
	}
}

func (d *PowerShellDecomposer) decomposePipelineChain(node *gotreesitter.Node, input []byte, cc *CompoundCommand) {
	// pipeline_chain contains command nodes separated by | (anonymous).
	var stages []PipelineStage

	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		ct := d.nodeType(child)

		switch ct {
		case "command":
			result := d.decomposeCommand(child, input)
			stages = append(stages, PipelineStage{
				Command: result.cmd,
				Inner:   result.inner,
			})
		default:
			// Expression nodes in pipeline (e.g., assignment_expression).
			// Walk for nested commands.
			innerCC := &CompoundCommand{}
			d.walkNode(child, input, innerCC)
			for _, s := range innerCC.Statements {
				stages = append(stages, s.Stages...)
			}
		}
	}

	if len(stages) > 0 {
		cc.Statements = append(cc.Statements, Statement{Stages: stages})
	}
}

func (d *PowerShellDecomposer) decomposeCommand(node *gotreesitter.Node, input []byte) commandResult {
	var result commandResult
	result.cmd.Raw = buildRaw(node, input)

	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		ct := d.nodeType(child)

		switch ct {
		case "command_name":
			name := nodeText(child, input)
			// PowerShell is case-insensitive: lowercase command names.
			result.cmd.Name = strings.ToLower(normalizeCommandName(name))

		case "command_elements":
			d.processCommandElements(child, input, &result)
		}
	}

	return result
}

func (d *PowerShellDecomposer) processCommandElements(node *gotreesitter.Node, input []byte, result *commandResult) {
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		ct := d.nodeType(child)

		switch ct {
		case "command_parameter":
			flag := nodeText(child, input)
			// Lowercase for case-insensitive matching.
			flag = strings.ToLower(flag)
			// Expand known abbreviations for security-critical cmdlets.
			flag = expandPSFlag(result.cmd.Name, flag)
			result.cmd.Flags = append(result.cmd.Flags, flag)

		case "command_argument_sep":
			// Skip whitespace separators.

		case "generic_token":
			text := nodeText(child, input)
			if isFlag(text) {
				flag := strings.ToLower(text)
				flag = expandPSFlag(result.cmd.Name, flag)
				result.cmd.Flags = append(result.cmd.Flags, flag)
			} else {
				result.cmd.Args = append(result.cmd.Args, text)
			}

		default:
			// Expression nodes: unwrap to get the text value.
			text := d.resolveExprText(child, input)
			if text != "" {
				result.cmd.Args = append(result.cmd.Args, text)
			}
			// Recursively search for subexpressions $(...) containing inner commands.
			d.collectSubexpressionCommands(child, input, &result.inner)
		}
	}
}

// collectSubexpressionCommands recursively searches for subexpression nodes $(...) within
// a node and extracts their inner commands, similar to how bash command_substitution works.
func (d *PowerShellDecomposer) collectSubexpressionCommands(node *gotreesitter.Node, input []byte, inner *[]SimpleCommand) {
	if node == nil || inner == nil {
		return
	}
	nt := d.nodeType(node)
	if nt == "subexpression" {
		// Walk the subexpression's children to extract commands.
		innerCC := &CompoundCommand{}
		d.walkNamedChildren(node, input, innerCC)
		*inner = append(*inner, innerCC.AllCommands()...)
		return
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		d.collectSubexpressionCommands(node.NamedChild(i), input, inner)
	}
}

// resolveExprText drills through PowerShell expression wrappers to get the text value.
// PS grammar wraps values deeply: array_literal_expression > unary_expression > string_literal > ...
func (d *PowerShellDecomposer) resolveExprText(node *gotreesitter.Node, input []byte) string {
	if node == nil {
		return ""
	}
	nt := d.nodeType(node)

	switch nt {
	case "string_literal", "expandable_string_literal":
		text := nodeText(node, input)
		return trimQuotes(text, '"')

	case "verbatim_string_literal":
		text := nodeText(node, input)
		return trimQuotes(text, '\'')

	case "variable":
		return nodeText(node, input)

	case "generic_token":
		return nodeText(node, input)

	case "command_name":
		return nodeText(node, input)

	case "array_literal_expression", "unary_expression":
		// Drill down to the actual value.
		if node.NamedChildCount() > 0 {
			return d.resolveExprText(node.NamedChild(0), input)
		}
		return nodeText(node, input)

	default:
		// For other expression types, try to drill down.
		if node.NamedChildCount() == 1 {
			return d.resolveExprText(node.NamedChild(0), input)
		}
		return nodeText(node, input)
	}
}

// ---------- PowerShell flag abbreviation expansion ----------

// psKnownParams maps security-critical cmdlet names (lowercase) to their
// known parameter names (lowercase, with leading dash).
var psKnownParams = map[string][]string{
	"remove-item": {
		"-recurse", "-force", "-path", "-literalpath",
		"-filter", "-include", "-exclude",
		"-whatif", "-confirm", "-erroraction",
	},
	"stop-process": {
		"-force", "-name", "-id", "-inputobject",
		"-whatif", "-confirm", "-passthru",
	},
	"clear-content": {
		"-force", "-path", "-literalpath",
		"-filter", "-include", "-exclude",
		"-whatif", "-confirm",
	},
	"stop-service": {
		"-force", "-name", "-displayname", "-inputobject",
		"-whatif", "-confirm", "-passthru", "-nowaiting",
	},
	"restart-computer": {
		"-force", "-computername", "-timeout",
		"-whatif", "-confirm", "-delay",
	},
	"stop-computer": {
		"-force", "-computername",
		"-whatif", "-confirm",
	},
	"format-volume": {
		"-force", "-filesystem", "-newfilesystemlabel",
		"-whatif", "-confirm",
	},
	"clear-disk": {
		"-removedata", "-removeoem",
		"-whatif", "-confirm",
	},
	"invoke-expression": {
		"-command",
	},
}

// expandPSFlag expands unambiguous flag abbreviations for known cmdlets.
// e.g., for remove-item: -Rec → -recurse, -Fo → -force
func expandPSFlag(cmdName, flag string) string {
	if !isFlag(flag) {
		return flag
	}
	params, ok := psKnownParams[cmdName]
	if !ok {
		return flag
	}
	var match string
	for _, p := range params {
		if strings.HasPrefix(p, flag) {
			if match != "" {
				return flag // Ambiguous prefix — don't expand.
			}
			match = p
		}
	}
	if match != "" {
		return match
	}
	return flag
}
