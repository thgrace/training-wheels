package skillassets

import "embed"

// FS contains the embedded skill assets.
//
//go:embed training-wheels/SKILL.md compose-rule/SKILL.md
var FS embed.FS

// SkillContent returns the training-wheels SKILL.md bytes.
func SkillContent() ([]byte, error) {
	return FS.ReadFile("training-wheels/SKILL.md")
}

// ComposeRuleSkillContent returns the compose-rule SKILL.md bytes.
func ComposeRuleSkillContent() ([]byte, error) {
	return FS.ReadFile("compose-rule/SKILL.md")
}
