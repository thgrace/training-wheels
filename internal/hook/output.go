package hook

import (
	"encoding/json"
	"fmt"
	"io"
)

// ClaudeHookOutput is the JSON envelope for Claude Code denial responses.
type ClaudeHookOutput struct {
	HookSpecificOutput ClaudeHookSpecificOutput `json:"hookSpecificOutput"`
}

// ClaudeHookSpecificOutput is the inner payload for Claude denial responses.
type ClaudeHookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
	RuleID                   string `json:"ruleId,omitempty"`
	PackID                   string `json:"packId,omitempty"`
	Severity                 string `json:"severity,omitempty"`
}

// CopilotHookOutput is the JSON envelope for Copilot CLI responses.
// See: https://docs.github.com/en/copilot/reference/hooks-configuration
type CopilotHookOutput struct {
	Continue                 bool   `json:"continue"`
	StopReason               string `json:"stopReason,omitempty"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	RuleID                   string `json:"ruleId,omitempty"`
	PackID                   string `json:"packId,omitempty"`
	Severity                 string `json:"severity,omitempty"`
}

// GeminiHookOutput is the JSON envelope for Gemini CLI responses.
type GeminiHookOutput struct {
	Decision      string `json:"decision"`
	Reason        string `json:"reason"`
	SystemMessage string `json:"systemMessage,omitempty"`
	RuleID        string `json:"ruleId,omitempty"`
}

// OutputDenial writes the protocol-appropriate denial JSON to w.
func OutputDenial(w io.Writer, protocol HookProtocol, reason, ruleID, packID, severity string) error {
	var output interface{}

	switch protocol {
	case ProtocolCopilot:
		output = CopilotHookOutput{
			Continue:   false,
			StopReason: reason,
			RuleID:     ruleID,
			PackID:     packID,
			Severity:   severity,
		}
	case ProtocolGemini:
		sysMsg := fmt.Sprintf("TW: denied — %s", reason)
		if ruleID != "" {
			sysMsg += fmt.Sprintf("\n  Rule: %s", ruleID)
		}
		output = GeminiHookOutput{
			Decision:      "deny",
			Reason:        reason,
			SystemMessage: sysMsg,
			RuleID:        ruleID,
		}
	default: // Claude and Unknown
		output = ClaudeHookOutput{
			HookSpecificOutput: ClaudeHookSpecificOutput{
				HookEventName:            "PreToolUse",
				PermissionDecision:       "deny",
				PermissionDecisionReason: reason,
				RuleID:                   ruleID,
				PackID:                   packID,
				Severity:                 severity,
			},
		}
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshaling hook output: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// OutputAsk writes the protocol-appropriate "ask" JSON to w.
func OutputAsk(w io.Writer, protocol HookProtocol, reason, ruleID, packID, severity string) error {
	var output interface{}

	switch protocol {
	case ProtocolCopilot:
		// Copilot doesn't support "ask", so we use the requested "continue: false".
		output = CopilotHookOutput{
			Continue:   false,
			StopReason: reason,
			RuleID:     ruleID,
			PackID:     packID,
			Severity:   severity,
		}
	case ProtocolGemini:
		sysMsg := fmt.Sprintf("TW: verify — %s", reason)
		if ruleID != "" {
			sysMsg += fmt.Sprintf("\n  Rule: %s", ruleID)
		}
		output = GeminiHookOutput{
			Decision:      "ask",
			Reason:        reason,
			SystemMessage: sysMsg,
			RuleID:        ruleID,
		}
	default: // Claude and Unknown
		output = ClaudeHookOutput{
			HookSpecificOutput: ClaudeHookSpecificOutput{
				HookEventName:            "PreToolUse",
				PermissionDecision:       "ask",
				PermissionDecisionReason: reason,
				RuleID:                   ruleID,
				PackID:                   packID,
				Severity:                 severity,
			},
		}
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshaling hook output: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// FormatStderrWarning returns a plain-text warning string for stderr.
func FormatStderrWarning(reason, ruleID, packID, severity string) string {
	msg := fmt.Sprintf("TW: denied — %s", reason)
	if ruleID != "" {
		msg += fmt.Sprintf("\n  Rule:     %s", ruleID)
	}
	if packID != "" {
		msg += fmt.Sprintf("\n  Pack:     %s", packID)
	}
	if severity != "" {
		msg += fmt.Sprintf("\n  Severity: %s", severity)
	}
	return msg
}
