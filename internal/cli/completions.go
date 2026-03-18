package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var completionsCmd = &cobra.Command{
	Use:   "completions [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for tw.

Usage:
  tw completions bash       > ~/.bash_completion.d/tw
  tw completions zsh        > ~/.zsh/completions/_tw
  tw completions fish       > ~/.config/fish/completions/tw.fish
  tw completions powershell > tw.ps1`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Hidden:    true,
	RunE:      runCompletions,
}

func runCompletions(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "bash":
		return rootCmd.GenBashCompletion(os.Stdout)
	case "zsh":
		return rootCmd.GenZshCompletion(os.Stdout)
	case "fish":
		return rootCmd.GenFishCompletion(os.Stdout, true)
	case "powershell":
		return rootCmd.GenPowerShellCompletion(os.Stdout)
	default:
		return cmd.Usage()
	}
}
