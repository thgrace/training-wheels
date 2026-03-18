package skills

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/thgrace/training-wheels/internal/osutil"
	skillassets "github.com/thgrace/training-wheels/skills"
)

const (
	// SkillRelDir is the cross-client skill directory relative to home.
	SkillRelDir = ".agents/skills/training-wheels"

	// ClaudeSkillRelDir is the Claude-specific skill directory relative to home.
	ClaudeSkillRelDir = ".claude/skills/training-wheels"

	// SkillFileName is the skill file name per agentskills.io spec.
	SkillFileName = "SKILL.md"
)

// InstallTo writes the embedded SKILL.md to baseDir/SKILL.md.
// It creates the directory if needed and uses an atomic write.
func InstallTo(baseDir string) error {
	content, err := skillassets.SkillContent()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	dst := filepath.Join(baseDir, SkillFileName)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return osutil.AtomicRename(tmp, dst)
}

// UninstallFrom removes the SKILL.md and its parent directory if empty.
func UninstallFrom(baseDir string) error {
	dst := filepath.Join(baseDir, SkillFileName)
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Remove the training-wheels directory if now empty.
	_ = os.Remove(baseDir)
	return nil
}

// ExistsAt returns true if the skill file exists at baseDir and its content
// matches the embedded version.
func ExistsAt(baseDir string) bool {
	dst := filepath.Join(baseDir, SkillFileName)
	data, err := os.ReadFile(dst)
	if err != nil {
		return false
	}
	embedded, err := skillassets.SkillContent()
	if err != nil {
		return false
	}
	return bytes.Equal(data, embedded)
}
