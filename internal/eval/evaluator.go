package eval

import (
	"context"
	"strings"
	"time"

	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/normalize"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/session"
	shellcontext "github.com/thgrace/training-wheels/internal/shellcontext"
)

// EvaluationDecision is the top-level verdict.
type EvaluationDecision int

const (
	DecisionAllow EvaluationDecision = iota
	DecisionDeny
	DecisionWarn
	DecisionAsk
)

func (d EvaluationDecision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	case DecisionWarn:
		return "warn"
	case DecisionAsk:
		return "ask"
	default:
		return "unknown"
	}
}

// MatchSource identifies which subsystem produced the match.
type MatchSource int

const (
	SourcePack          MatchSource = iota
	SourceOverrideDeny              // matched an override deny entry
	SourceSessionAllow              // matched a session allow entry
)

// MatchSpan is a byte-offset range within the evaluated command string.
type MatchSpan struct {
	Start int
	End   int
}

// PatternMatch carries all information about a denying match.
type PatternMatch struct {
	PackID      string
	PatternName string
	RuleID      string // "{PackID}:{PatternName}"
	Severity    string
	Reason      string
	Explanation string
	Suggestions []packs.PatternSuggestion
	Source      MatchSource
	MatchedSpan *MatchSpan
}

// EvaluationResult is the complete output of EvaluateCommand.
type EvaluationResult struct {
	Decision           EvaluationDecision
	PatternInfo        *PatternMatch
	OverrideEntry      *override.Entry // Set if an override entry affected the decision
	SessionEntry       *session.Entry  // Set if a session allow entry affected the decision
	SkippedDueToBudget bool
	Command            string
	NormalizedCommand  string
}

// Evaluator holds the pre-built state for command evaluation.
type Evaluator struct {
	cfg              *config.Config
	registry         *packs.PackRegistry
	packs            []*packs.Pack
	packIDs          []string
	kwIndex          *EnabledKeywordIndex
	userOverrides    *override.Overrides
	projectOverrides *override.Overrides
	shell            shellcontext.Shell
	trace            TraceCollector
	sessionAllows    *session.Allowlist
	minSeverity      packs.Severity
}

// NewEvaluator creates an Evaluator from a config and registry.
func NewEvaluator(cfg *config.Config, registry *packs.PackRegistry) *Evaluator {
	// Resolve active pack IDs (expanded enabled minus disabled).
	activeIDs, _ := registry.ResolveEnabledSet(cfg.Packs.Enabled, cfg.Packs.Disabled)

	// Build active pack list.
	var activePacks []*packs.Pack
	var resolvedIDs []string
	var kwEntries []PackKeywords

	for _, id := range activeIDs {
		p := registry.Get(id)
		if p == nil {
			continue
		}
		idx := len(activePacks)
		activePacks = append(activePacks, p)
		resolvedIDs = append(resolvedIDs, id)
		kwEntries = append(kwEntries, PackKeywords{
			PackIndex: idx,
			Keywords:  p.Keywords,
		})
	}

	minSev := packs.SeverityHigh // default: enforce critical + high
	if parsed, err := packs.ParseSeverity(cfg.Packs.MinSeverity); err == nil {
		minSev = parsed
	}

	return &Evaluator{
		cfg:         cfg,
		registry:    registry,
		packs:       activePacks,
		packIDs:     resolvedIDs,
		kwIndex:     NewEnabledKeywordIndex(kwEntries),
		shell:       shellcontext.DefaultShell(),
		minSeverity: minSev,
	}
}

// SetTrace sets the trace collector for this evaluator.
func (e *Evaluator) SetTrace(t TraceCollector) {
	e.trace = t
}

// SetShell sets the shell context for evaluation (affects sanitization).
func (e *Evaluator) SetShell(s shellcontext.Shell) {
	if s != nil {
		e.shell = s
	}
}

// SetOverrides sets the user and project overrides for the evaluator.
func (e *Evaluator) SetOverrides(user, project *override.Overrides) {
	e.userOverrides = user
	e.projectOverrides = project
}

// SetSessionAllows sets the session allowlist for the evaluator.
func (e *Evaluator) SetSessionAllows(sa *session.Allowlist) {
	e.sessionAllows = sa
}

// checkOverrideDeny checks if a command should be denied by an override entry.
func (e *Evaluator) checkOverrideDeny(command, ruleID string) *override.Entry {
	return override.CheckDeny(command, ruleID, e.userOverrides, e.projectOverrides)
}

// checkOverrideAllow checks if a denial should be overridden by an allow entry.
func (e *Evaluator) checkOverrideAllow(command, ruleID string) *override.Entry {
	return override.CheckAllow(command, ruleID, e.userOverrides, e.projectOverrides)
}

// severityEnforced returns true if the given severity meets the minimum threshold.
// Severity values: Critical=0, High=1, Medium=2, Low=3 — lower value = more severe.
func (e *Evaluator) severityEnforced(s packs.Severity) bool {
	return s <= e.minSeverity
}

// checkSessionAllow checks if a command is allowed by a session allow entry.
func (e *Evaluator) checkSessionAllow(command, ruleID string) *session.Entry {
	if e.sessionAllows == nil {
		return nil
	}
	return e.sessionAllows.MatchesAllow(command, ruleID)
}

// Evaluate runs the full evaluation pipeline for cmd.
func (e *Evaluator) Evaluate(ctx context.Context, cmd string) *EvaluationResult {
	start := time.Now()
	result := &EvaluationResult{
		Decision: DecisionAllow,
		Command:  cmd,
	}

	// Step 0 — Empty check.
	if strings.TrimSpace(cmd) == "" {
		if e.trace != nil {
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return result
	}

	// Step 0.5 — Pre-normalize.
	preNorm := normalize.PreNormalize(cmd)
	if e.trace != nil {
		e.trace.RecordPreNormalize(cmd, preNorm)
	}

	// Step 1 — Override deny entries.
	if blockEntry := e.checkOverrideDeny(cmd, ""); blockEntry != nil {
		if e.trace != nil {
			e.trace.RecordOverrideCheck("project/user", blockEntry, false)
			e.trace.RecordFinalDecision(DecisionDeny, time.Since(start))
		}
		return &EvaluationResult{
			Decision: DecisionDeny,
			Command:  cmd,
			PatternInfo: &PatternMatch{
				Reason: "command matches override deny: " + blockEntry.Value,
				Source: SourceOverrideDeny,
			},
			OverrideEntry: blockEntry,
		}
	}
	// Also check pre-normalized form.
	if preNorm != cmd {
		if blockEntry := e.checkOverrideDeny(preNorm, ""); blockEntry != nil {
			if e.trace != nil {
				e.trace.RecordOverrideCheck("project/user (pre-normalized)", blockEntry, false)
				e.trace.RecordFinalDecision(DecisionDeny, time.Since(start))
			}
			return &EvaluationResult{
				Decision: DecisionDeny,
				Command:  cmd,
				PatternInfo: &PatternMatch{
					Reason: "command matches override deny: " + blockEntry.Value,
					Source: SourceOverrideDeny,
				},
				OverrideEntry: blockEntry,
			}
		}
	}

	// Step 2 — Override allow entries.
	if allowEntry := e.checkOverrideAllow(cmd, ""); allowEntry != nil {
		if e.trace != nil {
			e.trace.RecordOverrideCheck("project/user", allowEntry, true)
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		result.OverrideEntry = allowEntry
		return result
	}
	if preNorm != cmd {
		if allowEntry := e.checkOverrideAllow(preNorm, ""); allowEntry != nil {
			if e.trace != nil {
				e.trace.RecordOverrideCheck("project/user (pre-normalized)", allowEntry, true)
				e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
			}
			result.OverrideEntry = allowEntry
			return result
		}
	}

	// Step 2.5 — Session allow entries (pre-pack).
	if entry := e.checkSessionAllow(cmd, ""); entry != nil {
		if e.trace != nil {
			e.trace.RecordSessionAllowCheck("pre-pack", entry, true)
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return &EvaluationResult{
			Decision:     DecisionAllow,
			Command:      cmd,
			SessionEntry: entry,
		}
	}
	if preNorm != cmd {
		if entry := e.checkSessionAllow(preNorm, ""); entry != nil {
			if e.trace != nil {
				e.trace.RecordSessionAllowCheck("pre-pack (pre-normalized)", entry, true)
				e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
			}
			return &EvaluationResult{
				Decision:     DecisionAllow,
				Command:      cmd,
				SessionEntry: entry,
			}
		}
	}

	// Step 3 — Quick-reject on pre-normalized command.
	rejected, candidateMask := e.kwIndex.QuickReject(preNorm)
	if e.trace != nil {
		var candidateIDs []string
		for i := range e.packs {
			if isBitSet(candidateMask, i) {
				candidateIDs = append(candidateIDs, e.packs[i].ID)
			}
		}
		e.trace.RecordQuickReject(preNorm, candidateIDs)
	}
	if rejected {
		if e.trace != nil {
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return result
	}

	// Step 4 — Sanitize the pre-normalized command.
	sanitizedCmd := shellcontext.Sanitize(preNorm, e.shell)
	if e.trace != nil {
		e.trace.RecordSanitization(preNorm, sanitizedCmd)
	}

	// If sanitization removed all keywords, re-check quick-reject.
	if sanitizedCmd != preNorm {
		rejectedSan, maskSan := e.kwIndex.QuickReject(sanitizedCmd)
		if rejectedSan {
			if e.trace != nil {
				e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
			}
			return result
		}
		// Merge candidate masks.
		candidateMask[0] |= maskSan[0]
		candidateMask[1] |= maskSan[1]
	}

	// Step 5 — Full normalize.
	normalizedCmd := normalize.NormalizeCommand(sanitizedCmd)
	result.NormalizedCommand = normalizedCmd
	if e.trace != nil {
		e.trace.RecordNormalization(sanitizedCmd, normalizedCmd)
	}

	if normalizedCmd != sanitizedCmd {
		rejected2, mask2 := e.kwIndex.QuickReject(normalizedCmd)
		if !rejected2 {
			candidateMask[0] |= mask2[0]
			candidateMask[1] |= mask2[1]
		}
	}

	// Step 6 — Deadline check.
	select {
	case <-ctx.Done():
		if e.trace != nil {
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return &EvaluationResult{
			Decision:           DecisionAllow,
			SkippedDueToBudget: true,
			Command:            cmd,
			NormalizedCommand:  normalizedCmd,
		}
	default:
	}

	// Step 7 — Pack evaluation.
	evalCmd := sanitizedCmd

	// Pass 1: Safe patterns.
	safeMatched := make([]bool, len(e.packs))
	for i, p := range e.packs {
		isCandidate := isBitSet(candidateMask, i)
		if !isCandidate {
			if e.trace != nil {
				e.trace.RecordPackEvaluation(p.ID, true, false, nil)
			}
			continue
		}
		if p.MatchesSafe(evalCmd) || (normalizedCmd != evalCmd && p.MatchesSafe(normalizedCmd)) {
			safeMatched[i] = true
			if e.trace != nil {
				e.trace.RecordPackEvaluation(p.ID, false, true, nil)
			}
		}
	}

	// Pass 2: Destructive patterns.
	for i, p := range e.packs {
		if !isBitSet(candidateMask, i) || safeMatched[i] {
			continue
		}

		// Deadline check per pack.
		select {
		case <-ctx.Done():
			if e.trace != nil {
				e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
			}
			return &EvaluationResult{
				Decision:           DecisionAllow,
				SkippedDueToBudget: true,
				Command:            cmd,
				NormalizedCommand:  normalizedCmd,
			}
		default:
		}

		// Try sanitized command.
		if dm := p.MatchesDestructive(evalCmd); dm != nil {
			if !e.severityEnforced(dm.Severity) {
				if e.trace != nil {
					e.trace.RecordPackEvaluation(p.ID, false, false, nil)
				}
				continue
			}
			ruleID := p.ID + ":" + dm.Name
			match := &PatternMatch{
				PackID:      p.ID,
				PatternName: dm.Name,
				RuleID:      ruleID,
				Severity:    dm.Severity.String(),
				Reason:      dm.Reason,
				Explanation: dm.Explanation,
				Suggestions: dm.Suggestions,
				Source:      SourcePack,
				MatchedSpan: &MatchSpan{Start: dm.MatchStart, End: dm.MatchEnd},
			}
			if e.trace != nil {
				e.trace.RecordPackEvaluation(p.ID, false, false, match)
			}

			// Check override allow before denying.
			if ovEntry := e.checkOverrideAllow(cmd, ruleID); ovEntry != nil {
				if e.trace != nil {
					e.trace.RecordOverrideCheck("allow-after-match", ovEntry, true)
					e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
				}
				return &EvaluationResult{
					Decision:          DecisionAllow,
					Command:           cmd,
					OverrideEntry:     ovEntry,
					NormalizedCommand: normalizedCmd,
				}
			}
			// Check session allow before denying.
			if saEntry := e.checkSessionAllow(cmd, ruleID); saEntry != nil {
				if e.trace != nil {
					e.trace.RecordSessionAllowCheck("post-match", saEntry, true)
					e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
				}
				return &EvaluationResult{
					Decision:          DecisionAllow,
					Command:           cmd,
					SessionEntry:      saEntry,
					NormalizedCommand: normalizedCmd,
				}
			}
			if e.trace != nil {
				e.trace.RecordFinalDecision(DecisionDeny, time.Since(start))
			}
			return &EvaluationResult{
				Decision:          DecisionDeny,
				Command:           cmd,
				PatternInfo:       match,
				NormalizedCommand: normalizedCmd,
			}
		}

		// Try normalized command.
		if normalizedCmd != evalCmd {
			if dm := p.MatchesDestructive(normalizedCmd); dm != nil {
				if !e.severityEnforced(dm.Severity) {
					continue
				}
				ruleID := p.ID + ":" + dm.Name
				match := &PatternMatch{
					PackID:      p.ID,
					PatternName: dm.Name,
					RuleID:      ruleID,
					Severity:    dm.Severity.String(),
					Reason:      dm.Reason,
					Explanation: dm.Explanation,
					Suggestions: dm.Suggestions,
					Source:      SourcePack,
					MatchedSpan: &MatchSpan{Start: dm.MatchStart, End: dm.MatchEnd},
				}
				if e.trace != nil {
					e.trace.RecordPackEvaluation(p.ID, false, false, match)
				}

				if ovEntry := e.checkOverrideAllow(cmd, ruleID); ovEntry != nil {
					if e.trace != nil {
						e.trace.RecordOverrideCheck("allow-after-match", ovEntry, true)
						e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
					}
					return &EvaluationResult{
						Decision:          DecisionAllow,
						Command:           cmd,
						OverrideEntry:     ovEntry,
						NormalizedCommand: normalizedCmd,
					}
				}
				if saEntry := e.checkSessionAllow(cmd, ruleID); saEntry != nil {
					if e.trace != nil {
						e.trace.RecordSessionAllowCheck("post-match (normalized)", saEntry, true)
						e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
					}
					return &EvaluationResult{
						Decision:          DecisionAllow,
						Command:           cmd,
						SessionEntry:      saEntry,
						NormalizedCommand: normalizedCmd,
					}
				}
				if e.trace != nil {
					e.trace.RecordFinalDecision(DecisionDeny, time.Since(start))
				}
				return &EvaluationResult{
					Decision:          DecisionDeny,
					Command:           cmd,
					PatternInfo:       match,
					NormalizedCommand: normalizedCmd,
				}
			}
		}
		
		// If we scanned but no match.
		if e.trace != nil {
			e.trace.RecordPackEvaluation(p.ID, false, false, nil)
		}
	}

	// Step 8 — Default Allow.
	if e.trace != nil {
		e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
	}
	return result
}

// EnabledPackIDs returns the list of enabled pack IDs.
func (e *Evaluator) EnabledPackIDs() []string {
	return e.packIDs
}

func isBitSet(mask [2]uint64, i int) bool {
	if i >= maxPacks {
		return false // out of bitmask range — fail-open
	}
	word := i / 64
	bit := uint64(1) << (i % 64)
	return mask[word]&bit != 0
}
