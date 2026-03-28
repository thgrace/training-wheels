package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/rules"
)

var (
	ruleJSON           bool
	ruleAddName        string
	ruleAddRuleID      string
	ruleAddReason      string
	ruleAddSeverity    string
	ruleAddExplanation string
	ruleAddSuggest     []string
	ruleAddKeyword     []string
	ruleAddProject     bool
	ruleAddDryRun      bool
	ruleRemoveProject  bool
	ruleRemoveYes      bool

	// v2 structural flags
	ruleAddCommand     []string
	ruleAddSubcommand  []string
	ruleAddFlag        []string
	ruleAddAllFlags    []string
	ruleAddArgExact    []string
	ruleAddArgPrefix   []string
	ruleAddArgContains []string
	ruleAddUnlessFlag  []string
	ruleAddUnlessArg   []string
)

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Manage custom rules",
	Long: `Manage custom deny, ask, and allow rules.

Actions:
  deny   Deny a command pattern
  ask    Require human confirmation
  allow  Permit a command or rule-ID

Match kinds (exactly one required for add):
  --command  Match by structural command conditions (deny/ask/allow)
  --rule     Match by rule-ID (allow action only)

Structural condition flags (for --command rules):
  --subcommand   Subcommand to match (repeatable)
  --flag         Any flag triggers match (repeatable, OR)
  --all-flags    All flags must be present (repeatable, AND)
  --arg-exact    Arg equals value (repeatable)
  --arg-prefix   Arg starts with value (repeatable)
  --arg-contains Arg contains value (repeatable)
  --unless-flag  Exempt if flag present (repeatable)
  --unless-arg   Exempt if arg equals value (repeatable)

Examples:
  tw rule add deny --name no-rm-rf --command rm --flag "-rf" --arg-prefix "/" --reason "dangerous"
  tw rule add deny --name no-force-push --command git --subcommand push --flag "--force" --unless-flag "--force-with-lease" --reason "use lease"
  tw rule add allow --name allow-git-status --command git --subcommand status --reason "safe"
  tw rule add allow --name allow-reset --rule core.git:reset-hard --reason "reviewed"
  tw rule list
  tw rule remove my-rule --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var ruleAddCmd = &cobra.Command{
	Use:   "add <deny|ask|allow>",
	Short: "Add a custom rule",
	Args:  cobra.ExactArgs(1),
	RunE:  runRuleAdd,
}

var ruleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all custom rules",
	Args:  cobra.NoArgs,
	RunE:  runRuleList,
}

var ruleRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a custom rule by name",
	Args:  cobra.ExactArgs(1),
	RunE:  runRuleRemove,
}

func init() {
	bindJSONOutputFlags(ruleCmd.PersistentFlags(), &ruleJSON)

	ruleAddCmd.Flags().StringVar(&ruleAddName, "name", "", "Rule name (required, lowercase, e.g. no-rm-rf)")
	ruleAddCmd.Flags().StringVar(&ruleAddRuleID, "rule", "", "Match by rule-ID (allow action only)")
	ruleAddCmd.Flags().StringVar(&ruleAddReason, "reason", "", "Reason for the rule (required)")
	ruleAddCmd.Flags().StringVar(&ruleAddSeverity, "severity", "", "Severity: critical, high, medium, low (default: medium)")
	ruleAddCmd.Flags().StringVar(&ruleAddExplanation, "explanation", "", "Detailed explanation (default: same as reason)")
	ruleAddCmd.Flags().StringSliceVar(&ruleAddSuggest, "suggest", nil, "Safer alternative (format: cmd||description, repeatable)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddKeyword, "keyword", nil, "Quick-reject keyword (repeatable)")
	ruleAddCmd.Flags().BoolVar(&ruleAddProject, "project", false, "Add to project-level rules (default: user)")
	ruleAddCmd.Flags().BoolVar(&ruleAddDryRun, "dry-run", false, "Show rule entry as JSON without saving")

	// v2 structural condition flags
	ruleAddCmd.Flags().StringArrayVar(&ruleAddCommand, "command", nil, "Command name to match (required for deny/ask, repeatable)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddSubcommand, "subcommand", nil, "Subcommand to match (repeatable)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddFlag, "flag", nil, "Flag that triggers match (repeatable, OR)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddAllFlags, "all-flags", nil, "All flags must be present (repeatable, AND)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddArgExact, "arg-exact", nil, "Arg equals value (repeatable)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddArgPrefix, "arg-prefix", nil, "Arg starts with value (repeatable)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddArgContains, "arg-contains", nil, "Arg contains value (repeatable)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddUnlessFlag, "unless-flag", nil, "Exempt if flag present (repeatable)")
	ruleAddCmd.Flags().StringArrayVar(&ruleAddUnlessArg, "unless-arg", nil, "Exempt if arg equals value (repeatable)")

	ruleRemoveCmd.Flags().BoolVar(&ruleRemoveProject, "project", false, "Remove from project-level rules (default: user)")
	ruleRemoveCmd.Flags().BoolVar(&ruleRemoveYes, "yes", false, "Skip confirmation prompt")

	ruleCmd.AddCommand(ruleAddCmd)
	ruleCmd.AddCommand(ruleListCmd)
	ruleCmd.AddCommand(ruleRemoveCmd)
}

func runRuleAdd(cmd *cobra.Command, args []string) error {
	action := strings.ToLower(args[0])
	if action != "deny" && action != "ask" && action != "allow" {
		return fmt.Errorf("invalid action %q: must be deny, ask, or allow", args[0])
	}

	if ruleAddName == "" {
		return fmt.Errorf("--name is required")
	}
	if ruleAddReason == "" {
		return fmt.Errorf("--reason is required")
	}

	// Determine kind: --command → "command", --rule → "rule".
	hasCommand := len(ruleAddCommand) > 0
	hasRule := ruleAddRuleID != ""

	if !hasCommand && !hasRule {
		return fmt.Errorf("exactly one of --command or --rule must be specified")
	}
	if hasCommand && hasRule {
		return fmt.Errorf("exactly one of --command or --rule must be specified")
	}

	// --rule is only valid with allow action.
	if hasRule && action != "allow" {
		return fmt.Errorf("--rule is only valid with allow action")
	}

	var kind string
	var entry rules.RuleEntry

	if hasRule {
		kind = "rule"
		entry = rules.RuleEntry{
			Name:    ruleAddName,
			Action:  action,
			Kind:    kind,
			Pattern: ruleAddRuleID,
			Reason:  ruleAddReason,
		}
	} else {
		kind = "command"

		// Build When condition from structural flags.
		when := packs.PatternCondition{
			Command:     ruleAddCommand,
			Subcommand:  ruleAddSubcommand,
			Flag:        ruleAddFlag,
			AllFlags:    ruleAddAllFlags,
			ArgExact:    ruleAddArgExact,
			ArgPrefix:   ruleAddArgPrefix,
			ArgContains: ruleAddArgContains,
		}

		// Build Unless condition if any unless flags are set.
		var unless *packs.PatternCondition
		if len(ruleAddUnlessFlag) > 0 || len(ruleAddUnlessArg) > 0 {
			unless = &packs.PatternCondition{
				Flag:     ruleAddUnlessFlag,
				ArgExact: ruleAddUnlessArg,
			}
		}

		entry = rules.RuleEntry{
			Name:   ruleAddName,
			Action: action,
			Kind:   kind,
			When:   &when,
			Unless: unless,
			Reason: ruleAddReason,
		}
	}

	// Defaults for deny/ask.
	severity := ruleAddSeverity
	if severity == "" && (action == "deny" || action == "ask") {
		severity = "medium"
	}
	entry.Severity = severity

	explanation := ruleAddExplanation
	if explanation == "" && (action == "deny" || action == "ask") {
		explanation = ruleAddReason
	}
	entry.Explanation = explanation

	// Parse --suggest flags.
	var suggestions []rules.Suggestion
	for _, s := range ruleAddSuggest {
		parts := strings.SplitN(s, "||", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --suggest format %q: expected cmd||description", s)
		}
		suggestions = append(suggestions, rules.Suggestion{
			Command:     strings.TrimSpace(parts[0]),
			Description: strings.TrimSpace(parts[1]),
		})
	}
	entry.Suggestions = suggestions

	// Determine keywords.
	var keywords []string
	if len(ruleAddKeyword) > 0 {
		keywords = ruleAddKeyword
	} else if kind == "command" && entry.When != nil {
		// Auto-keywords from When.Command.
		for _, cmd := range entry.When.Command {
			if len(cmd) >= 2 {
				keywords = append(keywords, cmd)
			}
		}
	}
	entry.Keywords = keywords

	// Dry-run: print JSON and return without saving.
	if ruleAddDryRun {
		return writeJSONOutput(cmd.OutOrStdout(), entry)
	}

	// Load target rules file.
	rf, err := loadTargetRules(ruleAddProject)
	if err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	if err := rf.Add(entry); err != nil {
		return err
	}

	if useJSONOutput(ruleJSON) {
		return writeJSONOutput(cmd.OutOrStdout(), ruleMutationJSONOutput{
			Operation: "add",
			Entry:     entry,
		})
	}

	scope := "user"
	if ruleAddProject {
		scope = "project"
	}

	// Format display string based on kind.
	var display string
	if kind == "rule" {
		display = fmt.Sprintf("rule=%q", ruleAddRuleID)
	} else {
		display = fmt.Sprintf("command=%v", entry.When.Command)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added %s rule: %s %s %s (%s)\n", scope, ruleAddName, action, display, ruleAddReason)
	return nil
}

func runRuleList(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	userPath, err := rules.UserRulesPath()
	if err != nil {
		return fmt.Errorf("failed to determine user rules path: %w", err)
	}
	userRF, err := rules.LoadOrCreate(userPath)
	if err != nil {
		return fmt.Errorf("failed to load user rules: %w", err)
	}

	projectPath := rules.ProjectRulesPath()
	projectRF, err := rules.LoadOrCreate(projectPath)
	if err != nil {
		return fmt.Errorf("failed to load project rules: %w", err)
	}

	userRules := userRF.List()
	projectRules := projectRF.List()

	if len(userRules) == 0 && len(projectRules) == 0 {
		if useJSONOutput(ruleJSON) {
			return writeJSONOutput(w, []ruleListJSONEntry{})
		}
		fmt.Fprintln(w, "No rules.")
		return nil
	}

	if useJSONOutput(ruleJSON) {
		return printRuleListJSON(w, userRules, projectRules)
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SOURCE\tNAME\tACTION\tKIND\tMATCH\tREASON")
	for _, r := range userRules {
		fmt.Fprintf(tw, "User\t%s\t%s\t%s\t%s\t%s\n", r.Name, r.Action, r.Kind, formatRuleMatch(r), r.Reason)
	}
	for _, r := range projectRules {
		fmt.Fprintf(tw, "Project\t%s\t%s\t%s\t%s\t%s\n", r.Name, r.Action, r.Kind, formatRuleMatch(r), r.Reason)
	}
	_ = tw.Flush()
	return nil
}

// formatRuleMatch returns a human-readable match description for a rule entry.
func formatRuleMatch(r rules.RuleEntry) string {
	switch r.Kind {
	case "command":
		if r.When == nil {
			return "<no conditions>"
		}
		var parts []string
		if len(r.When.Command) > 0 {
			parts = append(parts, "cmd="+strings.Join(r.When.Command, ","))
		}
		if len(r.When.Subcommand) > 0 {
			parts = append(parts, "sub="+strings.Join(r.When.Subcommand, ","))
		}
		if len(r.When.Flag) > 0 {
			parts = append(parts, "flag="+strings.Join(r.When.Flag, ","))
		}
		if len(r.When.AllFlags) > 0 {
			parts = append(parts, "all-flags="+strings.Join(r.When.AllFlags, ","))
		}
		if len(r.When.ArgExact) > 0 {
			parts = append(parts, "arg="+strings.Join(r.When.ArgExact, ","))
		}
		if len(r.When.ArgPrefix) > 0 {
			parts = append(parts, "arg-pfx="+strings.Join(r.When.ArgPrefix, ","))
		}
		if len(r.When.ArgContains) > 0 {
			parts = append(parts, "arg-has="+strings.Join(r.When.ArgContains, ","))
		}
		result := strings.Join(parts, " ")
		if r.Unless != nil {
			var uparts []string
			if len(r.Unless.Flag) > 0 {
				uparts = append(uparts, "flag="+strings.Join(r.Unless.Flag, ","))
			}
			if len(r.Unless.ArgExact) > 0 {
				uparts = append(uparts, "arg="+strings.Join(r.Unless.ArgExact, ","))
			}
			if len(uparts) > 0 {
				result += " unless(" + strings.Join(uparts, " ") + ")"
			}
		}
		return result
	case "rule":
		return r.Pattern
	default:
		// Legacy v1 kinds.
		return r.Pattern
	}
}

func runRuleRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Require --yes unless --json is set.
	if !ruleRemoveYes && !useJSONOutput(ruleJSON) {
		return fmt.Errorf("remove requires --yes to confirm (or use --json)")
	}

	rf, err := loadTargetRules(ruleRemoveProject)
	if err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	found, saveErr := rf.Remove(name)
	if !found {
		return fmt.Errorf("rule %q not found", name)
	}
	if saveErr != nil {
		return fmt.Errorf("removed rule %q but failed to save: %w", name, saveErr)
	}

	if useJSONOutput(ruleJSON) {
		return writeJSONOutput(cmd.OutOrStdout(), ruleRemoveJSONOutput{
			Operation:   "remove",
			RemovedName: name,
		})
	}

	scope := "user"
	if ruleRemoveProject {
		scope = "project"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed %s rule: %s\n", scope, name)
	return nil
}

func loadTargetRules(project bool) (*rules.RulesFile, error) {
	if project {
		return rules.LoadOrCreate(rules.ProjectRulesPath())
	}
	path, err := rules.UserRulesPath()
	if err != nil {
		return nil, err
	}
	return rules.LoadOrCreate(path)
}

// JSON output types.

type ruleMutationJSONOutput struct {
	Operation string          `json:"operation"`
	Entry     rules.RuleEntry `json:"entry"`
}

type ruleRemoveJSONOutput struct {
	Operation   string `json:"operation"`
	RemovedName string `json:"removed_name"`
}

type ruleListJSONEntry struct {
	Source string `json:"source"`
	rules.RuleEntry
}

func printRuleListJSON(w interface{ Write([]byte) (int, error) }, userRules, projectRules []rules.RuleEntry) error {
	entries := make([]ruleListJSONEntry, 0, len(userRules)+len(projectRules))
	for _, r := range userRules {
		entries = append(entries, ruleListJSONEntry{Source: "user", RuleEntry: r})
	}
	for _, r := range projectRules {
		entries = append(entries, ruleListJSONEntry{Source: "project", RuleEntry: r})
	}
	return writeJSONOutput(w, entries)
}
