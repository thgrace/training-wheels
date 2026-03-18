package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestOutputDenial(t *testing.T) {
	tests := []struct {
		name     string
		protocol HookProtocol
		reason   string
		contains []string
	}{
		{
			name:     "Claude denial",
			protocol: ProtocolClaude,
			reason:   "dangerous",
			contains: []string{"permissionDecision", "deny", "dangerous"},
		},
		{
			name:     "Copilot denial",
			protocol: ProtocolCopilot,
			reason:   "dangerous",
			contains: []string{"continue", "false", "stopReason", "dangerous"},
		},
		{
			name:     "Gemini denial",
			protocol: ProtocolGemini,
			reason:   "dangerous",
			contains: []string{"decision", "deny", "dangerous"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := OutputDenial(&buf, tt.protocol, tt.reason, "rule-1", "pack-1", "high")
			if err != nil {
				t.Fatalf("OutputDenial error: %v", err)
			}

			got := buf.String()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("output %q missing %q", got, want)
				}
			}
		})
	}
}

func TestOutputAsk(t *testing.T) {
	tests := []struct {
		name     string
		protocol HookProtocol
		reason   string
		contains []string
	}{
		{
			name:     "Claude ask",
			protocol: ProtocolClaude,
			reason:   "verify me",
			contains: []string{"permissionDecision", "ask", "verify me"},
		},
		{
			name:     "Copilot ask fallback to block",
			protocol: ProtocolCopilot,
			reason:   "verify me",
			contains: []string{"continue", "false", "stopReason", "verify me"},
		},
		{
			name:     "Gemini ask",
			protocol: ProtocolGemini,
			reason:   "verify me",
			contains: []string{"decision", "ask", "verify me"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := OutputAsk(&buf, tt.protocol, tt.reason, "rule-1", "pack-1", "high")
			if err != nil {
				t.Fatalf("OutputAsk error: %v", err)
			}

			got := buf.String()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("output %q missing %q", got, want)
				}
			}
		})
	}
}

func TestCopilotOutputFormat(t *testing.T) {
	var buf bytes.Buffer
	err := OutputDenial(&buf, ProtocolCopilot, "reason-text", "rule-id", "pack-id", "critical")
	if err != nil {
		t.Fatal(err)
	}

	var output CopilotHookOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatal(err)
	}

	if output.Continue != false {
		t.Errorf("got continue=%v, want false", output.Continue)
	}
	if output.StopReason != "reason-text" {
		t.Errorf("got stopReason=%q, want reason-text", output.StopReason)
	}
}
