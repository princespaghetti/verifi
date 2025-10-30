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
)

var (
	certName    string
	certForce   bool
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
	Use:   "add <path>",
	Short: "Add a certificate to the store",
	Long: `Add a certificate to the user certificate store.

The certificate will be validated before being added. By default, expired
certificates are rejected. Use --force to add expired certificates.

Examples:
  verifi cert add /path/to/cert.pem --name corporate
  verifi cert add proxy-cert.pem --name proxy --force`,
	Args: cobra.ExactArgs(1),
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
	certAddCmd.MarkFlagRequired("name")

	// cert list flags
	certListCmd.Flags().BoolVar(&certJSON, "json", false, "Output in JSON format")
	certListCmd.Flags().BoolVar(&certExpired, "expired", false, "Show only expired certificates")

	// cert inspect flags
	certInspectCmd.Flags().BoolVar(&certJSON, "json", false, "Output in JSON format")
}

func runCertAdd(cmd *cobra.Command, args []string) error {
	certPath := args[0]

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

	fmt.Printf("Adding certificate '%s' from %s...\n", certName, certPath)

	if err := store.AddCert(ctx, certPath, certName, certForce); err != nil {
		// Check for specific error types
		if verifierrors.IsError(err, verifierrors.ErrCertExpired) {
			fmt.Fprintf(os.Stderr, "Error: Certificate has expired\n")
			fmt.Fprintf(os.Stderr, "Use --force to add expired certificates\n")
			os.Exit(verifierrors.ExitCertError)
		}
		if verifierrors.IsError(err, verifierrors.ErrInvalidPEM) {
			fmt.Fprintf(os.Stderr, "Error: Invalid PEM format\n")
			os.Exit(verifierrors.ExitCertError)
		}

		fmt.Fprintf(os.Stderr, "Error: Failed to add certificate: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	// Get certificate info to display
	certs, err := store.ListCerts()
	if err != nil {
		// Certificate was added but we can't read it back - still success
		fmt.Printf("✓ Certificate '%s' added successfully\n", certName)
		return nil
	}

	// Find the certificate we just added
	for _, cert := range certs {
		if cert.Name == certName {
			fmt.Printf("✓ Certificate '%s' added successfully\n", certName)
			fmt.Printf("  Subject:     %s\n", cert.Subject)
			fmt.Printf("  Fingerprint: %s\n", cert.Fingerprint)
			fmt.Printf("  Expires:     %s\n", cert.Expires.Format("2006-01-02 15:04:05 MST"))
			fmt.Printf("  Path:        %s\n", cert.Path)
			fmt.Printf("\n")
			fmt.Printf("Combined bundle rebuilt: %s\n", store.CombinedBundlePath())
			return nil
		}
	}

	fmt.Printf("✓ Certificate '%s' added successfully\n", certName)
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
			fmt.Println("No expired certificates found")
		} else {
			fmt.Println("No user certificates in store")
			fmt.Println("\nAdd certificates with: verifi cert add <path> --name <name>")
		}
		return nil
	}

	// JSON output
	if certJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(certs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to encode JSON: %v\n", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
		return nil
	}

	// Table output
	fmt.Printf("User Certificates (%d)\n\n", len(certs))
	fmt.Printf("%-20s %-40s %-20s %s\n", "NAME", "SUBJECT", "EXPIRES", "STATUS")
	fmt.Printf("%s\n", repeatString("-", 120))

	now := time.Now()
	for _, cert := range certs {
		// Truncate subject if too long
		subject := cert.Subject
		if len(subject) > 40 {
			subject = subject[:37] + "..."
		}

		// Determine status
		status := "Valid"
		if now.After(cert.Expires) {
			status = "EXPIRED"
		}

		expiresStr := cert.Expires.Format("2006-01-02 15:04")

		fmt.Printf("%-20s %-40s %-20s %s\n", cert.Name, subject, expiresStr, status)
	}

	return nil
}

// repeatString repeats a string n times.
func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
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

	fmt.Printf("Removing certificate '%s'...\n", name)

	if err := store.RemoveCert(ctx, name); err != nil {
		// Check for specific error types
		if verifierrors.IsError(err, verifierrors.ErrCertNotFound) {
			fmt.Fprintf(os.Stderr, "Error: Certificate '%s' not found\n", name)
			fmt.Fprintf(os.Stderr, "Use 'verifi cert list' to see available certificates\n")
			os.Exit(verifierrors.ExitCertError)
		}

		fmt.Fprintf(os.Stderr, "Error: Failed to remove certificate: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	fmt.Printf("✓ Certificate '%s' removed successfully\n", name)
	fmt.Printf("\nCombined bundle rebuilt: %s\n", store.CombinedBundlePath())

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
			fmt.Fprintf(os.Stderr, "Error: Certificate '%s' not found\n", name)
			fmt.Fprintf(os.Stderr, "Use 'verifi cert list' to see available certificates\n")
			os.Exit(verifierrors.ExitCertError)
		}

		fmt.Fprintf(os.Stderr, "Error: Failed to get certificate info: %v\n", err)
		os.Exit(verifierrors.ExitGeneralError)
	}

	// JSON output
	if certJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(info); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to encode JSON: %v\n", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
		return nil
	}

	// Human-readable output
	fmt.Printf("Certificate: %s\n", info.Name)
	fmt.Printf("%s\n\n", repeatString("=", 40))

	fmt.Printf("Subject:     %s\n", info.Subject)
	fmt.Printf("Fingerprint: %s\n", info.Fingerprint)
	fmt.Printf("Expires:     %s\n", info.Expires.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Added:       %s\n", info.Added.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Path:        %s\n", info.Path)

	// Check if expired
	now := time.Now()
	if now.After(info.Expires) {
		fmt.Printf("\nStatus:      EXPIRED\n")
	} else {
		daysUntilExpiry := int(time.Until(info.Expires).Hours() / 24)
		fmt.Printf("\nStatus:      Valid (%d days until expiry)\n", daysUntilExpiry)
	}

	return nil
}
