package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/shellcontext"
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
	explainCmd.Flags().BoolVar(&explainJSON, "json", false, "Output as JSON")
	explainCmd.Flags().StringVar(&explainShell, "shell", "", "Shell context: posix, powershell, cmd")
}

func runExplain(cmd *cobra.Command, args []string) error {
	command := args[0]

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(exitcodes.ConfigError)
	}

	evaluator := eval.NewEvaluator(cfg, packs.DefaultRegistry())

	if explainShell != "" {
		evaluator.SetShell(shellcontext.FromName(explainShell))
	}

	// Enable detailed tracing for explain command.
	traceColl := eval.NewBufferedTraceCollector(command)
	evaluator.SetTrace(traceColl)

	// Load overrides.
	user, project, ovErr := override.LoadMerged()
	if ovErr != nil {
		logger.Warn("override load error", "error", ovErr)
	} else {
		evaluator.SetOverrides(user, project)
	}

	ctx, cancel := evalContext(cfg)
	defer cancel()

	start := time.Now()
	result := evaluator.Evaluate(ctx, command)
	duration := time.Since(start)

	enabledPacks := evaluator.EnabledPackIDs()
	trace := traceColl.GetTrace()

	if explainJSON {
		printExplainJSON(cmd.OutOrStdout(), result, command, duration, enabledPacks, trace)
	} else {
		printExplainPretty(cmd.OutOrStdout(), result, command, duration, enabledPacks, trace)
	}


	os.Exit(exitcodes.Allow)
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

	// Override section (for both allow and deny).
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

	// Override deny match (not from a pack).
	if result.PatternInfo != nil && result.PatternInfo.Source == eval.SourceOverrideDeny {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Match:")
		fmt.Fprintln(w, "  Source:  override deny")
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
	Override         *explainJSONOverride    `json:"override,omitempty"`
	PackSummary      explainJSONPackSummary  `json:"pack_summary"`
	Trace            *eval.EvaluationTrace   `json:"trace,omitempty"`
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

	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(w, string(data))
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
