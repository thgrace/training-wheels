package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print tw version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("tw %s %s/%s\n", Version, runtime.GOOS, runtime.GOARCH)
	},
}
