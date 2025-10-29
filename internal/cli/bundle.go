package cli

import (
	"context"
	"encoding/json"
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

func init() {
	rootCmd.AddCommand(bundleCmd)
	bundleCmd.AddCommand(bundleInfoCmd)
	bundleCmd.AddCommand(bundleUpdateCmd)

	// Flags for info command
	bundleInfoCmd.Flags().BoolVar(&bundleJSON, "json", false, "Output in JSON format")

	// Flags for update command
	bundleUpdateCmd.Flags().StringVar(&bundleURL, "url", fetcher.DefaultMozillaBundleURL, "URL to download bundle from")
}

// BundleInfoOutput represents the output of the bundle info command.
type BundleInfoOutput struct {
	Source      string    `json:"source"`
	Version     string    `json:"version,omitempty"`
	CertCount   int       `json:"cert_count"`
	SHA256      string    `json:"sha256"`
	Generated   time.Time `json:"generated"`
	SizeBytes   int64     `json:"size_bytes,omitempty"`
	FilePath    string    `json:"file_path"`
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
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to encode JSON: %v\n", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
	} else {
		printBundleInfoHuman(output)
	}

	return nil
}

func printBundleInfoHuman(info BundleInfoOutput) {
	fmt.Println("Mozilla CA Bundle Information")
	fmt.Println("==============================")
	fmt.Println()
	fmt.Printf("Source:          %s\n", info.Source)
	if info.Version != "" {
		fmt.Printf("Version:         %s\n", info.Version)
	}
	fmt.Printf("Certificates:    %d\n", info.CertCount)
	if info.SizeBytes > 0 {
		fmt.Printf("Size:            %s\n", formatBytes(info.SizeBytes))
	}
	fmt.Printf("Generated:       %s\n", info.Generated.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("File Path:       %s\n", info.FilePath)
	fmt.Println()
	fmt.Printf("SHA256:          %s\n", info.SHA256)
	fmt.Println()
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

	fmt.Printf("Downloading Mozilla CA bundle from %s...\n", bundleURL)

	// Download bundle with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f := fetcher.NewFetcher(nil)
	bundleData, err := f.FetchMozillaBundle(ctx, bundleURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to download bundle: %v\n", err)
		os.Exit(verifierrors.ExitNetworkError)
	}

	// Verify bundle
	verifyResult, err := fetcher.VerifyBundle(bundleData, currentCertCount)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Bundle verification failed: %v\n", err)
		os.Exit(verifierrors.ExitCertError)
	}

	// Show warning if there's degradation
	if verifyResult.Warning != "" {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", verifyResult.Warning)
		fmt.Fprintf(os.Stderr, "Continue anyway? This could indicate a problem with the download.\n")
		fmt.Fprintf(os.Stderr, "Press Ctrl+C to abort, or Enter to continue: ")
		fmt.Scanln() // Wait for user confirmation
	}

	// Write new bundle atomically
	mozillaBundlePath := store.BasePath() + "/certs/bundles/mozilla-ca-bundle.pem"
	tempPath := mozillaBundlePath + ".tmp"

	if err := os.WriteFile(tempPath, bundleData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to write temp bundle: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	if err := os.Rename(tempPath, mozillaBundlePath); err != nil {
		os.Remove(tempPath)
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
		fmt.Fprintf(os.Stderr, "Error: Failed to update metadata and rebuild bundle: %v\n", updateErr)
		fmt.Fprintf(os.Stderr, "The Mozilla bundle was updated but the combined bundle may be out of sync.\n")
		fmt.Fprintf(os.Stderr, "Run 'verifi doctor' (in Phase 7) to repair the store.\n")
		os.Exit(verifierrors.ExitGeneralError)
	}

	// Show success message
	fmt.Printf("\n✓ Bundle updated successfully\n")
	fmt.Printf("  Downloaded from: %s\n", bundleURL)
	if verifyResult.HasDateInHeader {
		fmt.Printf("  Mozilla date:    %s\n", verifyResult.MozillaDate.Format("January 2, 2006"))
	}
	fmt.Printf("  Certificates:    %d", verifyResult.CertCount)
	if currentCertCount > 0 {
		diff := verifyResult.CertCount - currentCertCount
		if diff > 0 {
			fmt.Printf(" (+%d from previous)\n", diff)
		} else if diff < 0 {
			fmt.Printf(" (%d from previous)\n", diff)
		} else {
			fmt.Printf(" (no change)\n")
		}
	} else {
		fmt.Println()
	}
	fmt.Println()

	return nil
}

// computeSHA256 computes the SHA256 hash of data and returns it as a hex string.
func computeSHA256(data []byte) string {
	hash := fetcher.ComputeSHA256(data)
	return hash
}
