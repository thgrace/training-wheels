package cli

import (
	"encoding/json"
	"io"

	"github.com/spf13/pflag"
)

// bindJSONOutputFlags registers --json as the machine-readable output flag.
func bindJSONOutputFlags(flags *pflag.FlagSet, jsonTarget *bool) {
	flags.BoolVar(jsonTarget, "json", false, "Output as JSON")
}

func useJSONOutput(jsonFlag bool) bool {
	return jsonFlag
}

func writeJSONOutput(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
