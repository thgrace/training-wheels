// Package hook handles parsing hook input from AI coding agents and writing
// protocol-specific denial responses.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// HookProtocol identifies which agent sent the hook payload.
type HookProtocol int

const (
	ProtocolUnknown HookProtocol = iota
	ProtocolClaude
	ProtocolCopilot
	ProtocolGemini
)

func (p HookProtocol) String() string {
	switch p {
	case ProtocolClaude:
		return "claude"
	case ProtocolCopilot:
		return "copilot"
	case ProtocolGemini:
		return "gemini"
	default:
		return "unknown"
	}
}

// HookInput is the union of all fields used by supported hook protocols.
// Claude Code and Gemini send snake_case fields; Copilot uses toolArgs.
// We use a custom UnmarshalJSON to accept both snake_case and camelCase
// for tool_name, tool_input, tool_args, and hook_event_name.
type HookInput struct {
	ToolName       *string         `json:"tool_name"`
	ToolInput      *ToolInput      `json:"tool_input"`
	ToolArgs       json.RawMessage `json:"tool_args"`
	HookEventName  *string         `json:"hook_event_name"`
	Event          *string         `json:"event"`
	SessionID      *string         `json:"session_id"`
	TranscriptPath *string         `json:"transcript_path"`
	CWD            *string         `json:"cwd"`
	Timestamp      *string         `json:"timestamp"`
}

// UnmarshalJSON accepts both snake_case and camelCase for fields that
// vary across protocols (tool_name/toolName, tool_input/toolInput, etc.).
func (h *HookInput) UnmarshalJSON(data []byte) error {
	// Use a raw map to check both casings.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Helper: try key1 first, fall back to key2.
	pick := func(key1, key2 string) json.RawMessage {
		if v, ok := raw[key1]; ok {
			return v
		}
		if v, ok := raw[key2]; ok {
			return v
		}
		return nil
	}

	// unmarshalStringPtr unmarshals a JSON value into a string pointer.
	// Returns nil if v is nil or not a valid JSON string.
	unmarshalStringPtr := func(v json.RawMessage) *string {
		if v == nil {
			return nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil
		}
		return &s
	}

	// tool_name / toolName
	h.ToolName = unmarshalStringPtr(pick("tool_name", "toolName"))

	// tool_input / toolInput
	if v := pick("tool_input", "toolInput"); v != nil {
		var ti ToolInput
		if err := json.Unmarshal(v, &ti); err == nil {
			h.ToolInput = &ti
		}
	}

	// tool_args / toolArgs
	if v := pick("tool_args", "toolArgs"); v != nil {
		h.ToolArgs = v
	}

	// hook_event_name / hookEventName
	h.HookEventName = unmarshalStringPtr(pick("hook_event_name", "hookEventName"))

	// event, session_id, transcript_path, cwd, timestamp
	h.Event = unmarshalStringPtr(raw["event"])
	h.SessionID = unmarshalStringPtr(raw["session_id"])
	h.TranscriptPath = unmarshalStringPtr(raw["transcript_path"])
	h.CWD = unmarshalStringPtr(raw["cwd"])
	h.Timestamp = unmarshalStringPtr(raw["timestamp"])

	return nil
}

// ToolInput holds the tool-specific parameters.
type ToolInput struct {
	Command json.RawMessage `json:"command"`
}

// ReadHookInput reads at most maxBytes from r, then JSON-decodes the result.
func ReadHookInput(r io.Reader, maxBytes int) (*HookInput, error) {
	limited := io.LimitReader(r, int64(maxBytes))
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading hook input: %w", err)
	}
	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return &HookInput{}, fmt.Errorf("parsing hook input: %w", err)
	}
	return &input, nil
}

// DetectProtocol infers the originating agent from the payload fields.
func DetectProtocol(input *HookInput) HookProtocol {
	if input.ToolName != nil && input.ToolInput != nil {
		return ProtocolClaude
	}
	if input.Event != nil && *input.Event == "pre-tool-use" {
		return ProtocolCopilot
	}
	if len(input.ToolArgs) > 0 {
		return ProtocolCopilot
	}
	if input.Event != nil && *input.Event == "BeforeTool" {
		return ProtocolGemini
	}
	if input.SessionID != nil {
		return ProtocolGemini
	}
	return ProtocolClaude // fallback
}

// shellToolNames is the set of tool names that represent shell command execution.
var shellToolNames = map[string]bool{
	"bash":              true,
	"launch-process":    true,
	"run_shell_command": true,
	"run-shell-command": true,
}

// ExtractCommand extracts the shell command string from a parsed HookInput.
func ExtractCommand(input *HookInput) (command string, protocol HookProtocol, err error) {
	protocol = DetectProtocol(input)

	// Check tool name.
	if input.ToolName == nil {
		// No tool name — check Copilot/Gemini paths.
		if len(input.ToolArgs) > 0 {
			return extractFromToolArgs(input.ToolArgs, protocol)
		}
		return "", protocol, nil
	}

	toolName := strings.ToLower(*input.ToolName)
	if !shellToolNames[toolName] {
		return "", protocol, nil // non-shell tool, allow
	}

	// Try toolInput.command first.
	if input.ToolInput != nil && len(input.ToolInput.Command) > 0 {
		cmd, err := decodeCommand(input.ToolInput.Command)
		if err != nil {
			return "", protocol, fmt.Errorf("decoding toolInput.command: %w", err)
		}
		return cmd, protocol, nil
	}

	// Try toolArgs (Copilot path).
	if len(input.ToolArgs) > 0 {
		return extractFromToolArgs(input.ToolArgs, protocol)
	}

	return "", protocol, nil
}

// decodeCommand decodes a command that may be a JSON string or a JSON array of strings.
func decodeCommand(raw json.RawMessage) (string, error) {
	// Try string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	// Try []string — shell-quote elements to preserve argv boundaries.
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		quoted := make([]string, len(arr))
		for i, arg := range arr {
			quoted[i] = shellQuoteArg(arg)
		}
		return strings.Join(quoted, " "), nil
	}
	return "", errors.New("command is neither a string nor an array of strings")
}

// shellMetachars is the set of characters that require quoting.
const shellMetachars = " \t\n\"'\\|&;$(){}[]<>*?~`!#"

// shellQuoteArg wraps an argument in single quotes if it contains shell
// metacharacters. Arguments without metacharacters are returned as-is.
func shellQuoteArg(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, shellMetachars) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// extractFromToolArgs handles the Copilot toolArgs path.
func extractFromToolArgs(toolArgs json.RawMessage, protocol HookProtocol) (string, HookProtocol, error) {
	// toolArgs may be a JSON object or a JSON string containing a JSON object.
	cmd, err := extractCommandFromObject(toolArgs)
	if err == nil && cmd != "" {
		return cmd, protocol, nil
	}

	// Try decoding toolArgs as a JSON string first, then parsing the result.
	var s string
	if err := json.Unmarshal(toolArgs, &s); err == nil {
		var inner json.RawMessage
		if err := json.Unmarshal([]byte(s), &inner); err == nil {
			cmd, err := extractCommandFromObject(inner)
			if err == nil {
				return cmd, protocol, nil
			}
		}
	}

	return "", protocol, nil
}

// extractCommandFromObject extracts a "command" key from a JSON object.
func extractCommandFromObject(data json.RawMessage) (string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", err
	}
	cmdRaw, ok := obj["command"]
	if !ok {
		return "", nil
	}
	return decodeCommand(cmdRaw)
}
