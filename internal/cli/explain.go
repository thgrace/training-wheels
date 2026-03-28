package cli

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/app"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/packs"
)

var explainJSON bool
var explainShell string

var explainCmd = &cobra.Command{
	Use:   "explain <command>",
	Short: "Show detailed explanation of why a command is allowed or denied",
	Args:  cobra.ExactArgs(1),
	RunE:  runExplain,
}

func init() {
	bindJSONOutputFlags(explainCmd.Flags(), &explainJSON)
	explainCmd.Flags().StringVar(&explainShell, "shell", "", shellFlagUsage)
}

func runExplain(cmd *cobra.Command, args []string) error {
	command := args[0]

	cfg, err := app.LoadConfig()
	if err != nil {
		return exitErrorf(exitcodes.ConfigError, "config error: %w", err)
	}

	shell, err := resolveShellFlag(explainShell)
	if err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), err)
		return silentExit(exitcodes.ConfigError)
	}

	// Enable detailed tracing for explain command.
	traceColl := eval.NewBufferedTraceCollector(command)
	evaluator := app.NewEvaluator(cfg, app.EvalOptions{
		Shell:         shell,
		Trace:         traceColl,
		LoadOverrides: true,
		LoadRules:     true,
		LoadSession:   true,
	})

	ctx, cancel := app.EvalContext(cfg)
	defer cancel()

	start := time.Now()
	result := evaluator.Evaluate(ctx, command)
	duration := time.Since(start)

	enabledPacks := evaluator.EnabledPackIDs()
	trace := traceColl.GetTrace()

	if useJSONOutput(explainJSON) {
		printExplainJSON(cmd.OutOrStdout(), result, command, duration, enabledPacks, trace)
	} else {
		printExplainPretty(cmd.OutOrStdout(), result, command, duration, enabledPacks, trace)
	}

	return nil
}

func printExplainPretty(w io.Writer, result *eval.EvaluationResult, command string, duration time.Duration, enabledPacks []string, trace *eval.EvaluationTrace) {
	fmt.Fprintln(w, "=== TW Explain ===")
	fmt.Fprintln(w)

	decisionStr := strings.ToUpper(result.Decision.String())
	fmt.Fprintf(w, "Decision:   %s\n", decisionStr)
	fmt.Fprintf(w, "Duration:   %s\n", formatDuration(duration))
	fmt.Fprintln(w)

	// Trace section.
	fmt.Fprintln(w, "Evaluation Trace:")
	for _, step := range trace.Steps {
		fmt.Fprintf(w, "  - [%s] %s\n", step.Timestamp.Format("15:04:05.000"), step.Message)
	}
	fmt.Fprintln(w)

	// Command section.
	fmt.Fprintln(w, "Command:")
	fmt.Fprintf(w, "  Input:      %s\n", command)
	if result.NormalizedCommand != "" && result.NormalizedCommand != command {
		fmt.Fprintf(w, "  Normalized: %s\n", result.NormalizedCommand)
	}

	// Override section (for allow, deny, and ask).
	if result.OverrideEntry != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Override:")
		fmt.Fprintf(w, "  ID:     %s\n", result.OverrideEntry.ID)
		fmt.Fprintf(w, "  Action: %s\n", result.OverrideEntry.Action)
		fmt.Fprintf(w, "  Kind:   %s\n", result.OverrideEntry.Kind)
		fmt.Fprintf(w, "  Value:  %s\n", result.OverrideEntry.Value)
		if result.OverrideEntry.Reason != "" {
			fmt.Fprintf(w, "  Reason: %s\n", result.OverrideEntry.Reason)
		}
	}

	if result.SessionEntry != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Session Allow:")
		fmt.Fprintf(w, "  ID:     %s\n", result.SessionEntry.ID)
		fmt.Fprintf(w, "  Kind:   %s\n", result.SessionEntry.Kind)
		fmt.Fprintf(w, "  Value:  %s\n", result.SessionEntry.Value)
		if result.SessionEntry.Reason != "" {
			fmt.Fprintf(w, "  Reason: %s\n", result.SessionEntry.Reason)
		}
	}

	if result.RuleEntry != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Custom Rule:")
		fmt.Fprintf(w, "  Name:   %s\n", result.RuleEntry.Name)
		fmt.Fprintf(w, "  Kind:   %s\n", result.RuleEntry.Kind)
		if result.RuleEntry.Pattern != "" {
			fmt.Fprintf(w, "  Match:  %s\n", result.RuleEntry.Pattern)
		}
	}

	// Match details (only for deny/warn).
	if result.PatternInfo != nil && result.PatternInfo.Source == eval.SourcePack {
		pi := result.PatternInfo
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Match:")
		fmt.Fprintf(w, "  Rule:       %s\n", pi.RuleID)
		fmt.Fprintf(w, "  Pack:       %s\n", pi.PackID)
		fmt.Fprintf(w, "  Severity:   %s\n", pi.Severity)
		fmt.Fprintf(w, "  Reason:     %s\n", pi.Reason)

		if pi.Explanation != "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "Explanation:")
			for _, line := range strings.Split(pi.Explanation, "\n") {
				fmt.Fprintf(w, "  %s\n", line)
			}
		}

		suggestions := filterSuggestions(pi.Suggestions)
		if len(suggestions) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "Suggestions:")
			for i, s := range suggestions {
				fmt.Fprintf(w, "  %d. %s\n", i+1, s.Command)
				if s.Description != "" {
					fmt.Fprintf(w, "     %s\n", s.Description)
				}
			}
		}
	}

	// Override deny/ask match (not from a pack).
	if result.PatternInfo != nil && result.PatternInfo.Source == eval.SourceOverrideDeny {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Match:")
		fmt.Fprintln(w, "  Source:  override deny")
		fmt.Fprintf(w, "  Reason:  %s\n", result.PatternInfo.Reason)
	}
	if result.PatternInfo != nil && result.PatternInfo.Source == eval.SourceOverrideAsk {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Match:")
		fmt.Fprintln(w, "  Source:  override ask")
		fmt.Fprintf(w, "  Reason:  %s\n", result.PatternInfo.Reason)
	}

	// Pack summary.
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Packs:")
	fmt.Fprintf(w, "  Enabled: %d packs\n", len(enabledPacks))
	if result.PatternInfo != nil && result.PatternInfo.PackID != "" {
		fmt.Fprintf(w, "  Matched: %s\n", result.PatternInfo.PackID)
	}
}

type explainJSONOutput struct {
	SchemaVersion   int                    `json:"schema_version"`
	Command         string                 `json:"command"`
	NormalizedCmd   string                 `json:"normalized_command,omitempty"`
	Decision        string                 `json:"decision"`
	TotalDurationUs int64                  `json:"total_duration_us"`
	Match           *explainJSONMatch      `json:"match,omitempty"`
	Override        *explainJSONOverride   `json:"override,omitempty"`
	PackSummary     explainJSONPackSummary `json:"pack_summary"`
	Trace           *eval.EvaluationTrace  `json:"trace,omitempty"`
}

type explainJSONMatch struct {
	RuleID      string                  `json:"rule_id"`
	PackID      string                  `json:"pack_id"`
	PatternName string                  `json:"pattern_name"`
	Severity    string                  `json:"severity"`
	Reason      string                  `json:"reason"`
	Explanation string                  `json:"explanation,omitempty"`
	Span        *explainJSONSpan        `json:"span,omitempty"`
	Suggestions []explainJSONSuggestion `json:"suggestions,omitempty"`
}

type explainJSONSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type explainJSONSuggestion struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
}

type explainJSONOverride struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Reason string `json:"reason,omitempty"`
}

type explainJSONPackSummary struct {
	EnabledCount int    `json:"enabled_count"`
	MatchedPack  string `json:"matched_pack,omitempty"`
}

func printExplainJSON(w io.Writer, result *eval.EvaluationResult, command string, duration time.Duration, enabledPacks []string, trace *eval.EvaluationTrace) {
	out := explainJSONOutput{
		SchemaVersion:   1,
		Command:         command,
		Decision:        result.Decision.String(),
		TotalDurationUs: duration.Microseconds(),
		PackSummary: explainJSONPackSummary{
			EnabledCount: len(enabledPacks),
		},
		Trace: trace,
	}

	if result.NormalizedCommand != "" && result.NormalizedCommand != command {
		out.NormalizedCmd = result.NormalizedCommand
	}

	if result.PatternInfo != nil && result.PatternInfo.Source == eval.SourcePack {
		pi := result.PatternInfo
		m := &explainJSONMatch{
			RuleID:      pi.RuleID,
			PackID:      pi.PackID,
			PatternName: pi.PatternName,
			Severity:    pi.Severity,
			Reason:      pi.Reason,
			Explanation: pi.Explanation,
		}
		if pi.MatchedSpan != nil {
			m.Span = &explainJSONSpan{
				Start: pi.MatchedSpan.Start,
				End:   pi.MatchedSpan.End,
			}
		}
		suggestions := filterSuggestions(pi.Suggestions)
		for _, s := range suggestions {
			m.Suggestions = append(m.Suggestions, explainJSONSuggestion{
				Command:     s.Command,
				Description: s.Description,
				Platform:    platformString(s.Platform),
			})
		}
		out.Match = m
		out.PackSummary.MatchedPack = pi.PackID
	}

	if result.OverrideEntry != nil {
		out.Override = &explainJSONOverride{
			ID:     result.OverrideEntry.ID,
			Action: result.OverrideEntry.Action,
			Kind:   result.OverrideEntry.Kind,
			Value:  result.OverrideEntry.Value,
			Reason: result.OverrideEntry.Reason,
		}
	}

	_ = writeJSONOutput(w, out)
}

// filterSuggestions returns suggestions applicable to the current platform.
func filterSuggestions(suggestions []packs.PatternSuggestion) []packs.PatternSuggestion {
	if len(suggestions) == 0 {
		return nil
	}
	currentPlatform := goOSToPlatform(runtime.GOOS)
	var filtered []packs.PatternSuggestion
	for _, s := range suggestions {
		if s.Platform == packs.PlatformAll || s.Platform == currentPlatform {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func goOSToPlatform(goos string) packs.Platform {
	switch goos {
	case "linux":
		return packs.PlatformLinux
	case "darwin":
		return packs.PlatformMacOS
	case "windows":
		return packs.PlatformWindows
	case "freebsd", "openbsd", "netbsd":
		return packs.PlatformBSD
	default:
		return packs.PlatformAll
	}
}

func platformString(p packs.Platform) string {
	switch p {
	case packs.PlatformAll:
		return "all"
	case packs.PlatformLinux:
		return "linux"
	case packs.PlatformMacOS:
		return "macos"
	case packs.PlatformWindows:
		return "windows"
	case packs.PlatformBSD:
		return "bsd"
	default:
		return "all"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fus", float64(d.Microseconds()))
	}
	return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000.0)
}
