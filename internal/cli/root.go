// Package cli provides the command-line interface for verifi.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information (will be set by build flags in production).
var (
	Version   = "dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "verifi",
	Short: "Unified certificate management for developers",
	Long: `verifi manages SSL/TLS certificates across multiple developer tools.

It maintains a central certificate store at ~/.verifi/ and configures
development environments through environment variables, eliminating the
need to configure certificates separately for npm, pip, git, curl, etc.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("verifi version %s\n", Version)
		fmt.Printf("  commit: %s\n", GitCommit)
		fmt.Printf("  built:  %s\n", BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command and handles errors.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
