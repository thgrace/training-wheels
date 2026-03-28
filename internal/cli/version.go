package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionJSON bool

func init() {
	bindJSONOutputFlags(versionCmd.Flags(), &versionJSON)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print tw version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if useJSONOutput(versionJSON) {
			return writeJSONOutput(cmd.OutOrStdout(), struct {
				Version string `json:"version"`
				GOOS    string `json:"goos"`
				GOARCH  string `json:"goarch"`
			}{
				Version: Version,
				GOOS:    runtime.GOOS,
				GOARCH:  runtime.GOARCH,
			})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "tw %s %s/%s\n", Version, runtime.GOOS, runtime.GOARCH)
		return nil
	},
}
