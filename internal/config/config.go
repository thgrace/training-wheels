// Package config provides layered configuration for TW.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	defaultHookTimeoutMs   = 200
	defaultMaxCommandBytes = 128 * 1024

	minHookTimeoutMs   = 50
	maxHookTimeoutMs   = 60 * 1000
	minMaxCommandBytes = 1024
	maxMaxCommandBytes = 1024 * 1024
)

// PacksConfig controls which packs are active.
type PacksConfig struct {
	Enabled       []string `json:"enabled"`
	Disabled      []string `json:"disabled"`
	Paths         []string `json:"paths"`
	DefaultAction string   `json:"default_action"` // "deny" or "ask"
	MinSeverity   string   `json:"min_severity"`   // "critical", "high", "medium", "low" — patterns below this are not enforced
}

// GeneralConfig holds process-level tunables.
type GeneralConfig struct {
	HookTimeoutMs   int `json:"hook_timeout_ms"`
	MaxCommandBytes int `json:"max_command_bytes"`
}

// UpdateConfig controls the self-update source.
type UpdateConfig struct {
	URL string `json:"url"` // GitHub releases API URL (or compatible endpoint)
}

// AllowConfig controls session allow behavior.
type AllowConfig struct {
	RequireReason bool `json:"require_reason"`
}

// Config is the root configuration object.
type Config struct {
	General GeneralConfig `json:"general"`
	Packs   PacksConfig   `json:"packs"`
	Update  UpdateConfig  `json:"update"`
	Allow   AllowConfig   `json:"allow"`
}

// DefaultConfig returns a Config with compiled defaults.
func DefaultConfig() *Config {
	enabled := []string{"core"}
	if runtime.GOOS == "windows" {
		enabled = append(enabled, "windows")
	}
	return &Config{
		General: GeneralConfig{
			HookTimeoutMs:   defaultHookTimeoutMs,
			MaxCommandBytes: defaultMaxCommandBytes,
		},
		Packs: PacksConfig{
			Enabled:       enabled,
			Disabled:      []string{},
			Paths:         []string{},
			DefaultAction: "deny",
			MinSeverity:   "low",
		},
	}
}

// JSON intermediate structs (pointer fields to distinguish "not set" from zero).

type fileGeneral struct {
	HookTimeoutMs   *int `json:"hook_timeout_ms"`
	MaxCommandBytes *int `json:"max_command_bytes"`
}

type filePacks struct {
	Enabled       []string `json:"enabled"`
	Disabled      []string `json:"disabled"`
	Paths         []string `json:"paths"`
	DefaultAction *string  `json:"default_action"`
	MinSeverity   *string  `json:"min_severity"`
}

type fileUpdate struct {
	URL *string `json:"url"`
}

type fileAllow struct {
	RequireReason *bool `json:"require_reason"`
}

type fileConfig struct {
	General *fileGeneral `json:"general"`
	Packs   *filePacks   `json:"packs"`
	Update  *fileUpdate  `json:"update"`
	Allow   *fileAllow   `json:"allow"`
}

// UserConfigPath returns the path to the user-level config file (~/.tw/config.json).
func UserConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tw", "config.json"), nil
}

// Load returns a Config populated from defaults → ~/.tw/config.json → env vars.
// Missing config files are not errors. JSON parse errors are returned.
func Load() (*Config, error) {
	cfg := DefaultConfig()
	if err := applyUserConfig(cfg); err != nil {
		return nil, err
	}
	if err := applyEnv(cfg); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadFromPath loads config with a specific JSON file path (empty means skip).
func LoadFromPath(path string) (*Config, error) {
	cfg := DefaultConfig()
	if err := applyEnv(cfg); err != nil {
		return nil, err
	}
	if path != "" {
		if err := applyJSONFile(cfg, path); err != nil {
			return nil, err
		}
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// UserPackDir returns the default user-level external pack directory (~/.tw/packs).
func UserPackDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tw", "packs"), nil
}

// FindProjectRoot walks up from the current working directory looking for the
// nearest directory containing .git.
// Returns the absolute path of the project root and true if found, or ("", false)
// if no .git directory is found before reaching the filesystem root.
func FindProjectRoot() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding a marker.
			return "", false
		}
		dir = parent
	}
}

// ProjectPackDir returns the default project-level external pack directory.
// If a project root can be found, it returns an absolute path.
// Otherwise it falls back to the relative path (fail-open for LoadFromDir).
func ProjectPackDir() string {
	if root, ok := FindProjectRoot(); ok {
		return filepath.Join(root, ".tw", "packs")
	}
	return filepath.Join(".tw", "packs")
}

// ExternalPackPaths returns the resolved search paths for external pack loading.
// The order is built-in defaults first, then any configured custom paths.
func (cfg *Config) ExternalPackPaths() ([]string, error) {
	return ResolveExternalPackPaths(cfg.Packs.Paths)
}

// ResolveExternalPackPaths combines the standard user/project pack locations
// with any configured custom files or directories, preserving order and
// removing duplicates.
func ResolveExternalPackPaths(custom []string) ([]string, error) {
	userDir, err := UserPackDir()
	if err != nil {
		return nil, err
	}

	paths := []string{
		filepath.Clean(userDir),
		filepath.Clean(ProjectPackDir()),
	}

	for _, path := range custom {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		paths = append(paths, filepath.Clean(path))
	}

	seen := make(map[string]struct{}, len(paths))
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		resolved = append(resolved, path)
	}

	return resolved, nil
}

func applyUserConfig(cfg *Config) error {
	path, err := UserConfigPath()
	if err != nil {
		return nil // can't determine home dir — skip silently
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return applyJSONFile(cfg, path)
}

func applyEnv(cfg *Config) error {
	if v := os.Getenv("TW_PACKS_ENABLED"); v != "" {
		cfg.Packs.Enabled = splitComma(v)
	}
	if v := os.Getenv("TW_PACKS_DISABLED"); v != "" {
		cfg.Packs.Disabled = splitComma(v)
	}
	if v := os.Getenv("TW_PACKS_PATHS"); v != "" {
		cfg.Packs.Paths = splitComma(v)
	}
	if v := os.Getenv("TW_DEFAULT_ACTION"); v != "" {
		cfg.Packs.DefaultAction = v
	}
	if v := os.Getenv("TW_MIN_SEVERITY"); v != "" {
		cfg.Packs.MinSeverity = v
	}
	if v := os.Getenv("TW_HOOK_TIMEOUT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("TW_HOOK_TIMEOUT_MS must be an integer: %w", err)
		}
		cfg.General.HookTimeoutMs = n
	}
	if v := os.Getenv("TW_MAX_COMMAND_BYTES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("TW_MAX_COMMAND_BYTES must be an integer: %w", err)
		}
		cfg.General.MaxCommandBytes = n
	}
	if v := os.Getenv("TW_UPDATE_URL"); v != "" {
		cfg.Update.URL = v
	}
	if v := os.Getenv("TW_ALLOW_REQUIRE_REASON"); v != "" {
		cfg.Allow.RequireReason = v == "true" || v == "1"
	}
	return nil
}

func applyJSONFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil // empty file = fail-open
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return err
	}
	if fc.General != nil {
		if fc.General.HookTimeoutMs != nil {
			cfg.General.HookTimeoutMs = *fc.General.HookTimeoutMs
		}
		if fc.General.MaxCommandBytes != nil {
			cfg.General.MaxCommandBytes = *fc.General.MaxCommandBytes
		}
	}
	if fc.Packs != nil {
		if fc.Packs.Enabled != nil {
			cfg.Packs.Enabled = fc.Packs.Enabled
		}
		if fc.Packs.Disabled != nil {
			cfg.Packs.Disabled = fc.Packs.Disabled
		}
		if fc.Packs.Paths != nil {
			cfg.Packs.Paths = fc.Packs.Paths
		}
		if fc.Packs.DefaultAction != nil {
			cfg.Packs.DefaultAction = *fc.Packs.DefaultAction
		}
		if fc.Packs.MinSeverity != nil {
			cfg.Packs.MinSeverity = *fc.Packs.MinSeverity
		}
	}
	if fc.Update != nil {
		if fc.Update.URL != nil {
			cfg.Update.URL = *fc.Update.URL
		}
	}
	if fc.Allow != nil {
		if fc.Allow.RequireReason != nil {
			cfg.Allow.RequireReason = *fc.Allow.RequireReason
		}
	}
	return nil
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func validateConfig(cfg *Config) error {
	switch {
	case cfg.General.HookTimeoutMs < minHookTimeoutMs || cfg.General.HookTimeoutMs > maxHookTimeoutMs:
		return fmt.Errorf(
			"general.hook_timeout_ms must be between %d and %d",
			minHookTimeoutMs,
			maxHookTimeoutMs,
		)
	case cfg.General.MaxCommandBytes < minMaxCommandBytes || cfg.General.MaxCommandBytes > maxMaxCommandBytes:
		return fmt.Errorf(
			"general.max_command_bytes must be between %d and %d",
			minMaxCommandBytes,
			maxMaxCommandBytes,
		)
	default:
		return nil
	}
}
