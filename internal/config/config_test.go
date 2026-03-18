package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.General.HookTimeoutMs != 200 {
		t.Errorf("HookTimeoutMs = %d, want 200", cfg.General.HookTimeoutMs)
	}
	if cfg.General.MaxCommandBytes != 131072 {
		t.Errorf("MaxCommandBytes = %d, want 131072", cfg.General.MaxCommandBytes)
	}
	wantEnabled := []string{"core.git", "core.filesystem", "core.tw"}
	if runtime.GOOS == "windows" {
		wantEnabled = append(wantEnabled, "windows")
	}
	if !reflect.DeepEqual(cfg.Packs.Enabled, wantEnabled) {
		t.Errorf("Packs.Enabled = %v, want %v", cfg.Packs.Enabled, wantEnabled)
	}
	if len(cfg.Packs.Paths) != 0 {
		t.Errorf("Packs.Paths = %v, want []", cfg.Packs.Paths)
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save and restore env.
	envVars := []string{
		"TW_PACKS_ENABLED", "TW_PACKS_DISABLED", "TW_PACKS_PATHS",
		"TW_HOOK_TIMEOUT_MS", "TW_MAX_COMMAND_BYTES",
	}
	saved := make(map[string]string)
	for _, k := range envVars {
		saved[k] = os.Getenv(k)
	}
	t.Cleanup(func() {
		for _, k := range envVars {
			if saved[k] == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, saved[k])
			}
		}
	})

	os.Setenv("TW_PACKS_ENABLED", "core,database")
	os.Setenv("TW_PACKS_DISABLED", "database.postgresql")
	os.Setenv("TW_PACKS_PATHS", "/tmp/user-packs,.tw/packs, ./custom-pack.json")
	os.Setenv("TW_HOOK_TIMEOUT_MS", "500")
	os.Setenv("TW_MAX_COMMAND_BYTES", "65536")
	cfg := DefaultConfig()
	if err := applyEnv(cfg); err != nil {
		t.Fatalf("applyEnv: %v", err)
	}

	if len(cfg.Packs.Enabled) != 2 || cfg.Packs.Enabled[0] != "core" || cfg.Packs.Enabled[1] != "database" {
		t.Errorf("Packs.Enabled = %v, want [core database]", cfg.Packs.Enabled)
	}
	if len(cfg.Packs.Disabled) != 1 || cfg.Packs.Disabled[0] != "database.postgresql" {
		t.Errorf("Packs.Disabled = %v", cfg.Packs.Disabled)
	}
	if !reflect.DeepEqual(cfg.Packs.Paths, []string{"/tmp/user-packs", ".tw/packs", "./custom-pack.json"}) {
		t.Errorf("Packs.Paths = %v, want custom search paths", cfg.Packs.Paths)
	}
	if cfg.General.HookTimeoutMs != 500 {
		t.Errorf("HookTimeoutMs = %d, want 500", cfg.General.HookTimeoutMs)
	}
	if cfg.General.MaxCommandBytes != 65536 {
		t.Errorf("MaxCommandBytes = %d, want 65536", cfg.General.MaxCommandBytes)
	}
}

func TestLoadFromJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".tw.json")
	content := `{"general": {"hook_timeout_ms": 300}, "packs": {"enabled": ["core", "database"], "disabled": ["database.postgresql"], "paths": ["./team-packs", "/tmp/rules.json"]}}`
	if err := os.WriteFile(jsonPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(jsonPath)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if cfg.General.HookTimeoutMs != 300 {
		t.Errorf("HookTimeoutMs = %d, want 300", cfg.General.HookTimeoutMs)
	}
	// MaxCommandBytes should still be the default since it wasn't in JSON.
	if cfg.General.MaxCommandBytes != 131072 {
		t.Errorf("MaxCommandBytes = %d, want 131072 (default)", cfg.General.MaxCommandBytes)
	}
	if len(cfg.Packs.Enabled) != 2 {
		t.Errorf("Packs.Enabled = %v, want [core database]", cfg.Packs.Enabled)
	}
	if !reflect.DeepEqual(cfg.Packs.Paths, []string{"./team-packs", "/tmp/rules.json"}) {
		t.Errorf("Packs.Paths = %v, want configured custom paths", cfg.Packs.Paths)
	}
}

func TestLoad_InvalidEnvGeneralValues(t *testing.T) {
	tests := []struct {
		name    string
		envKey  string
		envVal  string
		wantErr string
	}{
		{
			name:    "timeout below minimum",
			envKey:  "TW_HOOK_TIMEOUT_MS",
			envVal:  "9",
			wantErr: "general.hook_timeout_ms",
		},
		{
			name:    "timeout above maximum",
			envKey:  "TW_HOOK_TIMEOUT_MS",
			envVal:  fmt.Sprintf("%d", maxHookTimeoutMs+1),
			wantErr: "general.hook_timeout_ms",
		},
		{
			name:    "timeout not an integer",
			envKey:  "TW_HOOK_TIMEOUT_MS",
			envVal:  "fast",
			wantErr: "TW_HOOK_TIMEOUT_MS",
		},
		{
			name:    "max command bytes below minimum",
			envKey:  "TW_MAX_COMMAND_BYTES",
			envVal:  "512",
			wantErr: "general.max_command_bytes",
		},
		{
			name:    "max command bytes above maximum",
			envKey:  "TW_MAX_COMMAND_BYTES",
			envVal:  fmt.Sprintf("%d", maxMaxCommandBytes+1),
			wantErr: "general.max_command_bytes",
		},
		{
			name:    "max command bytes not an integer",
			envKey:  "TW_MAX_COMMAND_BYTES",
			envVal:  "many",
			wantErr: "TW_MAX_COMMAND_BYTES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("TW_HOOK_TIMEOUT_MS", "")
			t.Setenv("TW_MAX_COMMAND_BYTES", "")
			t.Setenv(tt.envKey, tt.envVal)

			_, err := Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

// invalid JSON returns an error.
func TestLoadFromPath_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".tw.json")
	if err := os.WriteFile(jsonPath, []byte(`{this is not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromPath(jsonPath)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// empty JSON file is valid, defaults preserved.
func TestLoadFromPath_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".tw.json")
	if err := os.WriteFile(jsonPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(jsonPath)
	if err != nil {
		t.Fatalf("empty JSON should not error: %v", err)
	}
	if cfg.General.HookTimeoutMs != 200 {
		t.Errorf("HookTimeoutMs = %d, want 200 (default)", cfg.General.HookTimeoutMs)
	}
	if cfg.General.MaxCommandBytes != 131072 {
		t.Errorf("MaxCommandBytes = %d, want 131072 (default)", cfg.General.MaxCommandBytes)
	}
}

// Empty JSON object should preserve all defaults.
func TestLoadFromPath_EmptyJSONObject(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".tw.json")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(jsonPath)
	if err != nil {
		t.Fatalf("empty JSON object should not error: %v", err)
	}
	if cfg.General.HookTimeoutMs != 200 {
		t.Errorf("HookTimeoutMs = %d, want 200 (default)", cfg.General.HookTimeoutMs)
	}
	if cfg.General.MaxCommandBytes != 131072 {
		t.Errorf("MaxCommandBytes = %d, want 131072 (default)", cfg.General.MaxCommandBytes)
	}
	wantEnabled := []string{"core.git", "core.filesystem", "core.tw"}
	if runtime.GOOS == "windows" {
		wantEnabled = append(wantEnabled, "windows")
	}
	if !reflect.DeepEqual(cfg.Packs.Enabled, wantEnabled) {
		t.Errorf("Packs.Enabled = %v, want %v (default)", cfg.Packs.Enabled, wantEnabled)
	}
	if len(cfg.Packs.Paths) != 0 {
		t.Errorf("Packs.Paths = %v, want [] (default)", cfg.Packs.Paths)
	}
}

// unknown fields in JSON are silently ignored.
func TestLoadFromPath_UnknownFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".tw.json")
	content := `{"general": {"hook_timeout_ms": 300}, "unknown_section": {"foo": "bar"}}`
	if err := os.WriteFile(jsonPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(jsonPath)
	if err != nil {
		t.Fatalf("unknown fields should not cause error: %v", err)
	}
	if cfg.General.HookTimeoutMs != 300 {
		t.Errorf("HookTimeoutMs = %d, want 300", cfg.General.HookTimeoutMs)
	}
}

func TestLoadFromPath_InvalidGeneralValues(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "timeout below minimum",
			content: `{"general":{"hook_timeout_ms":9}}`,
			wantErr: "general.hook_timeout_ms",
		},
		{
			name:    "timeout above maximum",
			content: fmt.Sprintf(`{"general":{"hook_timeout_ms":%d}}`, maxHookTimeoutMs+1),
			wantErr: "general.hook_timeout_ms",
		},
		{
			name:    "max command bytes below minimum",
			content: `{"general":{"max_command_bytes":512}}`,
			wantErr: "general.max_command_bytes",
		},
		{
			name:    "max command bytes above maximum",
			content: fmt.Sprintf(`{"general":{"max_command_bytes":%d}}`, maxMaxCommandBytes+1),
			wantErr: "general.max_command_bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			jsonPath := filepath.Join(dir, ".tw.json")
			if err := os.WriteFile(jsonPath, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := LoadFromPath(jsonPath)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

// config_toggles_e2e.rs — update URL from JSON.
func TestLoadFromJSON_UpdateURL(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".tw.json")
	content := `{"update": {"url": "https://custom.example.com/releases"}}`
	if err := os.WriteFile(jsonPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Update.URL != "https://custom.example.com/releases" {
		t.Errorf("Update.URL = %q, want custom URL", cfg.Update.URL)
	}
}

// config_toggles_e2e.rs — partial JSON preserves defaults for unset fields.
func TestLoadFromJSON_PartialPreservesDefaults(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".tw.json")
	content := `{"general": {"hook_timeout_ms": 150}}`
	if err := os.WriteFile(jsonPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.General.HookTimeoutMs != 150 {
		t.Errorf("HookTimeoutMs = %d, want 150", cfg.General.HookTimeoutMs)
	}
	if cfg.General.MaxCommandBytes != 131072 {
		t.Errorf("MaxCommandBytes = %d, want 131072 (default preserved)", cfg.General.MaxCommandBytes)
	}
	wantEnabled := []string{"core.git", "core.filesystem", "core.tw"}
	if runtime.GOOS == "windows" {
		wantEnabled = append(wantEnabled, "windows")
	}
	if !reflect.DeepEqual(cfg.Packs.Enabled, wantEnabled) {
		t.Errorf("Packs.Enabled = %v, want %v (default preserved)", cfg.Packs.Enabled, wantEnabled)
	}
	if len(cfg.Packs.Paths) != 0 {
		t.Errorf("Packs.Paths = %v, want [] (default preserved)", cfg.Packs.Paths)
	}
}

func TestExternalPackPaths(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))

	cfg := DefaultConfig()
	cfg.Packs.Paths = []string{
		".tw/packs",
		"./team-packs",
		"./team-packs",
		"/tmp/extra-pack.json",
	}

	paths, err := cfg.ExternalPackPaths()
	if err != nil {
		t.Fatalf("ExternalPackPaths: %v", err)
	}

	userDir, err := UserPackDir()
	if err != nil {
		t.Fatalf("UserPackDir: %v", err)
	}

	projectDir := ProjectPackDir()
	want := []string{
		filepath.Clean(userDir),
		filepath.Clean(projectDir),
		filepath.Join(".tw", "packs"),
		"team-packs",
		filepath.Clean("/tmp/extra-pack.json"),
	}
	// If ProjectPackDir resolved to an absolute path equal to the cleaned
	// custom ".tw/packs", dedup removes the duplicate and we get one fewer entry.
	if filepath.Clean(projectDir) == filepath.Join(".tw", "packs") {
		want = []string{
			filepath.Clean(userDir),
			filepath.Join(".tw", "packs"),
			"team-packs",
			filepath.Clean("/tmp/extra-pack.json"),
		}
	}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("ExternalPackPaths = %v, want %v", paths, want)
	}
}

func TestDefaultConfig_WindowsPackConditional(t *testing.T) {
	cfg := DefaultConfig()
	isWindows := runtime.GOOS == "windows"

	hasWindows := false
	for _, p := range cfg.Packs.Enabled {
		if p == "windows" {
			hasWindows = true
			break
		}
	}

	if isWindows && !hasWindows {
		t.Error("on Windows, 'windows' pack should be in default enabled list")
	}
	if !isWindows && hasWindows {
		t.Error("on non-Windows, 'windows' pack should NOT be in default enabled list")
	}

	wantCoreDefaults := []string{"core.git", "core.filesystem", "core.tw"}
	for _, want := range wantCoreDefaults {
		hasPack := false
		for _, p := range cfg.Packs.Enabled {
			if p == want {
				hasPack = true
				break
			}
		}
		if !hasPack {
			t.Errorf("%q must always be in default enabled list", want)
		}
	}
}
