package cli

import (
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
	Count int                      `json:"count"`
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
		if err := JSON(status); err != nil {
			Error("Failed to encode JSON: %v", err)
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
	Header("Certificate Store Status")

	// Store location
	Field("Store Location", status.StoreLocation)
	Field("Initialized", fmt.Sprintf("%v", status.Initialized))
	EmptyLine()

	if !status.Initialized {
		Warning("Store is not initialized. Run 'verifi init' to initialize.")
		return
	}

	// User certificates
	Subheader("User Certificates")
	Field("Count", fmt.Sprintf("%d", status.UserCerts.Count))
	if status.UserCerts.Count > 0 {
		EmptyLine()
		for _, cert := range status.UserCerts.Certs {
			fmt.Printf("  • %s\n", cert.Name)
			FieldIndented("Subject", cert.Subject, 4)
			FieldIndented("Expires", cert.Expires.Format("2006-01-02 15:04:05 MST"), 4)
			FieldIndented("Added", cert.Added.Format("2006-01-02 15:04:05 MST"), 4)
		}
	}
	EmptyLine()

	// Combined bundle
	Subheader("Combined Bundle")
	Field("Path", status.CombinedBundle.Path)
	Field("Certificates", fmt.Sprintf("%d", status.CombinedBundle.CertCount))
	if len(status.CombinedBundle.Sources) > 0 {
		Field("Sources", fmt.Sprintf("%v", status.CombinedBundle.Sources))
	}
	if status.CombinedBundle.SizeBytes > 0 {
		Field("Size", FormatBytes(status.CombinedBundle.SizeBytes))
	}
	if !status.CombinedBundle.Generated.IsZero() {
		Field("Generated", status.CombinedBundle.Generated.Format("2006-01-02 15:04:05 MST"))
	}
	EmptyLine()

	// Mozilla bundle
	Subheader("Mozilla CA Bundle")
	Field("Source", status.MozillaBundle.Source)
	Field("Certificates", fmt.Sprintf("%d", status.MozillaBundle.CertCount))
	if !status.MozillaBundle.Generated.IsZero() {
		Field("Generated", status.MozillaBundle.Generated.Format("2006-01-02 15:04:05 MST"))
	}
	EmptyLine()

	// Environment file
	Subheader("Environment File")
	Field("Path", status.EnvFile.Path)
	Field("Exists", fmt.Sprintf("%v", status.EnvFile.Exists))
	if !status.EnvFile.Exists {
		EmptyLine()
		Warning("env.sh not found. Run 'verifi env' to regenerate it.")
	}
	EmptyLine()
}
