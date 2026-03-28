package eval

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/thgrace/training-wheels/internal/ast"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/normalize"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/rules"
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
	SourceOverrideAsk               // matched an override ask entry
	SourceOverrideAllow             // matched an override allow entry
	SourceSessionAllow              // matched a session allow entry
	SourceRuleAllow                 // matched a custom rule allow entry
	SourceRuleDeny                  // matched a custom rule deny entry
	SourceRuleAsk                   // matched a custom rule ask entry
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
	OverrideEntry      *override.Entry   // Set if an override entry affected the decision
	SessionEntry       *session.Entry    // Set if a session allow entry affected the decision
	RuleEntry          *rules.AllowEntry // Set if a custom rule affected the decision
	SkippedDueToBudget bool
	Command            string
	NormalizedCommand  string
}

// Evaluator holds the pre-built state for command evaluation.
type Evaluator struct {
	cfg                  *config.Config
	registry             *packs.PackRegistry
	basePacks            []*packs.Pack
	basePackIDs          []string
	packs                []*packs.Pack
	packIDs              []string
	kwIndex              *EnabledKeywordIndex
	userOverrides        *override.Overrides
	projectOverrides     *override.Overrides
	shell                shellcontext.Shell
	trace                TraceCollector
	sessionAllows        *session.Allowlist
	minSeverity          packs.Severity
	ruleAllows           []rules.AllowEntry // custom allow rules (pre-pack + post-pack by rule-ID)
	hasCommandAllowRules bool               // true when any allow rule requires parsed commands
	astEnabled           bool               // true when AST parsing is available for this shell
}

// NewEvaluator creates an Evaluator from a config and registry.
func NewEvaluator(cfg *config.Config, registry *packs.PackRegistry) *Evaluator {
	// 1. Mandatory packs: ensure core security and self-protection are ALWAYS enabled.
	mandatory := []string{"core.git", "core.filesystem", "core.tw"}
	if runtime.GOOS == "windows" {
		mandatory = append(mandatory, "windows")
	}

	enabled := make([]string, 0, len(cfg.Packs.Enabled)+len(mandatory))
	enabled = append(enabled, cfg.Packs.Enabled...)
	enabled = append(enabled, mandatory...)

	// Resolve active pack IDs (expanded enabled minus disabled).
	activeIDs, _ := registry.ResolveEnabledSet(enabled, cfg.Packs.Disabled)

	// Build active pack list.
	var activePacks []*packs.Pack
	var resolvedIDs []string
	var kwEntries []PackKeywords

	mandatoryMap := make(map[string]bool)
	for _, id := range mandatory {
		mandatoryMap[id] = true
	}

	for _, id := range activeIDs {
		p := registry.Get(id)
		if p == nil {
			continue
		}

		// Warning: non-mandatory packs with zero keywords will be checked
		// on every command, which can impact performance.
		if len(p.Keywords) == 0 && !mandatoryMap[id] {
			logger.Warn("pack has no keywords; it will be evaluated for every command", "pack", id)
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

	defShell := shellcontext.DefaultShell()
	basePacks := append([]*packs.Pack(nil), activePacks...)
	basePackIDs := append([]string(nil), resolvedIDs...)
	return &Evaluator{
		cfg:         cfg,
		registry:    registry,
		basePacks:   basePacks,
		basePackIDs: basePackIDs,
		packs:       append([]*packs.Pack(nil), basePacks...),
		packIDs:     append([]string(nil), basePackIDs...),
		kwIndex:     NewEnabledKeywordIndex(kwEntries),
		shell:       defShell,
		minSeverity: minSev,
		astEnabled:  defShell.Name() == "posix" || defShell.Name() == "powershell",
	}
}

// SetTrace sets the trace collector for this evaluator.
func (e *Evaluator) SetTrace(t TraceCollector) {
	e.trace = t
}

// SetShell sets the shell context for evaluation (affects sanitization).
// Also enables AST parsing for POSIX shells (Bash/Zsh).
func (e *Evaluator) SetShell(s shellcontext.Shell) {
	if s != nil {
		e.shell = s
		// Enable AST for POSIX shells; CmdExe has no tree-sitter grammar.
		e.astEnabled = s.Name() == "posix" || s.Name() == "powershell"
	}
}

// ASTEnabled reports whether AST-based parsing is active for the current shell.
func (e *Evaluator) ASTEnabled() bool {
	return e.astEnabled
}

// getASTShellType returns the AST shell type for the current shell.
func (e *Evaluator) getASTShellType() ast.ShellType {
	if e.shell != nil && e.shell.Name() == "powershell" {
		return ast.ShellPowerShell
	}
	return ast.ShellBash
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

// SetRules integrates custom user and project rules into the evaluator.
// Deny/ask rules become a synthetic pack registered and enabled.
// Allow rules are stored for pre-pack and post-pack checking.
func (e *Evaluator) SetRules(user, project *rules.RulesFile) {
	e.resetRuleState()

	var allEntries []rules.RuleEntry
	if project != nil {
		allEntries = append(allEntries, project.List()...)
	}
	if user != nil {
		allEntries = append(allEntries, user.List()...)
	}

	if len(allEntries) == 0 {
		return
	}

	// Build synthetic pack from deny/ask entries.
	synPack := rules.ConvertToPack(allEntries, "rules.custom")
	if len(synPack.StructuralPatterns) > 0 {
		_ = e.registry.RegisterPack(synPack, "rules")
		// Add to our active packs and rebuild keyword index.
		e.packs = append(e.packs, synPack)
		e.packIDs = append(e.packIDs, synPack.ID)
		var kwEntries []PackKeywords
		for i, p := range e.packs {
			kwEntries = append(kwEntries, PackKeywords{
				PackIndex: i,
				Keywords:  p.Keywords,
			})
		}
		e.kwIndex = NewEnabledKeywordIndex(kwEntries)
	}

	// Extract allow entries for pre-pack and post-pack checking.
	e.ruleAllows = rules.ConvertToAllowEntries(allEntries, e.getASTShellType())
	e.hasCommandAllowRules = rules.HasCommandAllowEntries(e.ruleAllows)
}

// checkRuleAllow checks if a command or ruleID matches any custom rule allow entry.
func (e *Evaluator) checkRuleAllow(command, ruleID string, simpleCommands []ast.SimpleCommand) *rules.AllowEntry {
	return rules.CheckAllowParsed(command, ruleID, simpleCommands, e.ruleAllows)
}

// checkOverrideAsk checks if a command should require confirmation by an override entry.
func (e *Evaluator) checkOverrideAsk(command, ruleID string) *override.Entry {
	return override.CheckAsk(command, ruleID, e.userOverrides, e.projectOverrides)
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

// checkSessionMatch checks if a command is allowed or asked by a session entry.
func (e *Evaluator) checkSessionMatch(command, ruleID string) *session.Entry {
	if e.sessionAllows == nil {
		return nil
	}
	return e.sessionAllows.Matches(command, ruleID)
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

	// Step 0.5 — Pre-parse cleanup (CR stripping + Windows backslash normalization).
	cleaned := normalize.PreParseCleanup(cmd)
	if e.trace != nil {
		e.trace.RecordPreNormalize(cmd, cleaned)
	}

	var (
		simpleCommands []ast.SimpleCommand
		parsedCommands bool
	)
	parseSimpleCommands := func() []ast.SimpleCommand {
		if parsedCommands {
			return simpleCommands
		}
		parsedCommands = true
		cc := ast.ParseShell([]byte(cleaned), e.getASTShellType())
		if cc == nil {
			return nil
		}
		cc = ast.Unwrap(cc, e.getASTShellType())
		simpleCommands = cc.AllCommands()
		ast.EnrichCommands(simpleCommands)
		return simpleCommands
	}
	var ruleCommands []ast.SimpleCommand
	if e.hasCommandAllowRules {
		ruleCommands = parseSimpleCommands()
	}

	// Step 1 — Rule allow entries (pre-pack, command match only).
	if ra := e.checkRuleAllow(cmd, "", ruleCommands); ra != nil {
		if e.trace != nil {
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return &EvaluationResult{Decision: DecisionAllow, Command: cmd, RuleEntry: ra}
	}
	if cleaned != cmd {
		if ra := e.checkRuleAllow(cleaned, "", ruleCommands); ra != nil {
			if e.trace != nil {
				e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
			}
			return &EvaluationResult{Decision: DecisionAllow, Command: cmd, RuleEntry: ra}
		}
	}

	// Step 2 — Override ask entries (session/time only).
	if askEntry := e.checkOverrideAsk(cmd, ""); askEntry != nil {
		if e.trace != nil {
			e.trace.RecordOverrideCheck("project/user", askEntry)
			e.trace.RecordFinalDecision(DecisionAsk, time.Since(start))
		}
		return newOverrideDecisionResult(DecisionAsk, cmd, cleaned, askEntry, SourceOverrideAsk, nil)
	}
	if cleaned != cmd {
		if askEntry := e.checkOverrideAsk(cleaned, ""); askEntry != nil {
			if e.trace != nil {
				e.trace.RecordOverrideCheck("project/user (cleaned)", askEntry)
				e.trace.RecordFinalDecision(DecisionAsk, time.Since(start))
			}
			return newOverrideDecisionResult(DecisionAsk, cmd, cleaned, askEntry, SourceOverrideAsk, nil)
		}
	}

	// Step 3.2 — Override allow entries.
	if allowEntry := e.checkOverrideAllow(cmd, ""); allowEntry != nil {
		if e.trace != nil {
			e.trace.RecordOverrideCheck("project/user", allowEntry)
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return newOverrideDecisionResult(DecisionAllow, cmd, cleaned, allowEntry, SourceOverrideAllow, nil)
	}
	if cleaned != cmd {
		if allowEntry := e.checkOverrideAllow(cleaned, ""); allowEntry != nil {
			if e.trace != nil {
				e.trace.RecordOverrideCheck("project/user (cleaned)", allowEntry)
				e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
			}
			return newOverrideDecisionResult(DecisionAllow, cmd, cleaned, allowEntry, SourceOverrideAllow, nil)
		}
	}

	// Step 3.5 — Session entries (pre-pack).
	if entry := e.checkSessionMatch(cmd, ""); entry != nil {
		decision := DecisionAllow
		if entry.Action == "ask" {
			decision = DecisionAsk
		}
		if e.trace != nil {
			e.trace.RecordSessionAllowCheck("pre-pack", entry, decision == DecisionAllow)
			e.trace.RecordFinalDecision(decision, time.Since(start))
		}
		if decision == DecisionAsk {
			return newSessionDecisionResult(decision, cmd, cleaned, entry, nil)
		}
		return &EvaluationResult{
			Decision:          decision,
			Command:           cmd,
			NormalizedCommand: cleaned,
			SessionEntry:      entry,
		}
	}
	if cleaned != cmd {
		if entry := e.checkSessionMatch(cleaned, ""); entry != nil {
			decision := DecisionAllow
			if entry.Action == "ask" {
				decision = DecisionAsk
			}
			if e.trace != nil {
				e.trace.RecordSessionAllowCheck("pre-pack (cleaned)", entry, decision == DecisionAllow)
				e.trace.RecordFinalDecision(decision, time.Since(start))
			}
			if decision == DecisionAsk {
				return newSessionDecisionResult(decision, cmd, cleaned, entry, nil)
			}
			return &EvaluationResult{
				Decision:          decision,
				Command:           cmd,
				NormalizedCommand: cleaned,
				SessionEntry:      entry,
			}
		}
	}


	// Step 4 — Quick-reject on cleaned command.
	rejected, candidateMask := e.kwIndex.QuickReject(cleaned)
	if e.trace != nil {
		var candidateIDs []string
		for i := range e.packs {
			if isBitSet(candidateMask, i) {
				candidateIDs = append(candidateIDs, e.packs[i].ID)
			}
		}
		e.trace.RecordQuickReject(cleaned, candidateIDs)
	}
	if rejected {
		if e.trace != nil {
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return result
	}

	// Step 4.5 — Parse command through AST for structural matching.
	if e.astEnabled {
		simpleCommands = parseSimpleCommands()
	}

	result.NormalizedCommand = cleaned

	// Step 7 — Deadline check.
	select {
	case <-ctx.Done():
		if e.trace != nil {
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return &EvaluationResult{
			Decision:           DecisionAllow,
			SkippedDueToBudget: true,
			Command:            cmd,
			NormalizedCommand:  cleaned,
		}
	default:
	}

	// Step 8 — Pack evaluation (v2 structural matching only).
	for i, p := range e.packs {
		if !isBitSet(candidateMask, i) {
			if e.trace != nil {
				e.trace.RecordPackEvaluation(p.ID, true, false, nil)
			}
			continue
		}

		// Skip packs with no structural patterns or no parsed commands.
		if len(p.StructuralPatterns) == 0 || len(simpleCommands) == 0 {
			if e.trace != nil {
				e.trace.RecordPackEvaluation(p.ID, false, false, nil)
			}
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
				NormalizedCommand:  cleaned,
			}
		default:
		}

		// Structural matching on parsed AST commands.
		if dm := p.CheckStructural(simpleCommands); dm != nil {
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
			}
			if e.trace != nil {
				e.trace.RecordPackEvaluation(p.ID, false, false, match)
			}
			if result := e.postPackChecks(ctx, cmd, cleaned, ruleID, simpleCommands, match, dm, start); result != nil {
				return result
			}
		} else if e.trace != nil {
			e.trace.RecordPackEvaluation(p.ID, false, false, nil)
		}
	}

	// Step 9 — Default Allow.
	if e.trace != nil {
		e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
	}
	return result
}

func newOverrideDecisionResult(decision EvaluationDecision, command, normalizedCmd string, entry *override.Entry, source MatchSource, baseMatch *PatternMatch) *EvaluationResult {
	result := &EvaluationResult{
		Decision:          decision,
		Command:           command,
		NormalizedCommand: normalizedCmd,
		OverrideEntry:     entry,
	}
	if decision == DecisionAllow {
		return result
	}

	match := &PatternMatch{
		Reason: overrideDecisionReason(entry),
		Source: source,
	}
	if baseMatch != nil {
		match.RuleID = baseMatch.RuleID
		match.PackID = baseMatch.PackID
		match.PatternName = baseMatch.PatternName
		match.Severity = baseMatch.Severity
		match.Explanation = baseMatch.Explanation
		match.Suggestions = baseMatch.Suggestions
		match.MatchedSpan = baseMatch.MatchedSpan
	}
	result.PatternInfo = match
	return result
}

func overrideDecisionReason(entry *override.Entry) string {
	return "command matches override " + entry.Action + ": " + entry.Value
}

// EnabledPackIDs returns the list of enabled pack IDs.
func (e *Evaluator) EnabledPackIDs() []string {
	return e.packIDs
}

// postPackChecks runs the post-pack allow/override/session checks after a destructive match.
// Returns an EvaluationResult if the match should be overridden, or nil to proceed with deny/ask.
func (e *Evaluator) postPackChecks(ctx context.Context, cmd, normalizedCmd, ruleID string, simpleCommands []ast.SimpleCommand, match *PatternMatch, dm *packs.DestructiveMatch, start time.Time) *EvaluationResult {
	_ = ctx // reserved for future deadline checks

	// Post-pack: check rule allow by rule-ID first.
	if ra := e.checkRuleAllow(cmd, ruleID, simpleCommands); ra != nil {
		if e.trace != nil {
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return &EvaluationResult{
			Decision:          DecisionAllow,
			Command:           cmd,
			RuleEntry:         ra,
			NormalizedCommand: normalizedCmd,
		}
	}
	if normalizedCmd != cmd {
		if ra := e.checkRuleAllow(normalizedCmd, ruleID, simpleCommands); ra != nil {
			if e.trace != nil {
				e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
			}
			return &EvaluationResult{
				Decision:          DecisionAllow,
				Command:           cmd,
				RuleEntry:         ra,
				NormalizedCommand: normalizedCmd,
			}
		}
	}
	// Post-pack override checks.
	if ovEntry := e.checkOverrideAsk(cmd, ruleID); ovEntry != nil {
		if e.trace != nil {
			e.trace.RecordOverrideCheck("ask-after-match", ovEntry)
			e.trace.RecordFinalDecision(DecisionAsk, time.Since(start))
		}
		return newOverrideDecisionResult(DecisionAsk, cmd, normalizedCmd, ovEntry, SourceOverrideAsk, match)
	}
	if normalizedCmd != cmd {
		if ovEntry := e.checkOverrideAsk(normalizedCmd, ruleID); ovEntry != nil {
			if e.trace != nil {
				e.trace.RecordOverrideCheck("ask-after-match (cleaned)", ovEntry)
				e.trace.RecordFinalDecision(DecisionAsk, time.Since(start))
			}
			return newOverrideDecisionResult(DecisionAsk, cmd, normalizedCmd, ovEntry, SourceOverrideAsk, match)
		}
	}
	if ovEntry := e.checkOverrideAllow(cmd, ruleID); ovEntry != nil {
		if e.trace != nil {
			e.trace.RecordOverrideCheck("allow-after-match", ovEntry)
			e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
		}
		return newOverrideDecisionResult(DecisionAllow, cmd, normalizedCmd, ovEntry, SourceOverrideAllow, match)
	}
	if normalizedCmd != cmd {
		if ovEntry := e.checkOverrideAllow(normalizedCmd, ruleID); ovEntry != nil {
			if e.trace != nil {
				e.trace.RecordOverrideCheck("allow-after-match (cleaned)", ovEntry)
				e.trace.RecordFinalDecision(DecisionAllow, time.Since(start))
			}
			return newOverrideDecisionResult(DecisionAllow, cmd, normalizedCmd, ovEntry, SourceOverrideAllow, match)
		}
	}
	// Check session entries.
	if saEntry := e.checkSessionMatch(cmd, ruleID); saEntry != nil {
		decision := DecisionAllow
		if saEntry.Action == "ask" {
			decision = DecisionAsk
		}
		if e.trace != nil {
			e.trace.RecordSessionAllowCheck("post-match", saEntry, decision == DecisionAllow)
			e.trace.RecordFinalDecision(decision, time.Since(start))
		}
		if decision == DecisionAsk {
			return newSessionDecisionResult(decision, cmd, normalizedCmd, saEntry, match)
		}
		return &EvaluationResult{
			Decision:          decision,
			Command:           cmd,
			SessionEntry:      saEntry,
			NormalizedCommand: normalizedCmd,
		}
	}
	if normalizedCmd != cmd {
		if saEntry := e.checkSessionMatch(normalizedCmd, ruleID); saEntry != nil {
			decision := DecisionAllow
			if saEntry.Action == "ask" {
				decision = DecisionAsk
			}
			if e.trace != nil {
				e.trace.RecordSessionAllowCheck("post-match (cleaned)", saEntry, decision == DecisionAllow)
				e.trace.RecordFinalDecision(decision, time.Since(start))
			}
			if decision == DecisionAsk {
				return newSessionDecisionResult(decision, cmd, normalizedCmd, saEntry, match)
			}
			return &EvaluationResult{
				Decision:          decision,
				Command:           cmd,
				SessionEntry:      saEntry,
				NormalizedCommand: normalizedCmd,
			}
		}
	}
	// No override — return deny/ask.
	decision := DecisionDeny
	if dm.Action == "ask" {
		decision = DecisionAsk
	}
	if e.trace != nil {
		e.trace.RecordFinalDecision(decision, time.Since(start))
	}
	return &EvaluationResult{
		Decision:          decision,
		Command:           cmd,
		PatternInfo:       match,
		NormalizedCommand: normalizedCmd,
	}
}

func newSessionDecisionResult(decision EvaluationDecision, command, normalizedCmd string, entry *session.Entry, baseMatch *PatternMatch) *EvaluationResult {
	result := &EvaluationResult{
		Decision:          decision,
		Command:           command,
		NormalizedCommand: normalizedCmd,
		SessionEntry:      entry,
	}
	if decision == DecisionAllow {
		return result
	}

	match := &PatternMatch{
		Reason: "command matches session " + entry.Action + ": " + entry.Value,
		Source: SourceSessionAllow, // reuse SourceSessionAllow but decision is Ask
	}
	if baseMatch != nil {
		match.RuleID = baseMatch.RuleID
		match.PackID = baseMatch.PackID
		match.PatternName = baseMatch.PatternName
		match.Severity = baseMatch.Severity
		match.Explanation = baseMatch.Explanation
		match.Suggestions = baseMatch.Suggestions
		match.MatchedSpan = baseMatch.MatchedSpan
	}
	result.PatternInfo = match
	return result
}


func (e *Evaluator) resetRuleState() {
	e.packs = append(e.packs[:0], e.basePacks...)
	e.packIDs = append(e.packIDs[:0], e.basePackIDs...)
	e.kwIndex = e.rebuildKeywordIndex()
	e.ruleAllows = nil
	e.hasCommandAllowRules = false
}

func (e *Evaluator) rebuildKeywordIndex() *EnabledKeywordIndex {
	kwEntries := make([]PackKeywords, 0, len(e.packs))
	for i, p := range e.packs {
		kwEntries = append(kwEntries, PackKeywords{
			PackIndex: i,
			Keywords:  p.Keywords,
		})
	}
	return NewEnabledKeywordIndex(kwEntries)
}

func isBitSet(mask packMask, i int) bool {
	return mask.isSet(i)
}
