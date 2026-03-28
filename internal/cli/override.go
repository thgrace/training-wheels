package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/session"
)

var (
	overrideJSON       bool
	overrideAddSession bool
	overrideAddTime    string
	overrideAddPrefix  bool
	overrideAddRule    bool
	overrideAddReason  string
)

var overrideCmd = &cobra.Command{
	Use:   "override",
	Short: "Manage session/time-scoped allow and ask overrides",
	Long: `Manage ephemeral override entries that change TW decisions.

Override actions:
  allow  Permit a command or rule
  ask    Require human confirmation

Scope modes for "override add":
  --session     Until reboot or tw override clear
  --time <dur>  For a duration (e.g., 30m, 4h, 1d)

Match modes (default: exact command match):
  --prefix      Match by command prefix
  --rule        Match by rule ID (e.g., core.git:reset-hard)

For permanent policy (deny, ask, allow), use "tw rule" instead.

Examples:
  tw override add allow --session "rm -rf ./dist"
  tw override add allow --time 4h "git push --force"
  tw override add ask --session "git push --force"
  tw override add ask --time 2h --rule "core.git:reset-hard" --reason "review"
  tw override list
  tw override clear
  tw override remove sa-1a2b`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var overrideAddCmd = &cobra.Command{
	Use:   "add <allow|ask> <command-or-rule>",
	Short: "Add an override entry",
	Args:  cobra.ExactArgs(2),
	RunE:  runOverrideAdd,
}

var overrideListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all override entries",
	Args:  cobra.NoArgs,
	RunE:  runOverrideList,
}

var overrideClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear session and timed override entries",
	Args:  cobra.NoArgs,
	RunE:  runOverrideClear,
}

var overrideRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove an override entry by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runOverrideRemove,
}

func init() {
	bindJSONOutputFlags(overrideCmd.PersistentFlags(), &overrideJSON)

	overrideAddCmd.Flags().BoolVar(&overrideAddSession, "session", false, "Session-scoped (until reboot or tw override clear)")
	overrideAddCmd.Flags().StringVar(&overrideAddTime, "time", "", "Time-scoped (e.g., 30m, 4h, 1d)")
	overrideAddCmd.Flags().BoolVar(&overrideAddPrefix, "prefix", false, "Match by command prefix")
	overrideAddCmd.Flags().BoolVar(&overrideAddRule, "rule", false, "Match by rule ID (e.g., core.git:reset-hard)")
	overrideAddCmd.Flags().StringVar(&overrideAddReason, "reason", "", "Reason for the override entry")

	overrideCmd.AddCommand(overrideAddCmd)
	overrideCmd.AddCommand(overrideListCmd)
	overrideCmd.AddCommand(overrideClearCmd)
	overrideCmd.AddCommand(overrideRemoveCmd)
}

func runOverrideAdd(cmd *cobra.Command, args []string) error {
	action, err := parseOverrideAction(args[0])
	if err != nil {
		return err
	}

	value := args[1]

	if overrideAddPrefix && overrideAddRule {
		return fmt.Errorf("at most one of --prefix or --rule may be specified")
	}

	scopeCount := 0
	if overrideAddSession {
		scopeCount++
	}
	if overrideAddTime != "" {
		scopeCount++
	}
	if scopeCount != 1 {
		return fmt.Errorf("exactly one of --session or --time must be specified")
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	if cfg.Allow.RequireReason && overrideAddReason == "" {
		return fmt.Errorf("--reason is required (allow.require_reason is enabled in config)")
	}

	if !session.IsInteractiveTerminal() {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: tw override add --session/--time requires an interactive terminal\n")
		return silentExit(exitcodes.ConfigError)
	}

	kind := override.SelectorExact
	if overrideAddRule {
		kind = override.SelectorRule
	} else if overrideAddPrefix {
		kind = override.SelectorPrefix
	}

	return runOverrideAddEphemeral(cmd, action.String(), value, kind)
}

func parseOverrideAction(s string) (override.Action, error) {
	switch strings.ToLower(s) {
	case "allow":
		return override.ActionAllow, nil
	case "ask":
		return override.ActionAsk, nil
	default:
		return 0, fmt.Errorf("invalid action %q: must be allow or ask", s)
	}
}

func runOverrideAddEphemeral(cmd *cobra.Command, action, value string, kind override.SelectorKind) error {
	var expiresAt time.Time
	scopeLabel := "session"

	if overrideAddTime != "" {
		dur, err := parseOverrideDuration(overrideAddTime)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", overrideAddTime, err)
		}
		expiresAt = time.Now().Add(dur)
		scopeLabel = fmt.Sprintf("time (%s)", overrideAddTime)
	}

	token, err := session.ReadOrCreateToken()
	if err != nil {
		return exitErrorf(exitcodes.IOError, "failed to read/create session token: %w", err)
	}

	secret, err := session.LoadOrCreateSecret(session.SecretPath())
	if err != nil {
		return exitErrorf(exitcodes.IOError, "failed to load/create session secret: %w", err)
	}

	al, err := session.Load(token, secret)
	if err != nil {
		logger.Warn("failed to load session allowlist, starting fresh", "error", err)
		al = &session.Allowlist{}
	}

	entry := al.Add(secret, action, kind.String(), value, overrideAddReason, expiresAt)
	if err := al.Save(); err != nil {
		return exitErrorf(exitcodes.IOError, "failed to save session allowlist: %w", err)
	}

	expiresStr := "session (until reboot or override clear)"
	if !expiresAt.IsZero() {
		expiresStr = expiresAt.Format(time.RFC3339)
	}

	if useJSONOutput(overrideJSON) {
		return writeJSONOutput(cmd.OutOrStdout(), overrideMutationJSONOutput{
			Operation: "add",
			Entry:     newOverrideJSONEntryFromSession(*entry),
		})
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added %s override: %s %s=%q  expires=%s (id: %s)\n", scopeLabel, action, kind, value, expiresStr, entry.ID)
	return nil
}


func runOverrideList(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	var sessionEntries []session.Entry
	token, err := session.ReadToken()
	if err == nil && token != "" {
		secret, err := session.LoadOrCreateSecret(session.SecretPath())
		if err == nil {
			al, err := session.Load(token, secret)
			if err == nil {
				sessionEntries = al.Entries
			}
		}
	}

	user, project, err := override.LoadMerged()
	if err != nil {
		return exitErrorf(exitcodes.IOError, "failed to load overrides: %w", err)
	}

	totalCount := len(sessionEntries) + len(user.Entries) + len(project.Entries)
	if totalCount == 0 {
		if useJSONOutput(overrideJSON) {
			printOverrideListJSON(w, sessionEntries, user.Entries, project.Entries)
			return nil
		}
		fmt.Fprintln(w, "No override entries.")
		return nil
	}

	if useJSONOutput(overrideJSON) {
		printOverrideListJSON(w, sessionEntries, user.Entries, project.Entries)
		return nil
	}

	if len(sessionEntries) > 0 {
		fmt.Fprintln(w, "Session/time overrides:")
		for _, e := range sessionEntries {
			printSessionOverrideEntry(w, e)
		}
		if len(project.Entries) > 0 || len(user.Entries) > 0 {
			fmt.Fprintln(w)
		}
	}

	if len(project.Entries) > 0 {
		fmt.Fprintf(w, "Permanent project overrides (%s):\n", project.Path())
		for _, e := range project.Entries {
			printPermanentOverrideEntry(w, e)
		}
		if len(user.Entries) > 0 {
			fmt.Fprintln(w)
		}
	}

	if len(user.Entries) > 0 {
		fmt.Fprintf(w, "Permanent user overrides (%s):\n", user.Path())
		for _, e := range user.Entries {
			printPermanentOverrideEntry(w, e)
		}
	}

	return nil
}

func runOverrideClear(cmd *cobra.Command, args []string) error {
	tokenPath := session.TokenPath()
	token, _ := session.ReadToken()

	removed := 0
	if token != "" {
		allowlistPath := session.AllowlistPath(token)
		if err := os.Remove(allowlistPath); err == nil {
			removed++
		}
	}
	if err := os.Remove(tokenPath); err == nil {
		removed++
	}

	if useJSONOutput(overrideJSON) {
		return writeJSONOutput(cmd.OutOrStdout(), overrideClearJSONOutput{
			Operation: "clear",
			Cleared:   removed > 0,
		})
	}

	if removed > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Cleared session/time override entries and token.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "No session override entries to clear.")
	}
	return nil
}

func runOverrideRemove(cmd *cobra.Command, args []string) error {
	id := args[0]

	if strings.HasPrefix(id, "sa-") {
		if removed, err := removeFromSession(id); err != nil {
			return err
		} else if removed {
			if useJSONOutput(overrideJSON) {
				return writeJSONOutput(cmd.OutOrStdout(), overrideRemoveJSONOutput{
					Operation: "remove",
					RemovedID: id,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed session override entry: %s\n", id)
			return nil
		}
	}

	if strings.HasPrefix(id, "ov-") {
		if removed, err := removeFromOverrides(id, false); err != nil {
			logger.Warn("could not check user overrides", "error", err)
		} else if removed {
			if useJSONOutput(overrideJSON) {
				return writeJSONOutput(cmd.OutOrStdout(), overrideRemoveJSONOutput{
					Operation: "remove",
					RemovedID: id,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed permanent override entry: %s\n", id)
			return nil
		}
		if removed, err := removeFromOverrides(id, true); err != nil {
			logger.Warn("could not check project overrides", "error", err)
		} else if removed {
			if useJSONOutput(overrideJSON) {
				return writeJSONOutput(cmd.OutOrStdout(), overrideRemoveJSONOutput{
					Operation: "remove",
					RemovedID: id,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed permanent override entry: %s\n", id)
			return nil
		}
	}

	if !strings.HasPrefix(id, "sa-") && !strings.HasPrefix(id, "ov-") {
		if removed, err := removeFromSession(id); err != nil {
			return err
		} else if removed {
			if useJSONOutput(overrideJSON) {
				return writeJSONOutput(cmd.OutOrStdout(), overrideRemoveJSONOutput{
					Operation: "remove",
					RemovedID: id,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed session override entry: %s\n", id)
			return nil
		}
		if removed, err := removeFromOverrides(id, false); err != nil {
			logger.Warn("could not check user overrides", "error", err)
		} else if removed {
			if useJSONOutput(overrideJSON) {
				return writeJSONOutput(cmd.OutOrStdout(), overrideRemoveJSONOutput{
					Operation: "remove",
					RemovedID: id,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed permanent override entry: %s\n", id)
			return nil
		}
		if removed, err := removeFromOverrides(id, true); err != nil {
			logger.Warn("could not check project overrides", "error", err)
		} else if removed {
			if useJSONOutput(overrideJSON) {
				return writeJSONOutput(cmd.OutOrStdout(), overrideRemoveJSONOutput{
					Operation: "remove",
					RemovedID: id,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed permanent override entry: %s\n", id)
			return nil
		}
	}

	return fmt.Errorf("entry %q not found in any scope", id)
}

func removeFromSession(id string) (bool, error) {
	token, err := session.ReadToken()
	if err != nil || token == "" {
		return false, nil
	}

	secret, err := session.LoadOrCreateSecret(session.SecretPath())
	if err != nil {
		return false, nil
	}

	al, err := session.Load(token, secret)
	if err != nil {
		return false, nil
	}

	if !al.Remove(id) {
		return false, nil
	}

	if err := al.Save(); err != nil {
		return false, fmt.Errorf("failed to save session allowlist: %w", err)
	}
	return true, nil
}

func parseOverrideDuration(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		n, parseErr := strconv.ParseFloat(numStr, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}

	return 0, err
}

func printSessionOverrideEntry(w io.Writer, e session.Entry) {
	expires := "session"
	if !e.ExpiresAt.IsZero() {
		expires = e.ExpiresAt.Format(time.RFC3339)
	}
	reason := ""
	if e.Reason != "" {
		reason = fmt.Sprintf("  reason=%q", e.Reason)
	}
	fmt.Fprintf(w, "  [%s] allow %s=%q%s  expires=%s\n", e.ID, e.Kind, e.Value, reason, expires)
}

func loadTargetOverrides(project bool) (*override.Overrides, error) {
	if project {
		return override.Load(override.ProjectOverridesPath())
	}
	path, err := override.UserOverridesPath()
	if err != nil {
		return nil, err
	}
	return override.Load(path)
}

func removeFromOverrides(id string, project bool) (bool, error) {
	ov, err := loadTargetOverrides(project)
	if err != nil {
		return false, fmt.Errorf("failed to load overrides: %w", err)
	}

	if !ov.Remove(id) {
		return false, nil
	}

	if err := ov.Save(); err != nil {
		return false, fmt.Errorf("failed to save overrides: %w", err)
	}

	return true, nil
}

func printPermanentOverrideEntry(w io.Writer, e override.Entry) {
	fmt.Fprintf(w, "  [%s] %s %s=%q  reason=%q\n", e.ID, e.Action, e.Kind, e.Value, e.Reason)
}

type overrideJSONEntry struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	Reason  string `json:"reason"`
	Scope   string `json:"scope"`
	Expires string `json:"expires,omitempty"`
}

type overrideMutationJSONOutput struct {
	Operation string             `json:"operation"`
	Entry     *overrideJSONEntry `json:"entry,omitempty"`
}

type overrideRemoveJSONOutput struct {
	Operation string `json:"operation"`
	RemovedID string `json:"removed_id"`
}

type overrideClearJSONOutput struct {
	Operation string `json:"operation"`
	Cleared   bool   `json:"cleared"`
}

func newOverrideJSONEntryFromSession(e session.Entry) *overrideJSONEntry {
	entry := &overrideJSONEntry{
		ID:     e.ID,
		Action: e.Action,
		Kind:   e.Kind,
		Value:  e.Value,
		Reason: e.Reason,
		Scope:  "session",
	}
	if !e.ExpiresAt.IsZero() {
		entry.Expires = e.ExpiresAt.Format(time.RFC3339)
	}
	return entry
}

func newOverrideJSONEntryFromOverride(e override.Entry, scope string) *overrideJSONEntry {
	return &overrideJSONEntry{
		ID:     e.ID,
		Action: e.Action,
		Kind:   e.Kind,
		Value:  e.Value,
		Reason: e.Reason,
		Scope:  scope,
	}
}

func printOverrideListJSON(w io.Writer, sessionEntries []session.Entry, userEntries, projectEntries []override.Entry) {
	entries := make([]overrideJSONEntry, 0, len(sessionEntries)+len(userEntries)+len(projectEntries))
	for _, e := range sessionEntries {
		entries = append(entries, *newOverrideJSONEntryFromSession(e))
	}
	for _, e := range projectEntries {
		entries = append(entries, *newOverrideJSONEntryFromOverride(e, "project"))
	}
	for _, e := range userEntries {
		entries = append(entries, *newOverrideJSONEntryFromOverride(e, "user"))
	}
	if entries == nil {
		entries = []overrideJSONEntry{}
	}
	_ = writeJSONOutput(w, entries)
}
