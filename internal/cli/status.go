package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/princespaghetti/verifi/internal/certstore"
	verifierrors "github.com/princespaghetti/verifi/internal/errors"
	"github.com/princespaghetti/verifi/internal/shell"
)

var statusJSON bool

// statusCmd represents the status command.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display certificate store status",
	Long: `Display information about the certificate store without making network connections.

Shows:
  - Certificate store location
  - Initialization status
  - Number of user certificates
  - Combined bundle details
  - Mozilla bundle information
  - Environment file status

Examples:
  verifi status
  verifi status --json`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output in JSON format")
}

// StatusOutput represents the structured output of the status command.
type StatusOutput struct {
	StoreLocation  string               `json:"store_location"`
	Initialized    bool                 `json:"initialized"`
	UserCerts      UserCertsStatus      `json:"user_certificates"`
	CombinedBundle CombinedBundleStatus `json:"combined_bundle"`
	MozillaBundle  MozillaBundleStatus  `json:"mozilla_bundle"`
	EnvFile        EnvFileStatus        `json:"env_file"`
}

// UserCertsStatus represents user certificate information.
type UserCertsStatus struct {
	Count int                       `json:"count"`
	Certs []certstore.UserCertInfo `json:"certs,omitempty"`
}

// CombinedBundleStatus represents combined bundle information.
type CombinedBundleStatus struct {
	Path      string    `json:"path"`
	CertCount int       `json:"cert_count"`
	Generated time.Time `json:"generated"`
	SHA256    string    `json:"sha256"`
	SizeBytes int64     `json:"size_bytes"`
	Sources   []string  `json:"sources"`
}

// MozillaBundleStatus represents Mozilla bundle information.
type MozillaBundleStatus struct {
	Source    string    `json:"source"`
	CertCount int       `json:"cert_count"`
	Generated time.Time `json:"generated"`
}

// EnvFileStatus represents environment file information.
type EnvFileStatus struct {
	Exists bool   `json:"exists"`
	Path   string `json:"path"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Create store
	store, err := certstore.NewStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create store: %v\n", err)
		os.Exit(verifierrors.ExitConfigError)
	}

	// Gather status information
	status := gatherStatus(store)

	// Output
	if statusJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(status); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to encode JSON: %v\n", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
	} else {
		printStatusHuman(status)
	}

	return nil
}

// gatherStatus collects status information from the store.
func gatherStatus(store *certstore.Store) StatusOutput {
	status := StatusOutput{
		StoreLocation: store.BasePath(),
		Initialized:   store.IsInitialized(),
	}

	// If not initialized, return early
	if !status.Initialized {
		return status
	}

	// Get user certificates
	if userCerts, err := store.ListCerts(); err == nil {
		status.UserCerts.Count = len(userCerts)
		status.UserCerts.Certs = userCerts
	}

	// Get metadata for bundle info
	if metadata, err := store.GetMetadata(); err == nil {
		// Combined bundle
		bundlePath := store.CombinedBundlePath()
		status.CombinedBundle = CombinedBundleStatus{
			Path:      bundlePath,
			CertCount: metadata.CombinedBundle.CertCount,
			Generated: metadata.CombinedBundle.Generated,
			SHA256:    metadata.CombinedBundle.SHA256,
			Sources:   metadata.CombinedBundle.Sources,
		}

		// Get file size
		if info, err := os.Stat(bundlePath); err == nil {
			status.CombinedBundle.SizeBytes = info.Size()
		}

		// Mozilla bundle
		status.MozillaBundle = MozillaBundleStatus{
			Source:    metadata.MozillaBundle.Source,
			CertCount: metadata.MozillaBundle.CertCount,
			Generated: metadata.MozillaBundle.Generated,
		}
	}

	// Check env.sh
	envPath := shell.EnvFilePath(store.BasePath())
	_, err := os.Stat(envPath)
	status.EnvFile = EnvFileStatus{
		Exists: err == nil,
		Path:   envPath,
	}

	return status
}

// printStatusHuman prints the status in a human-readable format.
func printStatusHuman(status StatusOutput) {
	fmt.Println("Certificate Store Status")
	fmt.Println("========================")
	fmt.Println()

	// Store location
	fmt.Printf("Store Location:  %s\n", status.StoreLocation)
	fmt.Printf("Initialized:     %v\n", status.Initialized)
	fmt.Println()

	if !status.Initialized {
		fmt.Println("Store is not initialized. Run 'verifi init' to initialize.")
		return
	}

	// User certificates
	fmt.Println("User Certificates")
	fmt.Println("-----------------")
	fmt.Printf("Count:           %d\n", status.UserCerts.Count)
	if status.UserCerts.Count > 0 {
		fmt.Println()
		for _, cert := range status.UserCerts.Certs {
			fmt.Printf("  - %s\n", cert.Name)
			fmt.Printf("    Subject:   %s\n", cert.Subject)
			fmt.Printf("    Expires:   %s\n", cert.Expires.Format("2006-01-02 15:04:05 MST"))
			fmt.Printf("    Added:     %s\n", cert.Added.Format("2006-01-02 15:04:05 MST"))
		}
	}
	fmt.Println()

	// Combined bundle
	fmt.Println("Combined Bundle")
	fmt.Println("---------------")
	fmt.Printf("Path:            %s\n", status.CombinedBundle.Path)
	fmt.Printf("Certificates:    %d\n", status.CombinedBundle.CertCount)
	if len(status.CombinedBundle.Sources) > 0 {
		fmt.Printf("Sources:         %v\n", status.CombinedBundle.Sources)
	}
	if status.CombinedBundle.SizeBytes > 0 {
		fmt.Printf("Size:            %s\n", formatBytes(status.CombinedBundle.SizeBytes))
	}
	if !status.CombinedBundle.Generated.IsZero() {
		fmt.Printf("Generated:       %s\n", status.CombinedBundle.Generated.Format("2006-01-02 15:04:05 MST"))
	}
	fmt.Println()

	// Mozilla bundle
	fmt.Println("Mozilla CA Bundle")
	fmt.Println("-----------------")
	fmt.Printf("Source:          %s\n", status.MozillaBundle.Source)
	fmt.Printf("Certificates:    %d\n", status.MozillaBundle.CertCount)
	if !status.MozillaBundle.Generated.IsZero() {
		fmt.Printf("Generated:       %s\n", status.MozillaBundle.Generated.Format("2006-01-02 15:04:05 MST"))
	}
	fmt.Println()

	// Environment file
	fmt.Println("Environment File")
	fmt.Println("----------------")
	fmt.Printf("Path:            %s\n", status.EnvFile.Path)
	fmt.Printf("Exists:          %v\n", status.EnvFile.Exists)
	if !status.EnvFile.Exists {
		fmt.Println()
		fmt.Println("Warning: env.sh not found. Run 'verifi env' to regenerate it.")
	}
	fmt.Println()
}

// formatBytes formats byte sizes in human-readable format (B, KB, MB, etc.).
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
