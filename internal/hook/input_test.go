package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestDetectProtocol_Claude(t *testing.T) {
	input := &HookInput{
		ToolName:  strPtr("Bash"),
		ToolInput: &ToolInput{Command: json.RawMessage(`"git status"`)},
	}
	if got := DetectProtocol(input); got != ProtocolClaude {
		t.Errorf("got %v, want ProtocolClaude", got)
	}
}

func TestDetectProtocol_Copilot(t *testing.T) {
	input := &HookInput{
		ToolArgs: json.RawMessage(`{"command":"git status"}`),
	}
	if got := DetectProtocol(input); got != ProtocolCopilot {
		t.Errorf("got %v, want ProtocolCopilot", got)
	}
}

func TestDetectProtocol_Gemini(t *testing.T) {
	input := &HookInput{
		Event:     strPtr("BeforeTool"),
		SessionID: strPtr("abc123"),
	}
	if got := DetectProtocol(input); got != ProtocolGemini {
		t.Errorf("got %v, want ProtocolGemini", got)
	}
}

func TestDetectProtocol_Fallback(t *testing.T) {
	input := &HookInput{}
	if got := DetectProtocol(input); got != ProtocolClaude {
		t.Errorf("got %v, want ProtocolClaude (fallback)", got)
	}
}

func TestExtractCommand_Claude(t *testing.T) {
	input := &HookInput{
		ToolName:  strPtr("Bash"),
		ToolInput: &ToolInput{Command: json.RawMessage(`"git reset --hard"`)},
	}
	cmd, proto, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ProtocolClaude {
		t.Errorf("protocol = %v, want Claude", proto)
	}
	if cmd != "git reset --hard" {
		t.Errorf("command = %q, want %q", cmd, "git reset --hard")
	}
}

func TestExtractCommand_Claude_ArrayCommand(t *testing.T) {
	input := &HookInput{
		ToolName:  strPtr("Bash"),
		ToolInput: &ToolInput{Command: json.RawMessage(`["git", "reset", "--hard"]`)},
	}
	cmd, _, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "git reset --hard" {
		t.Errorf("command = %q, want %q", cmd, "git reset --hard")
	}
}

func TestExtractCommand_NonShellTool(t *testing.T) {
	input := &HookInput{
		ToolName:  strPtr("Read"),
		ToolInput: &ToolInput{Command: json.RawMessage(`"/some/file"`)},
	}
	cmd, _, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "" {
		t.Errorf("command = %q, want empty (non-shell tool)", cmd)
	}
}

func TestExtractCommand_Copilot(t *testing.T) {
	input := &HookInput{
		ToolName: strPtr("launch-process"),
		ToolArgs: json.RawMessage(`{"command":"rm -rf /"}`),
	}
	cmd, proto, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ProtocolCopilot {
		t.Errorf("protocol = %v, want Copilot", proto)
	}
	if cmd != "rm -rf /" {
		t.Errorf("command = %q, want %q", cmd, "rm -rf /")
	}
}

func TestExtractCommand_Copilot_StringWrappedArgs(t *testing.T) {
	// toolArgs is a JSON string containing a JSON object.
	inner := `{"command":"git push --force"}`
	outerJSON, _ := json.Marshal(inner)
	input := &HookInput{
		ToolName: strPtr("launch-process"),
		ToolArgs: outerJSON,
	}
	cmd, _, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "git push --force" {
		t.Errorf("command = %q, want %q", cmd, "git push --force")
	}
}

func TestExtractCommand_NoToolName(t *testing.T) {
	input := &HookInput{}
	cmd, _, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "" {
		t.Errorf("command = %q, want empty", cmd)
	}
}

func TestReadHookInput_Valid(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
	input, err := ReadHookInput(strings.NewReader(payload), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if input.ToolName == nil || *input.ToolName != "Bash" {
		t.Error("expected tool_name=Bash")
	}
}

func TestReadHookInput_CamelCase(t *testing.T) {
	payload := `{"toolName":"Bash","toolInput":{"command":"git status"}}`
	input, err := ReadHookInput(strings.NewReader(payload), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if input.ToolName == nil || *input.ToolName != "Bash" {
		t.Error("expected toolName=Bash (camelCase)")
	}
	if input.ToolInput == nil {
		t.Error("expected toolInput to be parsed (camelCase)")
	}
}

func TestReadHookInput_InvalidJSON(t *testing.T) {
	_, err := ReadHookInput(strings.NewReader("not json"), 1024)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadHookInput_SizeLimit(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
	input, err := ReadHookInput(strings.NewReader(payload), 10) // too small
	if err == nil {
		t.Log("ReadHookInput with truncated input: might error or produce partial result")
		_ = input
	}
}

func TestOutputDenial_Claude(t *testing.T) {
	var buf bytes.Buffer
	err := OutputDenial(&buf, ProtocolClaude, "dangerous command", "core.git:reset-hard", "core.git", "critical")
	if err != nil {
		t.Fatal(err)
	}
	var out ClaudeHookOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("decision = %q, want deny", out.HookSpecificOutput.PermissionDecision)
	}
	if out.HookSpecificOutput.RuleID != "core.git:reset-hard" {
		t.Errorf("ruleId = %q", out.HookSpecificOutput.RuleID)
	}
}

func TestOutputDenial_Copilot(t *testing.T) {
	var buf bytes.Buffer
	err := OutputDenial(&buf, ProtocolCopilot, "dangerous", "core.git:reset-hard", "core.git", "critical")
	if err != nil {
		t.Fatal(err)
	}
	var out CopilotHookOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.Continue != false {
		t.Errorf("got continue=%v, want false", out.Continue)
	}
	if out.StopReason != "dangerous" {
		t.Errorf("got stopReason=%q, want dangerous", out.StopReason)
	}
	if out.RuleID != "core.git:reset-hard" {
		t.Errorf("ruleId = %q, want %q", out.RuleID, "core.git:reset-hard")
	}
	// Verify no "permissionDecision" or "permissionDecisionReason" fields in JSON output.
	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["permissionDecision"]; ok {
		t.Error("unexpected 'permissionDecision' field in Copilot output")
	}
	if _, ok := raw["permissionDecisionReason"]; ok {
		t.Error("unexpected 'permissionDecisionReason' field in Copilot output")
	}
}

func TestDecodeCommand_ArrayWithSpaces(t *testing.T) {
	input := &HookInput{
		ToolName:  strPtr("Bash"),
		ToolInput: &ToolInput{Command: json.RawMessage(`["rm", "-rf", "my file"]`)},
	}
	cmd, _, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "rm -rf 'my file'" {
		t.Errorf("command = %q, want %q", cmd, "rm -rf 'my file'")
	}
}

func TestDecodeCommand_ArrayWithSingleQuotes(t *testing.T) {
	input := &HookInput{
		ToolName:  strPtr("Bash"),
		ToolInput: &ToolInput{Command: json.RawMessage(`["echo", "it's here"]`)},
	}
	cmd, _, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	want := "echo 'it'\\''s here'"
	if cmd != want {
		t.Errorf("command = %q, want %q", cmd, want)
	}
}

func TestDecodeCommand_EmptyArray(t *testing.T) {
	input := &HookInput{
		ToolName:  strPtr("Bash"),
		ToolInput: &ToolInput{Command: json.RawMessage(`[]`)},
	}
	cmd, _, err := ExtractCommand(input)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "" {
		t.Errorf("command = %q, want empty", cmd)
	}
}

func TestOutputDenial_Gemini(t *testing.T) {
	var buf bytes.Buffer
	err := OutputDenial(&buf, ProtocolGemini, "dangerous", "core.git:reset-hard", "core.git", "critical")
	if err != nil {
		t.Fatal(err)
	}
	var out GeminiHookOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.Decision != "deny" {
		t.Errorf("decision = %q", out.Decision)
	}
	if !strings.Contains(out.SystemMessage, "denied") {
		t.Errorf("systemMessage = %q, want to contain 'denied'", out.SystemMessage)
	}
}
