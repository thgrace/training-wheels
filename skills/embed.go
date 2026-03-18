package skillassets

import "embed"

// FS contains the embedded skill assets.
//
//go:embed training-wheels/SKILL.md
var FS embed.FS

// SkillContent returns the raw SKILL.md bytes.
func SkillContent() ([]byte, error) {
	return FS.ReadFile("training-wheels/SKILL.md")
}
