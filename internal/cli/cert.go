package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/princespaghetti/verifi/internal/certstore"
	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

var (
	certName    string
	certForce   bool
	certStdin   bool
	certJSON    bool
	certExpired bool
)

// certCmd represents the cert command group.
var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Certificate management commands",
	Long:  `Manage user certificates in the verifi certificate store.`,
}

// certAddCmd represents the cert add command.
var certAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a certificate to the store",
	Long: `Add a certificate to the user certificate store.

The certificate will be validated before being added. By default, expired
certificates are rejected. Use --force to add expired certificates.

Use --stdin to read the certificate from standard input instead of a file.

Examples:
  verifi cert add /path/to/cert.pem --name corporate
  verifi cert add proxy-cert.pem --name proxy --force
  curl https://internal.corp.com/ca.crt | verifi cert add --stdin --name internal`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCertAdd,
}

// certListCmd represents the cert list command.
var certListCmd = &cobra.Command{
	Use:   "list",
	Short: "List certificates in the store",
	Long: `List all user certificates in the certificate store.

By default, certificates are displayed in a table format. Use --json for
machine-readable output. Use --expired to show only expired certificates.

Examples:
  verifi cert list
  verifi cert list --json
  verifi cert list --expired`,
	RunE: runCertList,
}

// certRemoveCmd represents the cert remove command.
var certRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a certificate from the store",
	Long: `Remove a user certificate from the certificate store by name.

The certificate file will be deleted and the combined bundle will be rebuilt.

Examples:
  verifi cert remove corporate
  verifi cert remove proxy`,
	Args: cobra.ExactArgs(1),
	RunE: runCertRemove,
}

// certInspectCmd represents the cert inspect command.
var certInspectCmd = &cobra.Command{
	Use:   "inspect <name>",
	Short: "Show detailed information about a certificate",
	Long: `Display detailed information about a specific user certificate.

Shows the subject, issuer, expiration date, fingerprint, and other details.

Examples:
  verifi cert inspect corporate
  verifi cert inspect proxy --json`,
	Args: cobra.ExactArgs(1),
	RunE: runCertInspect,
}

func init() {
	// Add cert command to root
	rootCmd.AddCommand(certCmd)

	// Add subcommands
	certCmd.AddCommand(certAddCmd)
	certCmd.AddCommand(certListCmd)
	certCmd.AddCommand(certRemoveCmd)
	certCmd.AddCommand(certInspectCmd)

	// cert add flags
	certAddCmd.Flags().StringVar(&certName, "name", "", "Certificate name (required)")
	certAddCmd.Flags().BoolVar(&certForce, "force", false, "Force add even if expired")
	certAddCmd.Flags().BoolVar(&certStdin, "stdin", false, "Read certificate from stdin")
	_ = certAddCmd.MarkFlagRequired("name") // Ignore error - setup failure would be caught at runtime

	// cert list flags
	certListCmd.Flags().BoolVar(&certJSON, "json", false, "Output in JSON format")
	certListCmd.Flags().BoolVar(&certExpired, "expired", false, "Show only expired certificates")

	// cert inspect flags
	certInspectCmd.Flags().BoolVar(&certJSON, "json", false, "Output in JSON format")
}

func runCertAdd(cmd *cobra.Command, args []string) error {
	var certPath string
	var tempFile *os.File
	var cleanupTemp bool

	// Handle stdin vs file path
	if certStdin {
		// Read from stdin
		if len(args) > 0 {
			Error("Cannot specify both --stdin and a file path")
			os.Exit(verifierrors.ExitConfigError)
		}

		// Create temporary file for stdin data
		var err error
		tempFile, err = os.CreateTemp("", "verifi-cert-*.pem")
		if err != nil {
			Error("Failed to create temporary file: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
		certPath = tempFile.Name()
		cleanupTemp = true
		defer func() {
			if cleanupTemp {
				_ = os.Remove(certPath)
			}
		}()

		// Read from stdin and write to temp file
		Info("Reading certificate from stdin...")
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			Error("Failed to read from stdin: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}

		if len(data) == 0 {
			Error("No certificate data provided on stdin")
			os.Exit(verifierrors.ExitConfigError)
		}

		if _, err := tempFile.Write(data); err != nil {
			Error("Failed to write certificate data: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}

		if err := tempFile.Close(); err != nil {
			Error("Failed to close temporary file: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
	} else {
		// Read from file path
		if len(args) == 0 {
			Error("Certificate path required (or use --stdin)")
			fmt.Fprintf(os.Stderr, "Usage: verifi cert add <path> --name <name>\n")
			fmt.Fprintf(os.Stderr, "   or: verifi cert add --stdin --name <name>\n")
			os.Exit(verifierrors.ExitConfigError)
		}
		certPath = args[0]
	}

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

	// Add certificate with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if certStdin {
		Info("Adding certificate '%s' from stdin...", certName)
	} else {
		Info("Adding certificate '%s' from %s...", certName, certPath)
	}

	if err := store.AddCert(ctx, certPath, certName, certForce); err != nil {
		// Check for specific error types
		if verifierrors.IsError(err, verifierrors.ErrCertExpired) {
			Error("Certificate has expired")
			fmt.Fprintf(os.Stderr, "Use --force to add expired certificates\n")
			os.Exit(verifierrors.ExitCertError)
		}
		if verifierrors.IsError(err, verifierrors.ErrInvalidPEM) {
			Error("Invalid PEM format")
			os.Exit(verifierrors.ExitCertError)
		}

		Error("Failed to add certificate: %v", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	// Get certificate info to display
	certs, err := store.ListCerts()
	if err != nil {
		// Certificate was added but we can't read it back - still success
		Success("Certificate '%s' added successfully", certName)
		return nil
	}

	// Find the certificate we just added
	for _, cert := range certs {
		if cert.Name == certName {
			Success("Certificate '%s' added successfully", certName)
			FieldIndented("Subject", cert.Subject, 2)
			FieldIndented("Fingerprint", cert.Fingerprint, 2)
			FieldIndented("Expires", cert.Expires.Format("2006-01-02 15:04:05 MST"), 2)
			FieldIndented("Path", cert.Path, 2)
			EmptyLine()
			Info("Combined bundle rebuilt: %s", store.CombinedBundlePath())
			return nil
		}
	}

	Success("Certificate '%s' added successfully", certName)
	return nil
}

func runCertList(cmd *cobra.Command, args []string) error {
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

	// Get certificates
	certs, err := store.ListCerts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to list certificates: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	// Filter expired if requested
	if certExpired {
		now := time.Now()
		filtered := []certstore.UserCertInfo{}
		for _, cert := range certs {
			if now.After(cert.Expires) {
				filtered = append(filtered, cert)
			}
		}
		certs = filtered
	}

	// Handle no certificates
	if len(certs) == 0 {
		if certExpired {
			Info("No expired certificates found")
		} else {
			Info("No user certificates in store")
			EmptyLine()
			Info("Add certificates with: verifi cert add <path> --name <name>")
		}
		return nil
	}

	// JSON output
	if certJSON {
		if err := JSON(certs); err != nil {
			Error("Failed to encode JSON: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
		return nil
	}

	// Table output
	fmt.Printf("User Certificates (%d)\n\n", len(certs))

	table := NewTable("NAME", "SUBJECT", "EXPIRES", "STATUS")
	now := time.Now()
	for _, cert := range certs {
		// Truncate subject if too long
		subject := TruncateString(cert.Subject, 40)

		// Determine status
		status := "Valid"
		if now.After(cert.Expires) {
			status = "EXPIRED"
		}

		expiresStr := cert.Expires.Format("2006-01-02 15:04")
		table.AddRow(cert.Name, subject, expiresStr, status)
	}
	table.Print()

	return nil
}

func runCertRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

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

	// Remove certificate with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	Info("Removing certificate '%s'...", name)

	if err := store.RemoveCert(ctx, name); err != nil {
		// Check for specific error types
		if verifierrors.IsError(err, verifierrors.ErrCertNotFound) {
			Error("Certificate '%s' not found", name)
			fmt.Fprintf(os.Stderr, "Use 'verifi cert list' to see available certificates\n")
			os.Exit(verifierrors.ExitCertError)
		}

		Error("Failed to remove certificate: %v", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	Success("Certificate '%s' removed successfully", name)
	EmptyLine()
	Info("Combined bundle rebuilt: %s", store.CombinedBundlePath())

	return nil
}

func runCertInspect(cmd *cobra.Command, args []string) error {
	name := args[0]

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

	// Get certificate info
	info, err := store.GetCertInfo(name)
	if err != nil {
		// Check for specific error types
		if verifierrors.IsError(err, verifierrors.ErrCertNotFound) {
			Error("Certificate '%s' not found", name)
			fmt.Fprintf(os.Stderr, "Use 'verifi cert list' to see available certificates\n")
			os.Exit(verifierrors.ExitCertError)
		}

		Error("Failed to get certificate info: %v", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	// JSON output
	if certJSON {
		if err := JSON(info); err != nil {
			Error("Failed to encode JSON: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
		return nil
	}

	// Human-readable output
	fmt.Printf("Certificate: %s\n", info.Name)
	Separator("=", 40)
	EmptyLine()

	Field("Subject", info.Subject)
	Field("Fingerprint", info.Fingerprint)
	Field("Expires", info.Expires.Format("2006-01-02 15:04:05 MST"))
	Field("Added", info.Added.Format("2006-01-02 15:04:05 MST"))
	Field("Path", info.Path)

	// Check if expired
	now := time.Now()
	EmptyLine()
	if now.After(info.Expires) {
		Field("Status", "EXPIRED")
	} else {
		daysUntilExpiry := int(time.Until(info.Expires).Hours() / 24)
		Field("Status", fmt.Sprintf("Valid (%d days until expiry)", daysUntilExpiry))
	}

	return nil
}
