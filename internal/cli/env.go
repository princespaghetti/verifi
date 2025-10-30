package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/princespaghetti/verifi/internal/certstore"
	verifierrors "github.com/princespaghetti/verifi/internal/errors"
	"github.com/princespaghetti/verifi/internal/shell"
)

// envCmd represents the env command.
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Generate or regenerate the environment configuration file",
	Long: `Generate or regenerate the env.sh file for shell environment configuration.

This command creates ~/.verifi/env.sh with environment variables pointing to
the combined certificate bundle. Use this if you need to regenerate the file
or if it was accidentally deleted.

The env.sh file sets the following environment variables:
  - SSL_CERT_FILE (Python, Ruby, Go, curl, wget)
  - REQUESTS_CA_BUNDLE (Python requests)
  - NODE_EXTRA_CA_CERTS (Node.js, npm, yarn, pnpm)
  - CURL_CA_BUNDLE (curl, libcurl)
  - AWS_CA_BUNDLE (AWS CLI, boto3)
  - GIT_SSL_CAINFO (git)

To activate, add this to your shell config:
  source ~/.verifi/env.sh`,
	RunE: runEnv,
}

func init() {
	rootCmd.AddCommand(envCmd)
}

func runEnv(cmd *cobra.Command, args []string) error {
	// Create store
	store, err := certstore.NewStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create store: %v\n", err)
		os.Exit(verifierrors.ExitConfigError)
	}

	// Check if initialized
	if !store.IsInitialized() {
		fmt.Fprintf(os.Stderr, "Error: Certificate store not initialized\n")
		fmt.Fprintf(os.Stderr, "Run 'verifi init' first to initialize the store\n")
		os.Exit(verifierrors.ExitConfigError)
	}

	// Generate env.sh
	envPath := shell.EnvFilePath(store.BasePath())
	if err := shell.GenerateEnvFile(store.BasePath(), store.CombinedBundlePath()); err != nil {
		Error("Failed to generate env.sh: %v", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	Success("Environment file regenerated: %s", envPath)

	// Print setup instructions
	shell.PrintSetupInstructions(envPath)

	return nil
}
