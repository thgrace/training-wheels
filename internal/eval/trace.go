package eval

import (
	"time"

	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/session"
)

// TraceCollector defines the interface for recording evaluation steps.
type TraceCollector interface {
	RecordPreNormalize(input, output string)
	RecordOverrideCheck(scope string, entry *override.Entry)
	RecordSessionAllowCheck(scope string, entry *session.Entry, allowed bool)
	RecordQuickReject(input string, candidatePackIDs []string)
	RecordSanitization(input, output string)
	RecordNormalization(input, output string)
	RecordPackEvaluation(packID string, skipped bool, safeMatched bool, destructiveMatch *PatternMatch)
	RecordFinalDecision(decision EvaluationDecision, duration time.Duration)

	// GetTrace returns the accumulated trace.
	GetTrace() *EvaluationTrace
}

// EvaluationTrace holds the full history of a single evaluation.
type EvaluationTrace struct {
	Input            string            `json:"input"`
	PreNormalized    string            `json:"pre_normalized"`
	Sanitized        string            `json:"sanitized"`
	Normalized       string            `json:"normalized"`
	QuickRejectPacks []string          `json:"quick_reject_packs"`
	Steps            []TraceStep       `json:"steps"`
	PackDetails      []PackTraceDetail `json:"pack_details"`
	FinalDecision    string            `json:"final_decision"`
	TotalDurationNs  int64             `json:"total_duration_ns"`
}

// TraceStep records a high-level event in the evaluation.
type TraceStep struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Details   any       `json:"details,omitempty"`
}

// PackTraceDetail records the evaluation result for a specific pack.
type PackTraceDetail struct {
	PackID           string        `json:"pack_id"`
	Status           string        `json:"status"` // "triggered", "skipped", "safe_match", "destructive_match"
	DestructiveMatch *PatternMatch `json:"destructive_match,omitempty"`
}

// BufferedTraceCollector is a concrete implementation that stores steps in memory.
type BufferedTraceCollector struct {
	trace     *EvaluationTrace
	startTime time.Time
}

func NewBufferedTraceCollector(input string) *BufferedTraceCollector {
	return &BufferedTraceCollector{
		trace: &EvaluationTrace{
			Input: input,
			Steps: []TraceStep{},
		},
		startTime: time.Now(),
	}
}

func (c *BufferedTraceCollector) addStep(msg string, details any) {
	c.trace.Steps = append(c.trace.Steps, TraceStep{
		Timestamp: time.Now(),
		Message:   msg,
		Details:   details,
	})
}

func (c *BufferedTraceCollector) RecordPreNormalize(input, output string) {
	c.trace.PreNormalized = output
	c.addStep("Pre-normalization complete", map[string]string{"output": output})
}

func (c *BufferedTraceCollector) RecordOverrideCheck(scope string, entry *override.Entry) {
	msg := "Override check: " + scope
	if entry != nil {
		c.addStep(msg, map[string]any{
			"match":  true,
			"action": entry.Action,
			"id":     entry.ID,
			"value":  entry.Value,
		})
	} else {
		c.addStep(msg, map[string]any{"match": false})
	}
}

func (c *BufferedTraceCollector) RecordSessionAllowCheck(scope string, entry *session.Entry, allowed bool) {
	msg := "Session allow check: " + scope
	if entry != nil {
		action := "deny"
		if allowed {
			action = "allow"
		}
		c.addStep(msg, map[string]any{
			"match":  true,
			"action": action,
			"id":     entry.ID,
			"value":  entry.Value,
		})
	} else {
		c.addStep(msg, map[string]any{"match": false})
	}
}

func (c *BufferedTraceCollector) RecordQuickReject(input string, candidatePackIDs []string) {
	c.trace.QuickRejectPacks = candidatePackIDs
	c.addStep("Quick-reject filter applied", map[string]any{
		"candidate_count": len(candidatePackIDs),
		"candidates":      candidatePackIDs,
	})
}

func (c *BufferedTraceCollector) RecordSanitization(input, output string) {
	c.trace.Sanitized = output
	c.addStep("Sanitization complete", map[string]string{"output": output})
}

func (c *BufferedTraceCollector) RecordNormalization(input, output string) {
	c.trace.Normalized = output
	c.addStep("Full normalization complete", map[string]string{"output": output})
}

func (c *BufferedTraceCollector) RecordPackEvaluation(packID string, skipped bool, safeMatched bool, destructiveMatch *PatternMatch) {
	var status string
	switch {
	case skipped:
		status = "skipped (no keyword match)"
	case safeMatched:
		status = "safe match"
	case destructiveMatch != nil:
		status = "destructive match"
	default:
		status = "scanned"
	}

	c.trace.PackDetails = append(c.trace.PackDetails, PackTraceDetail{
		PackID:           packID,
		Status:           status,
		DestructiveMatch: destructiveMatch,
	})
}

func (c *BufferedTraceCollector) RecordFinalDecision(decision EvaluationDecision, duration time.Duration) {
	c.trace.FinalDecision = decision.String()
	c.trace.TotalDurationNs = duration.Nanoseconds()
	c.addStep("Final decision reached", map[string]string{"decision": decision.String()})
}

func (c *BufferedTraceCollector) GetTrace() *EvaluationTrace {
	return c.trace
}
