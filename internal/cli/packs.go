package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/exitcodes"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/packs"
)

var packsFormat string

var packsCmd = &cobra.Command{
	Use:   "packs",
	Short: "List available pattern packs",
	RunE:  runPacks,
}

func init() {
	packsCmd.Flags().StringVar(&packsFormat, "format", "pretty", "Output format: pretty or json")
}

func runPacks(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(exitcodes.ConfigError)
	}

	reg := packs.DefaultRegistry()

	// Determine enabled set.
	_, enabledSet := reg.ResolveEnabledSet(cfg.Packs.Enabled, cfg.Packs.Disabled)

	allIDs := reg.AllIDs()

	switch packsFormat {
	case "json":
		printPacksJSON(cmd.OutOrStdout(), reg, allIDs, enabledSet)
	default:
		printPacksPretty(cmd.OutOrStdout(), reg, allIDs, enabledSet)
	}

	return nil
}

type packJSONOutput struct {
	ID               string   `json:"id"`
	Enabled          bool     `json:"enabled"`
	Keywords         []string `json:"keywords"`
	PatternCount     int      `json:"pattern_count"`
	SafePatternCount int      `json:"safe_pattern_count"`
}

func printPacksJSON(w io.Writer, reg *packs.PackRegistry, ids []string, enabledSet map[string]bool) {
	var out []packJSONOutput
	for _, id := range ids {
		p := reg.Get(id)
		if p == nil {
			continue
		}
		out = append(out, packJSONOutput{
			ID:               id,
			Enabled:          enabledSet[id],
			Keywords:         p.Keywords,
			PatternCount:     len(p.DestructivePatterns),
			SafePatternCount: len(p.SafePatterns),
		})
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(w, string(data))
}

func printPacksPretty(w io.Writer, reg *packs.PackRegistry, ids []string, enabledSet map[string]bool) {
	fmt.Fprintf(w, "%-35s %-10s %s\n", "PACK ID", "STATUS", "KEYWORDS")
	for _, id := range ids {
		p := reg.Get(id)
		if p == nil {
			continue
		}
		status := "disabled"
		if enabledSet[id] {
			status = "enabled"
		}
		kws := strings.Join(p.Keywords, ", ")
		fmt.Fprintf(w, "%-35s %-10s %s\n", id, status, kws)
	}
}
