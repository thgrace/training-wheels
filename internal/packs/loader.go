package packs

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/thgrace/training-wheels/internal/logger"
)

var categoryNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type jsonCategoryFile struct {
	Category *string     `json:"category"`
	Packs    *[]jsonPack `json:"packs"`
}

type jsonPack struct {
	ID                  *string                   `json:"id"`
	Name                *string                   `json:"name"`
	Description         *string                   `json:"description"`
	Keywords            *[]string                 `json:"keywords"`
	SafePatterns        *[]jsonSafePattern        `json:"safe_patterns"`
	DestructivePatterns *[]jsonDestructivePattern `json:"destructive_patterns"`
}

type jsonSafePattern struct {
	Name  *string `json:"name"`
	Regex *string `json:"regex"`
}

type jsonDestructivePattern struct {
	Name        *string           `json:"name"`
	Regex       *string           `json:"regex"`
	Reason      *string           `json:"reason"`
	Severity    *string           `json:"severity"`
	Explanation *string           `json:"explanation"`
	Suggestions *[]jsonSuggestion `json:"suggestions"`
}

type jsonSuggestion struct {
	Command     *string `json:"command"`
	Description *string `json:"description"`
	Platform    *string `json:"platform"`
}

// LoadFromEmbed loads pack files from an embedded filesystem glob.
func (r *PackRegistry) LoadFromEmbed(fsys embed.FS, pattern string) error {
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return fmt.Errorf("load embedded packs %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("load embedded packs %q: no matches", pattern)
	}
	sort.Strings(matches)

	var errs []error
	for _, name := range matches {
		if err := r.loadFileFromFS(fsys, name, name); err != nil {
			logger.Warn("pack file skipped", "source", name, "error", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// LoadFromDir loads every top-level JSON file in a directory. Missing
// directories are ignored so optional user/project pack roots fail open.
func (r *PackRegistry) LoadFromDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pack dir %q: %w", path, err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	var errs []error
	for _, name := range names {
		fullPath := filepath.Join(path, name)
		if err := r.LoadFile(fullPath); err != nil {
			logger.Warn("pack file skipped", "source", fullPath, "error", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// LoadFile loads one JSON pack file from disk.
func (r *PackRegistry) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read pack file %q: %w", path, err)
	}
	return r.loadFileData(data, path)
}

func (r *PackRegistry) loadFileFromFS(fsys fs.FS, name, source string) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("read pack file %q: %w", source, err)
	}
	return r.loadFileData(data, source)
}

func (r *PackRegistry) loadFileData(data []byte, source string) error {
	file, err := decodeCategoryFile(data, source)
	if err != nil {
		return err
	}

	packs, err := validateAndConvertCategoryFile(file, source)
	if err != nil {
		return err
	}

	var errs []error
	for _, pack := range packs {
		if err := r.RegisterPack(pack, source); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func decodeCategoryFile(data []byte, source string) (*jsonCategoryFile, error) {
	var file jsonCategoryFile

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode pack file %q: %w", source, err)
	}
	if err := ensureEOF(dec); err != nil {
		return nil, fmt.Errorf("decode pack file %q: %w", source, err)
	}

	return &file, nil
}

func ensureEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func validateAndConvertCategoryFile(file *jsonCategoryFile, source string) ([]*Pack, error) {
	category, err := requireNonEmptyString("category", file.Category)
	if err != nil {
		return nil, fmt.Errorf("validate pack file %q: %w", source, err)
	}
	if !categoryNamePattern.MatchString(category) {
		return nil, fmt.Errorf("validate pack file %q: category %q does not match %s", source, category, categoryNamePattern.String())
	}
	if file.Packs == nil || len(*file.Packs) == 0 {
		return nil, fmt.Errorf("validate pack file %q: packs must contain at least one pack", source)
	}

	seen := make(map[string]struct{}, len(*file.Packs))
	packs := make([]*Pack, 0, len(*file.Packs))
	for idx, rawPack := range *file.Packs {
		pack, err := convertJSONPack(rawPack, category)
		if err != nil {
			return nil, fmt.Errorf("validate pack file %q: packs[%d]: %w", source, idx, err)
		}
		if _, exists := seen[pack.ID]; exists {
			return nil, fmt.Errorf("validate pack file %q: duplicate pack id %q within file", source, pack.ID)
		}
		seen[pack.ID] = struct{}{}
		packs = append(packs, pack)
	}

	return packs, nil
}

func convertJSONPack(raw jsonPack, category string) (*Pack, error) {
	id, err := requireNonEmptyString("id", raw.ID)
	if err != nil {
		return nil, err
	}
	if id != category && !strings.HasPrefix(id, category+".") {
		return nil, fmt.Errorf("pack id %q must equal %q or start with category prefix %q", id, category, category+".")
	}

	name, err := requireNonEmptyString("name", raw.Name)
	if err != nil {
		return nil, err
	}
	description, err := requireString("description", raw.Description)
	if err != nil {
		return nil, err
	}
	if raw.Keywords == nil || len(*raw.Keywords) == 0 {
		return nil, fmt.Errorf("keywords must contain at least one item")
	}
	if raw.SafePatterns == nil {
		return nil, fmt.Errorf("safe_patterns is required")
	}
	if raw.DestructivePatterns == nil {
		return nil, fmt.Errorf("destructive_patterns is required")
	}

	pack := &Pack{
		ID:                  id,
		Name:                name,
		Description:         description,
		Keywords:            append([]string(nil), (*raw.Keywords)...),
		SafePatterns:        make([]SafePattern, 0, len(*raw.SafePatterns)),
		DestructivePatterns: make([]DestructivePattern, 0, len(*raw.DestructivePatterns)),
	}

	for i, keyword := range pack.Keywords {
		if strings.TrimSpace(keyword) == "" {
			return nil, fmt.Errorf("keywords[%d] must be non-empty", i)
		}
	}

	seenPatternNames := make(map[string]struct{}, len(*raw.SafePatterns)+len(*raw.DestructivePatterns))
	for idx, rawPattern := range *raw.SafePatterns {
		pattern, err := convertSafePattern(rawPattern)
		if err != nil {
			return nil, fmt.Errorf("safe_patterns[%d]: %w", idx, err)
		}
		if err := ensureUniquePatternName(seenPatternNames, pattern.Name); err != nil {
			return nil, err
		}
		pack.SafePatterns = append(pack.SafePatterns, pattern)
	}

	for idx, rawPattern := range *raw.DestructivePatterns {
		pattern, err := convertDestructivePattern(rawPattern)
		if err != nil {
			return nil, fmt.Errorf("destructive_patterns[%d]: %w", idx, err)
		}
		if err := ensureUniquePatternName(seenPatternNames, pattern.Name); err != nil {
			return nil, err
		}
		pack.DestructivePatterns = append(pack.DestructivePatterns, pattern)
	}

	return pack, nil
}

func convertSafePattern(raw jsonSafePattern) (SafePattern, error) {
	name, err := requireNonEmptyString("name", raw.Name)
	if err != nil {
		return SafePattern{}, err
	}
	regex, err := requireNonEmptyString("regex", raw.Regex)
	if err != nil {
		return SafePattern{}, err
	}
	if err := ValidateRegex(regex); err != nil {
		return SafePattern{}, fmt.Errorf("invalid regex %q: %w", regex, err)
	}
	return SafePattern{
		Name:  name,
		Regex: NewLazyRegex(regex),
	}, nil
}

func convertDestructivePattern(raw jsonDestructivePattern) (DestructivePattern, error) {
	name, err := requireNonEmptyString("name", raw.Name)
	if err != nil {
		return DestructivePattern{}, err
	}
	regex, err := requireNonEmptyString("regex", raw.Regex)
	if err != nil {
		return DestructivePattern{}, err
	}
	if err := ValidateRegex(regex); err != nil {
		return DestructivePattern{}, fmt.Errorf("invalid regex %q: %w", regex, err)
	}
	reason, err := requireNonEmptyString("reason", raw.Reason)
	if err != nil {
		return DestructivePattern{}, err
	}
	severityText, err := requireNonEmptyString("severity", raw.Severity)
	if err != nil {
		return DestructivePattern{}, err
	}
	severity, err := ParseSeverity(severityText)
	if err != nil {
		return DestructivePattern{}, err
	}

	pattern := DestructivePattern{
		Name:     name,
		Regex:    NewLazyRegex(regex),
		Reason:   reason,
		Severity: severity,
	}
	if raw.Explanation != nil {
		pattern.Explanation = *raw.Explanation
	}
	if raw.Suggestions != nil {
		pattern.Suggestions = make([]PatternSuggestion, 0, len(*raw.Suggestions))
		for idx, rawSuggestion := range *raw.Suggestions {
			suggestion, err := convertSuggestion(rawSuggestion)
			if err != nil {
				return DestructivePattern{}, fmt.Errorf("suggestions[%d]: %w", idx, err)
			}
			pattern.Suggestions = append(pattern.Suggestions, suggestion)
		}
	}

	return pattern, nil
}

func convertSuggestion(raw jsonSuggestion) (PatternSuggestion, error) {
	command, err := requireNonEmptyString("command", raw.Command)
	if err != nil {
		return PatternSuggestion{}, err
	}
	description, err := requireNonEmptyString("description", raw.Description)
	if err != nil {
		return PatternSuggestion{}, err
	}

	platform := PlatformAll
	if raw.Platform != nil {
		if strings.TrimSpace(*raw.Platform) == "" {
			return PatternSuggestion{}, fmt.Errorf("platform must be one of all, linux, macos, windows, bsd")
		}
		platform, err = ParsePlatform(*raw.Platform)
		if err != nil {
			return PatternSuggestion{}, err
		}
	}

	return PatternSuggestion{
		Command:     command,
		Description: description,
		Platform:    platform,
	}, nil
}

func ensureUniquePatternName(seen map[string]struct{}, name string) error {
	if _, exists := seen[name]; exists {
		return fmt.Errorf("duplicate pattern name %q", name)
	}
	seen[name] = struct{}{}
	return nil
}

func requireString(field string, value *string) (string, error) {
	if value == nil {
		return "", fmt.Errorf("%s is required", field)
	}
	return *value, nil
}

func requireNonEmptyString(field string, value *string) (string, error) {
	if value == nil {
		return "", fmt.Errorf("%s is required", field)
	}
	if strings.TrimSpace(*value) == "" {
		return "", fmt.Errorf("%s must be non-empty", field)
	}
	return *value, nil
}
