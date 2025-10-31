package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/princespaghetti/verifi/internal/certstore"
	verifierrors "github.com/princespaghetti/verifi/internal/errors"
	"github.com/princespaghetti/verifi/internal/fetcher"
)

var (
	bundleJSON bool
	bundleURL  string
)

// bundleCmd represents the bundle command.
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Mozilla CA bundle management commands",
	Long: `Manage the Mozilla CA certificate bundle.

Commands:
  info   - Display information about the current Mozilla bundle
  update - Download and update the Mozilla bundle from curl.se

Examples:
  verifi bundle info
  verifi bundle info --json
  verifi bundle update
  verifi bundle update --url https://custom-mirror.example.com/cacert.pem`,
}

// bundleInfoCmd represents the bundle info command.
var bundleInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display Mozilla CA bundle information",
	Long: `Display information about the current Mozilla CA bundle.

Shows:
  - Source (embedded or curl.se)
  - Mozilla bundle date (if available)
  - Certificate count
  - SHA256 hash
  - File size
  - Last updated timestamp

Examples:
  verifi bundle info
  verifi bundle info --json`,
	RunE: runBundleInfo,
}

// bundleUpdateCmd represents the bundle update command.
var bundleUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and update the Mozilla CA bundle",
	Long: `Download the latest Mozilla CA bundle from curl.se and update the local copy.

By default, downloads from: https://curl.se/ca/cacert.pem

The bundle is:
  1. Downloaded to a temporary file
  2. Verified (valid PEM format, minimum cert count)
  3. Checked for degradation (warns if cert count drops >20%)
  4. Atomically replaces the current Mozilla bundle
  5. Triggers rebuild of the combined bundle
  6. Updates metadata with new version information

Examples:
  verifi bundle update
  verifi bundle update --url https://internal-mirror.corp.com/cacert.pem`,
	RunE: runBundleUpdate,
}

// bundleResetCmd represents the bundle reset command.
var bundleResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset Mozilla CA bundle to embedded version",
	Long: `Reset the Mozilla CA bundle to the original embedded version.

This is useful if:
  - A bundle update failed or resulted in a corrupted file
  - You want to revert to the known-good embedded bundle
  - You're troubleshooting certificate issues

The embedded bundle is the version included in the verifi binary at build time.
The combined bundle will be rebuilt after the reset.

Examples:
  verifi bundle reset`,
	RunE: runBundleReset,
}

func init() {
	rootCmd.AddCommand(bundleCmd)
	bundleCmd.AddCommand(bundleInfoCmd)
	bundleCmd.AddCommand(bundleUpdateCmd)
	bundleCmd.AddCommand(bundleResetCmd)

	// Flags for info command
	bundleInfoCmd.Flags().BoolVar(&bundleJSON, "json", false, "Output in JSON format")

	// Flags for update command
	bundleUpdateCmd.Flags().StringVar(&bundleURL, "url", fetcher.DefaultMozillaBundleURL, "URL to download bundle from")
}

// BundleInfoOutput represents the output of the bundle info command.
type BundleInfoOutput struct {
	Source    string    `json:"source"`
	Version   string    `json:"version,omitempty"`
	CertCount int       `json:"cert_count"`
	SHA256    string    `json:"sha256"`
	Generated time.Time `json:"generated"`
	SizeBytes int64     `json:"size_bytes,omitempty"`
	FilePath  string    `json:"file_path"`
}

func runBundleInfo(cmd *cobra.Command, args []string) error {
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

	// Get metadata
	metadata, err := store.GetMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read metadata: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	// Get file size
	mozillaBundlePath := store.BasePath() + "/certs/bundles/mozilla-ca-bundle.pem"
	var sizeBytes int64
	if info, err := os.Stat(mozillaBundlePath); err == nil {
		sizeBytes = info.Size()
	}

	// Prepare output
	output := BundleInfoOutput{
		Source:    metadata.MozillaBundle.Source,
		Version:   metadata.MozillaBundle.Version,
		CertCount: metadata.MozillaBundle.CertCount,
		SHA256:    metadata.MozillaBundle.SHA256,
		Generated: metadata.MozillaBundle.Generated,
		SizeBytes: sizeBytes,
		FilePath:  mozillaBundlePath,
	}

	// Output
	if bundleJSON {
		if err := JSON(output); err != nil {
			Error("Failed to encode JSON: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
	} else {
		printBundleInfoHuman(output)
	}

	return nil
}

func printBundleInfoHuman(info BundleInfoOutput) {
	Header("Mozilla CA Bundle Information")
	Field("Source", info.Source)
	if info.Version != "" {
		Field("Version", info.Version)
	}
	Field("Certificates", fmt.Sprintf("%d", info.CertCount))
	if info.SizeBytes > 0 {
		Field("Size", FormatBytes(info.SizeBytes))
	}
	Field("Generated", info.Generated.Format("2006-01-02 15:04:05 MST"))
	Field("File Path", info.FilePath)
	EmptyLine()
	Field("SHA256", info.SHA256)
	EmptyLine()
}

func runBundleUpdate(cmd *cobra.Command, args []string) error {
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

	// Get current metadata for comparison
	metadata, err := store.GetMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read metadata: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	currentCertCount := metadata.MozillaBundle.CertCount

	Info("Downloading Mozilla CA bundle from %s...", bundleURL)

	// Download bundle with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f := fetcher.NewFetcher(nil)
	bundleData, err := f.FetchMozillaBundle(ctx, bundleURL)
	if err != nil {
		Error("Failed to download bundle: %v", err)
		os.Exit(verifierrors.ExitNetworkError)
	}

	// Verify bundle
	verifyResult, err := fetcher.VerifyBundle(bundleData, currentCertCount)
	if err != nil {
		Error("Bundle verification failed: %v", err)
		os.Exit(verifierrors.ExitCertError)
	}

	// Show warning if there's degradation
	if verifyResult.Warning != "" {
		Warning("%s", verifyResult.Warning)
		fmt.Fprintf(os.Stderr, "Continue anyway? This could indicate a problem with the download.\n")
		fmt.Fprintf(os.Stderr, "Press Ctrl+C to abort, or Enter to continue: ")
		_, _ = fmt.Scanln() // Wait for user confirmation (ignore error - continue anyway)
	}

	// Write new bundle atomically
	mozillaBundlePath := store.BasePath() + "/certs/bundles/mozilla-ca-bundle.pem"
	tempPath := mozillaBundlePath + ".tmp"

	if err := os.WriteFile(tempPath, bundleData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to write temp bundle: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	if err := os.Rename(tempPath, mozillaBundlePath); err != nil {
		_ = os.Remove(tempPath)
		fmt.Fprintf(os.Stderr, "Error: Failed to replace bundle: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	// Update metadata
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	updateErr := store.UpdateMetadata(ctx2, func(md *certstore.Metadata) error {
		// Parse Mozilla date from bundle header
		mozillaDateStr := fetcher.ExtractMozillaDateString(bundleData)

		// Update Mozilla bundle info
		md.MozillaBundle = certstore.BundleInfo{
			Generated: time.Now(),
			SHA256:    computeSHA256(bundleData),
			CertCount: verifyResult.CertCount,
			Source:    bundleURL,
			Version:   mozillaDateStr,
		}

		// Rebuild combined bundle
		return store.RebuildBundle(ctx2, md)
	})

	if updateErr != nil {
		Error("Failed to update metadata and rebuild bundle: %v", updateErr)
		fmt.Fprintf(os.Stderr, "The Mozilla bundle was updated but the combined bundle may be out of sync.\n")
		fmt.Fprintf(os.Stderr, "Run 'verifi doctor' to repair the store.\n")
		os.Exit(verifierrors.ExitGeneralError)
	}

	// Show success message
	EmptyLine()
	Success("Bundle updated successfully")
	FieldIndented("Downloaded from", bundleURL, 2)
	if verifyResult.HasDateInHeader {
		FieldIndented("Mozilla date", verifyResult.MozillaDate.Format("January 2, 2006"), 2)
	}
	certInfo := fmt.Sprintf("%d", verifyResult.CertCount)
	if currentCertCount > 0 {
		diff := verifyResult.CertCount - currentCertCount
		if diff > 0 {
			certInfo = fmt.Sprintf("%d (+%d from previous)", verifyResult.CertCount, diff)
		} else if diff < 0 {
			certInfo = fmt.Sprintf("%d (%d from previous)", verifyResult.CertCount, diff)
		} else {
			certInfo = fmt.Sprintf("%d (no change)", verifyResult.CertCount)
		}
	}
	FieldIndented("Certificates", certInfo, 2)
	EmptyLine()

	return nil
}

// computeSHA256 computes the SHA256 hash of data and returns it as a hex string.
func computeSHA256(data []byte) string {
	hash := fetcher.ComputeSHA256(data)
	return hash
}

func runBundleReset(cmd *cobra.Command, args []string) error {
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

	// Reset bundle with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	Info("Resetting Mozilla CA bundle to embedded version...")

	if err := store.ResetMozillaBundle(ctx); err != nil {
		Error("Failed to reset bundle: %v", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	// Get updated metadata to show info
	metadata, err := store.GetMetadata()
	if err != nil {
		// Reset succeeded but can't read metadata - still success
		EmptyLine()
		Success("Mozilla CA bundle reset to embedded version")
		return nil
	}

	// Show success message with details
	EmptyLine()
	Success("Mozilla CA bundle reset to embedded version")
	FieldIndented("Source", "embedded", 2)
	FieldIndented("Certificates", fmt.Sprintf("%d", metadata.MozillaBundle.CertCount), 2)
	FieldIndented("Updated", metadata.MozillaBundle.Generated.Format("2006-01-02 15:04:05 MST"), 2)
	EmptyLine()
	Info("Combined bundle rebuilt: %s", store.CombinedBundlePath())

	return nil
}
