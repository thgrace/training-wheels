package ast

// SimpleCommand represents a decomposed shell command with its name, flags, and arguments.
type SimpleCommand struct {
	Name            string   // Normalized command name (no path prefix, no .exe suffix)
	Subcommand      string   // Extracted subcommand for multi-level CLIs (git reset, docker rm)
	Flags           []string // Command flags (e.g., "-r", "--recursive", "-rf")
	Args            []string // Non-flag arguments (after subcommand extraction)
	OutputRedirects []string // Non-stdin redirect targets (e.g. ">", ">>")
	Raw             string   // Original command text before decomposition
}

// PipelineStage represents one command in a pipeline.
type PipelineStage struct {
	Command SimpleCommand
	Inner   []SimpleCommand // Commands extracted from command/process substitutions
}

// Statement represents a pipeline, potentially connected to the next
// statement by an operator (&&, ||, ;, &).
type Statement struct {
	Stages   []PipelineStage
	Operator string // "&&", "||", ";", "&", or "" for last statement
}

// CompoundCommand is the root result of decomposing a shell command string.
type CompoundCommand struct {
	Statements []Statement
}

// AllCommands returns a flat slice of all SimpleCommands, including
// inner commands from command substitutions.
func (cc *CompoundCommand) AllCommands() []SimpleCommand {
	if cc == nil {
		return nil
	}
	var cmds []SimpleCommand
	for _, stmt := range cc.Statements {
		for _, stage := range stmt.Stages {
			cmds = append(cmds, stage.Command)
			cmds = append(cmds, stage.Inner...)
		}
	}
	return cmds
}
