package cli

import (
	"os"

	"github.com/spf13/cobra"

	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

// completionCmd represents the completion command.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for bash or zsh.

To load completions:

Bash:

  $ source <(verifi completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ verifi completion bash > /etc/bash_completion.d/verifi
  # macOS:
  $ verifi completion bash > $(brew --prefix)/etc/bash_completion.d/verifi

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ verifi completion zsh > "${fpath[1]}/_verifi"

  # You will need to start a new shell for this setup to take effect.

Example usage:
  verifi completion bash > /usr/local/etc/bash_completion.d/verifi
  verifi completion zsh > ~/.zsh/completions/_verifi`,
	ValidArgs: []string{"bash", "zsh"},
	Args:      cobra.ExactValidArgs(1),
	RunE:      runCompletion,
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]

	switch shell {
	case "bash":
		if err := cmd.Root().GenBashCompletion(os.Stdout); err != nil {
			Error("Failed to generate bash completion: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
	case "zsh":
		if err := cmd.Root().GenZshCompletion(os.Stdout); err != nil {
			Error("Failed to generate zsh completion: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
	default:
		Error("Unsupported shell: %s. Supported shells: bash, zsh", shell)
		os.Exit(verifierrors.ExitConfigError)
	}

	return nil
}
