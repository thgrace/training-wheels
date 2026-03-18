package cli

import (
	"encoding/json"
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
	allowSession   bool
	allowTime      string
	allowPermanent bool
	allowProject   bool
	allowPrefix    bool
	allowRule      bool
	allowReason    string
	allowList      bool
	allowClear     bool
	allowRemove    string
	allowDeny     bool
)

var allowCmd = &cobra.Command{
	Use:   "allow [command-or-rule]",
	Short: "Allow commands by session, time, or permanently",
	Long: `Add allow entries to bypass TW rules.

Scope modes (exactly one required when adding):
  --session     Until reboot or tw allow --clear
  --time <dur>  For a duration (e.g., 30m, 4h, 1d)
  --permanent   Persistent (writes to overrides.json)

Match modes (default: exact command match):
  --prefix      Match by command prefix
  --rule        Match by rule ID (e.g., core.git:reset-hard)

Management:
  --list        List all entries (session + permanent)
  --clear       Clear session/time entries
  --remove <id> Remove entry by ID

Examples:
  tw allow --session "rm -rf ./dist"
  tw allow --time 4h "git push --force"
  tw allow --permanent "rm -rf ./dist" --reason "build cleanup"
  tw allow --permanent --deny "evil-cmd" --reason "never allow"
  tw allow --list
  tw allow --clear
  tw allow --remove sa-1a2b`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAllow,
}

func init() {
	allowCmd.Flags().BoolVar(&allowSession, "session", false, "Session-scoped (until reboot or tw allow --clear)")
	allowCmd.Flags().StringVar(&allowTime, "time", "", "Time-scoped (e.g., 30m, 4h, 1d)")
	allowCmd.Flags().BoolVar(&allowPermanent, "permanent", false, "Permanent (writes to overrides.json)")
	allowCmd.Flags().BoolVar(&allowProject, "project", false, "Add to project-level overrides (with --permanent only)")
	allowCmd.Flags().BoolVar(&allowPrefix, "prefix", false, "Match by command prefix")
	allowCmd.Flags().BoolVar(&allowRule, "rule", false, "Match by rule ID (e.g., core.git:reset-hard)")
	allowCmd.Flags().StringVar(&allowReason, "reason", "", "Reason for the allow entry")
	allowCmd.Flags().BoolVar(&allowList, "list", false, "List all entries (session + permanent)")
	allowCmd.Flags().BoolVar(&allowClear, "clear", false, "Clear session/time entries and token")
	allowCmd.Flags().StringVar(&allowRemove, "remove", "", "Remove entry by ID (any scope)")
	allowCmd.Flags().BoolVar(&allowDeny, "deny", false, "Deny instead of allow (with --permanent only)")
}

func runAllow(cmd *cobra.Command, args []string) error {
	// Management operations (no positional arg required).
	if allowList {
		return runAllowList(cmd)
	}
	if allowClear {
		return runAllowClear(cmd)
	}
	if allowRemove != "" {
		return runAllowRemove(cmd, allowRemove)
	}

	// Adding an entry — require exactly 1 positional arg.
	if len(args) == 0 {
		return fmt.Errorf("requires a command, prefix, or rule argument")
	}

	// TTY check for ephemeral scopes.
	if allowSession || allowTime != "" {
		if !session.IsInteractiveTerminal() {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: tw allow --session/--time requires an interactive terminal\n")
			os.Exit(exitcodes.ConfigError)
		}
	}

	// Validate scope: exactly one of --session, --time, --permanent must be set.
	scopeCount := 0
	if allowSession {
		scopeCount++
	}
	if allowTime != "" {
		scopeCount++
	}
	if allowPermanent {
		scopeCount++
	}
	if scopeCount != 1 {
		return fmt.Errorf("exactly one of --session, --time, or --permanent must be specified")
	}

	// Load config for RequireReason check.
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Validate --reason.
	if cfg.Allow.RequireReason && allowReason == "" {
		return fmt.Errorf("--reason is required (allow.require_reason is enabled in config)")
	}

	// Validate --project: only valid with --permanent.
	if allowProject && !allowPermanent {
		return fmt.Errorf("--project can only be used with --permanent")
	}

	// Validate --deny: only valid with --permanent.
	if allowDeny && !allowPermanent {
		return fmt.Errorf("--deny can only be used with --permanent")
	}

	// Determine selector kind.
	kind := override.SelectorExact
	if allowRule {
		kind = override.SelectorRule
	} else if allowPrefix {
		kind = override.SelectorPrefix
	}

	// Route to the appropriate handler.
	if allowPermanent {
		return runAllowPermanent(cmd, args[0], kind)
	}
	return runAllowEphemeral(cmd, args[0], kind)
}

func runAllowPermanent(cmd *cobra.Command, value string, kind override.SelectorKind) error {
	// Determine action.
	action := override.ActionAllow
	if allowDeny {
		action = override.ActionDeny
	}

	// Load target overrides file.
	ov, err := loadTargetOverrides(allowProject)
	if err != nil {
		logger.Error("failed to load overrides", "error", err)
		os.Exit(exitcodes.IOError)
	}

	entry := ov.Add(action, kind, value, allowReason)

	if err := ov.Save(); err != nil {
		logger.Error("failed to save overrides", "error", err)
		os.Exit(exitcodes.IOError)
	}

	scope := "user"
	if allowProject {
		scope = "project"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added permanent %s override: %s %s=%q (id: %s)\n", scope, action, kind, value, entry.ID)
	return nil
}

func runAllowEphemeral(cmd *cobra.Command, value string, kind override.SelectorKind) error {
	// Determine expiry.
	var expiresAt time.Time
	scopeLabel := "session"

	if allowTime != "" {
		dur, err := parseDuration(allowTime)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", allowTime, err)
		}
		expiresAt = time.Now().Add(dur)
		scopeLabel = fmt.Sprintf("time (%s)", allowTime)
	}

	token, err := session.ReadOrCreateToken()
	if err != nil {
		logger.Error("failed to read/create session token", "error", err)
		os.Exit(exitcodes.IOError)
	}

	secret, err := session.LoadOrCreateSecret(session.SecretPath())
	if err != nil {
		logger.Error("failed to load/create session secret", "error", err)
		os.Exit(exitcodes.IOError)
	}

	al, err := session.Load(token, secret)
	if err != nil {
		logger.Warn("failed to load session allowlist, starting fresh", "error", err)
		// Use an empty allowlist on error — fail-open philosophy.
		al = &session.Allowlist{}
	}

	entry := al.Add(secret, kind.String(), value, allowReason, expiresAt)

	if err := al.Save(); err != nil {
		logger.Error("failed to save session allowlist", "error", err)
		os.Exit(exitcodes.IOError)
	}

	expiresStr := "session (until reboot or --clear)"
	if !expiresAt.IsZero() {
		expiresStr = expiresAt.Format(time.RFC3339)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added %s allow: %s=%q  expires=%s (id: %s)\n", scopeLabel, kind, value, expiresStr, entry.ID)
	return nil
}

func runAllowList(cmd *cobra.Command) error {
	w := cmd.OutOrStdout()

	// Load session entries.
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

	// Load permanent overrides.
	user, project, err := override.LoadMerged()
	if err != nil {
		logger.Error("failed to load overrides", "error", err)
		os.Exit(exitcodes.IOError)
	}

	totalCount := len(sessionEntries) + len(user.Entries) + len(project.Entries)
	if totalCount == 0 {
		fmt.Fprintln(w, "No allow entries.")
		return nil
	}

	// Session/time entries.
	if len(sessionEntries) > 0 {
		fmt.Fprintln(w, "Session/time entries:")
		for _, e := range sessionEntries {
			printSessionEntry(w, e)
		}
		if len(project.Entries) > 0 || len(user.Entries) > 0 {
			fmt.Fprintln(w)
		}
	}

	// Project permanent overrides.
	if len(project.Entries) > 0 {
		fmt.Fprintf(w, "Permanent project overrides (%s):\n", project.Path())
		for _, e := range project.Entries {
			printOverrideEntry(w, e)
		}
		if len(user.Entries) > 0 {
			fmt.Fprintln(w)
		}
	}

	// User permanent overrides.
	if len(user.Entries) > 0 {
		fmt.Fprintf(w, "Permanent user overrides (%s):\n", user.Path())
		for _, e := range user.Entries {
			printOverrideEntry(w, e)
		}
	}

	return nil
}

func runAllowClear(cmd *cobra.Command) error {
	// Remove the session token file and the allowlist file.
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

	if removed > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Cleared session/time entries and token.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "No session entries to clear.")
	}
	return nil
}

func runAllowRemove(cmd *cobra.Command, id string) error {
	// Try session allowlist for "sa-" prefixed IDs.
	if strings.HasPrefix(id, "sa-") {
		if removed, err := removeFromSession(id); err != nil {
			return err
		} else if removed {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed session entry: %s\n", id)
			return nil
		}
	}

	// Try permanent overrides for "ov-" prefixed IDs.
	if strings.HasPrefix(id, "ov-") {
		// Try user overrides first, then project.
		if err := removeFromOverrides(cmd, id, false); err == nil {
			return nil
		}
		if err := removeFromOverrides(cmd, id, true); err == nil {
			return nil
		}
	}

	// If no prefix match, try everything.
	if !strings.HasPrefix(id, "sa-") && !strings.HasPrefix(id, "ov-") {
		// Try session.
		if removed, err := removeFromSession(id); err != nil {
			return err
		} else if removed {
			fmt.Fprintf(cmd.OutOrStdout(), "Removed session entry: %s\n", id)
			return nil
		}
		// Try permanent overrides.
		if err := removeFromOverrides(cmd, id, false); err == nil {
			return nil
		}
		if err := removeFromOverrides(cmd, id, true); err == nil {
			return nil
		}
	}

	return fmt.Errorf("entry %q not found in any scope", id)
}

// removeFromSession tries to remove a session entry by ID.
// Returns (true, nil) if found and removed, (false, nil) if not found.
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

// parseDuration parses a duration string. Supports standard Go durations
// (h, m, s) and also a "d" suffix for days.
func parseDuration(s string) (time.Duration, error) {
	// Try standard Go duration first.
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	// Try "d" suffix for days.
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

// printSessionEntry prints a session/time allowlist entry.
func printSessionEntry(w io.Writer, e session.Entry) {
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

func removeFromOverrides(cmd *cobra.Command, id string, project bool) error {
	ov, err := loadTargetOverrides(project)
	if err != nil {
		logger.Error("failed to load overrides", "error", err)
		os.Exit(exitcodes.IOError)
	}

	if !ov.Remove(id) {
		scope := "user"
		if project {
			scope = "project"
		}
		return fmt.Errorf("entry %q not found in %s overrides", id, scope)
	}

	if err := ov.Save(); err != nil {
		logger.Error("failed to save overrides", "error", err)
		os.Exit(exitcodes.IOError)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed override entry: %s\n", id)
	return nil
}

func printOverrideEntry(w io.Writer, e override.Entry) {
	fmt.Fprintf(w, "  [%s] %s %s=%q  reason=%q\n", e.ID, e.Action, e.Kind, e.Value, e.Reason)
}

// printAllowListJSON outputs all entries as JSON (for potential future use).
func printAllowListJSON(w io.Writer, sessionEntries []session.Entry, userEntries, projectEntries []override.Entry) {
	type jsonEntry struct {
		ID      string `json:"id"`
		Action  string `json:"action"`
		Kind    string `json:"kind"`
		Value   string `json:"value"`
		Reason  string `json:"reason"`
		Scope   string `json:"scope"`
		Expires string `json:"expires,omitempty"`
	}

	entries := make([]jsonEntry, 0, len(sessionEntries)+len(userEntries)+len(projectEntries))
	for _, e := range sessionEntries {
		expires := ""
		if !e.ExpiresAt.IsZero() {
			expires = e.ExpiresAt.Format(time.RFC3339)
		}
		entries = append(entries, jsonEntry{
			ID: e.ID, Action: "allow", Kind: e.Kind, Value: e.Value,
			Reason: e.Reason, Scope: "session", Expires: expires,
		})
	}
	for _, e := range projectEntries {
		entries = append(entries, jsonEntry{
			ID: e.ID, Action: e.Action, Kind: e.Kind, Value: e.Value,
			Reason: e.Reason, Scope: "project",
		})
	}
	for _, e := range userEntries {
		entries = append(entries, jsonEntry{
			ID: e.ID, Action: e.Action, Kind: e.Kind, Value: e.Value,
			Reason: e.Reason, Scope: "user",
		})
	}
	if entries == nil {
		entries = []jsonEntry{}
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	fmt.Fprintln(w, string(data))
}
