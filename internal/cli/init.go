package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/princespaghetti/verifi/internal/certstore"
	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

var (
	initForce bool
)

// initCmd represents the init command.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the certificate store",
	Long: `Initialize the certificate store at ~/.verifi with the embedded Mozilla CA bundle.

This command creates the directory structure and extracts the Mozilla CA
certificate bundle that is embedded in the verifi binary. The tool works
completely offline and does not require network access.

The following structure will be created:
  ~/.verifi/
    certs/
      user/              # User-added certificates
      bundles/           # Mozilla and combined bundles
      metadata.json      # Store metadata
    logs/                # Optional logs

Use --force to reinitialize an existing store (WARNING: this will reset your configuration).`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initForce, "force", false, "Force initialization even if store already exists")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Create store
	store, err := certstore.NewStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create store: %v\n", err)
		os.Exit(verifierrors.ExitConfigError)
	}

	// Check if already initialized
	if store.IsInitialized() && !initForce {
		fmt.Fprintf(os.Stderr, "Error: Certificate store already initialized at %s\n", store.BasePath())
		fmt.Fprintf(os.Stderr, "Use --force to reinitialize (WARNING: this will reset your configuration)\n")
		os.Exit(verifierrors.ExitConfigError)
	}

	// Initialize with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("Initializing certificate store at %s...\n", store.BasePath())

	if err := store.Init(ctx, initForce); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize store: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	fmt.Printf("✓ Certificate store initialized successfully\n")
	fmt.Printf("✓ Mozilla CA bundle extracted (%s)\n", store.CombinedBundlePath())
	fmt.Printf("\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. Add corporate certificates (if needed):\n")
	fmt.Printf("     verifi cert add /path/to/cert.pem --name corporate\n")
	fmt.Printf("\n")
	fmt.Printf("  2. Configure your shell (Phase 4 will add env.sh generation):\n")
	fmt.Printf("     export SSL_CERT_FILE=%s\n", store.CombinedBundlePath())
	fmt.Printf("\n")
	fmt.Printf("  3. Verify it works:\n")
	fmt.Printf("     verifi verify --url https://registry.npmjs.org\n")
	fmt.Printf("\n")

	return nil
}
