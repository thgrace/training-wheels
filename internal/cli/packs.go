package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/packs"
)

var packsJSON bool

var packsCmd = &cobra.Command{
	Use:   "packs",
	Short: "List available pattern packs",
	RunE:  runPacks,
}

func init() {
	bindJSONOutputFlags(packsCmd.PersistentFlags(), &packsJSON)
}

func runPacks(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return exitErrorf(exitcodes.ConfigError, "config error: %w", err)
	}

	reg := packs.DefaultRegistry()

	// Determine enabled set.
	_, enabledSet := reg.ResolveEnabledSet(cfg.Packs.Enabled, cfg.Packs.Disabled)

	allIDs := reg.AllIDs()

	switch {
	case useJSONOutput(packsJSON):
		printPacksJSON(cmd.OutOrStdout(), reg, allIDs, enabledSet)
	default:
		printPacksPretty(cmd.OutOrStdout(), reg, allIDs, enabledSet)
	}

	return nil
}

type packJSONOutput struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Enabled      bool     `json:"enabled"`
	Source       string   `json:"source"`
	Keywords     []string `json:"keywords"`
	PatternCount int      `json:"pattern_count"`
	Patterns     []string `json:"patterns,omitempty"`
	SafePatterns []string `json:"safe_patterns,omitempty"`
}

func printPacksJSON(w io.Writer, reg *packs.PackRegistry, ids []string, enabledSet map[string]bool) {
	var out []packJSONOutput
	for _, id := range ids {
		p := reg.Get(id)
		if p == nil {
			continue
		}
		source := "disabled"
		if enabledSet[id] {
			source = "enabled"
		}
		out = append(out, packJSONOutput{
			ID:           id,
			Name:         p.Name,
			Description:  p.Description,
			Enabled:      enabledSet[id],
			Source:       source,
			Keywords:     p.Keywords,
			PatternCount: len(p.StructuralPatterns),
			Patterns:     patternNames(p),
			SafePatterns: safePatternNames(p),
		})
	}
	_ = writeJSONOutput(w, out)
}

func printPacksPretty(w io.Writer, reg *packs.PackRegistry, ids []string, enabledSet map[string]bool) {
	fmt.Fprintf(w, "%-35s %-10s %-10s %s\n", "PACK ID", "STATUS", "SOURCE", "KEYWORDS")
	for _, id := range ids {
		p := reg.Get(id)
		if p == nil {
			continue
		}
		status := "disabled"
		source := "config"
		if enabledSet[id] {
			status = "enabled"
		} else {
			source = "-"
		}
		kws := strings.Join(p.Keywords, ", ")
		fmt.Fprintf(w, "%-35s %-10s %-10s %s\n", id, status, source, kws)
	}
}

func safePatternNames(p *packs.Pack) []string {
	if p == nil {
		return nil
	}
	var names []string
	for _, sp := range p.StructuralPatterns {
		if !sp.Unless.IsEmpty() {
			names = append(names, sp.Name)
		}
	}
	return names
}

func patternNames(p *packs.Pack) []string {
	if p == nil {
		return nil
	}
	names := make([]string, 0, len(p.StructuralPatterns))
	for _, pattern := range p.StructuralPatterns {
		names = append(names, pattern.Name)
	}
	return names
}
