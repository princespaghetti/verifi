package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/princespaghetti/verifi/internal/certstore"
	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

var (
	cleanFull  bool
	cleanForce bool
)

// cleanCmd represents the clean command.
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up temporary files or remove the entire store",
	Long: `Clean up temporary files from the certificate store or remove it entirely.

By default, removes only temporary files (*.tmp, *.lock).

Use --full to remove the entire certificate store (requires confirmation).
Use --full --force to skip confirmation (dangerous).

Examples:
  verifi clean                # Remove temp files only
  verifi clean --full         # Remove entire store (with confirmation)
  verifi clean --full --force # Remove entire store (no confirmation)`,
	RunE: runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanFull, "full", false, "Remove entire certificate store")
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false, "Skip confirmation prompts")
}

func runClean(cmd *cobra.Command, args []string) error {
	// Create store
	store, err := certstore.NewStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create store: %v\n", err)
		os.Exit(verifierrors.ExitConfigError)
	}

	basePath := store.BasePath()

	// Full cleanup
	if cleanFull {
		return runFullCleanup(basePath)
	}

	// Temp file cleanup
	return runTempCleanup(basePath)
}

func runTempCleanup(basePath string) error {
	fmt.Println("Cleaning temporary files...")

	// Find all temp and lock files
	tempPatterns := []string{"*.tmp", "*.lock"}
	foundFiles := []string{}

	for _, pattern := range tempPatterns {
		matches, err := filepath.Glob(filepath.Join(basePath, "**", pattern))
		if err != nil {
			continue
		}
		foundFiles = append(foundFiles, matches...)
	}

	// Also check in the base directory
	for _, pattern := range tempPatterns {
		matches, err := filepath.Glob(filepath.Join(basePath, pattern))
		if err != nil {
			continue
		}
		foundFiles = append(foundFiles, matches...)
	}

	// Also check in certs directory
	for _, pattern := range tempPatterns {
		matches, err := filepath.Glob(filepath.Join(basePath, "certs", pattern))
		if err != nil {
			continue
		}
		foundFiles = append(foundFiles, matches...)
	}

	// Also check in bundles directory
	for _, pattern := range tempPatterns {
		matches, err := filepath.Glob(filepath.Join(basePath, "certs", "bundles", pattern))
		if err != nil {
			continue
		}
		foundFiles = append(foundFiles, matches...)
	}

	if len(foundFiles) == 0 {
		fmt.Println("\n✓ No temporary files found")
		return nil
	}

	// Remove temp files
	removedCount := 0
	for _, file := range foundFiles {
		if err := os.Remove(file); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to remove %s: %v\n", file, err)
		} else {
			removedCount++
			fmt.Printf("  Removed: %s\n", filepath.Base(file))
		}
	}

	fmt.Printf("\n✓ Removed %d temporary file(s)\n", removedCount)
	return nil
}

func runFullCleanup(basePath string) error {
	// Check if store exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		fmt.Println("Certificate store does not exist")
		return nil
	}

	// Require confirmation unless --force
	if !cleanForce {
		fmt.Println("WARNING: This will permanently delete the entire certificate store!")
		fmt.Printf("Location: %s\n", basePath)
		fmt.Println()
		fmt.Println("This will remove:")
		fmt.Println("  - All user certificates")
		fmt.Println("  - Mozilla CA bundle")
		fmt.Println("  - Combined bundle")
		fmt.Println("  - Metadata")
		fmt.Println("  - Environment file (env.sh)")
		fmt.Println()
		fmt.Print("Are you sure you want to continue? Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			os.Exit(verifierrors.ExitGeneralError)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("\nAborted. Certificate store was not removed.")
			return nil
		}
	}

	// Remove the entire directory
	fmt.Printf("\nRemoving certificate store at %s...\n", basePath)

	if err := os.RemoveAll(basePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to remove store: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	fmt.Println("\n✓ Certificate store removed successfully")
	fmt.Println("\nTo recreate the store, run: verifi init")

	return nil
}
