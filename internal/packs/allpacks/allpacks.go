// Package allpacks loads built-in pattern packs.
package allpacks

import (
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/packs"
	packassets "github.com/thgrace/training-wheels/packs"
)

// RegisterAll loads all built-in packs from the embedded JSON artifacts.
func RegisterAll(r *packs.PackRegistry) {
	if err := r.LoadFromEmbed(packassets.Files, packassets.BuiltinJSONPattern); err != nil {
		logger.Error("builtin pack load completed with errors", "error", err)
	}
}
