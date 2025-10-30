package cli

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/princespaghetti/verifi/internal/certstore"
	verifierrors "github.com/princespaghetti/verifi/internal/errors"
	"github.com/princespaghetti/verifi/internal/shell"
)

var (
	doctorVerbose bool
	doctorJSON    bool
)

// doctorCmd represents the doctor command.
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics on the certificate store",
	Long: `Run comprehensive diagnostics on the certificate store to identify issues.

Checks performed:
  - Store directory structure exists and is readable
  - Metadata file is valid JSON with correct schema
  - Mozilla bundle exists and contains valid PEM certificates
  - Combined bundle exists and contains valid PEM certificates
  - User certificates exist and are valid (not expired)
  - env.sh file exists and contains correct environment variables
  - File permissions allow read access

Use --verbose for detailed diagnostic information.
Use --json for machine-readable output.

Examples:
  verifi doctor
  verifi doctor --verbose
  verifi doctor --json`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorVerbose, "verbose", false, "Show detailed diagnostic information")
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output in JSON format")
}

// CheckResult represents the result of a single diagnostic check.
type CheckResult struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"` // "pass", "warn", "fail"
	Issues      []string `json:"issues,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// DoctorOutput represents the complete diagnostic output.
type DoctorOutput struct {
	Checks      []CheckResult `json:"checks"`
	Summary     Summary       `json:"summary"`
	OverallPass bool          `json:"overall_pass"`
}

// Summary contains counts of check results.
type Summary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Failures int `json:"failures"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
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

	// Run all diagnostic checks
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results := []CheckResult{
		checkStoreStructure(store),
		checkMetadata(store),
		checkMozillaBundle(store),
		checkCombinedBundle(store),
		checkUserCertificates(ctx, store),
		checkEnvFile(store),
		checkFilePermissions(store),
	}

	// Calculate summary
	summary := Summary{Total: len(results)}
	overallPass := true
	for _, result := range results {
		switch result.Status {
		case "pass":
			summary.Passed++
		case "warn":
			summary.Warnings++
			// Warnings don't fail the overall check
		case "fail":
			summary.Failures++
			overallPass = false
		}
	}

	// Output results
	output := DoctorOutput{
		Checks:      results,
		Summary:     summary,
		OverallPass: overallPass,
	}

	if doctorJSON {
		if err := JSON(output); err != nil {
			Error("Failed to encode JSON: %v", err)
			os.Exit(verifierrors.ExitGeneralError)
		}
	} else {
		printDoctorOutput(output)
	}

	// Exit with appropriate code
	if !overallPass {
		os.Exit(verifierrors.ExitGeneralError)
	}

	return nil
}

func printDoctorOutput(output DoctorOutput) {
	Header("Certificate Store Diagnostics")

	for _, check := range output.Checks {
		// Print status icon and check name
		statusIcon := StatusIcon(check.Status)
		fmt.Printf("%s %s\n", statusIcon, check.Name)

		// Print issues if verbose or if there are problems
		if (doctorVerbose || check.Status != "pass") && len(check.Issues) > 0 {
			for _, issue := range check.Issues {
				fmt.Printf("  - %s\n", issue)
			}
		}

		// Print suggestions for problems
		if check.Status != "pass" && len(check.Suggestions) > 0 {
			for _, suggestion := range check.Suggestions {
				fmt.Printf("  → %s\n", suggestion)
			}
		}

		EmptyLine()
	}

	// Print summary
	Subheader("Summary")
	Field("Total checks", fmt.Sprintf("%d", output.Summary.Total))
	Field("Passed", fmt.Sprintf("%d", output.Summary.Passed))
	if output.Summary.Warnings > 0 {
		Field("Warnings", fmt.Sprintf("%d", output.Summary.Warnings))
	}
	if output.Summary.Failures > 0 {
		Field("Failures", fmt.Sprintf("%d", output.Summary.Failures))
	}
	EmptyLine()

	if output.OverallPass {
		if output.Summary.Warnings > 0 {
			Info("Status: PASS (with warnings)")
		} else {
			Info("Status: PASS")
		}
	} else {
		Info("Status: FAIL")
	}
}

// checkStoreStructure verifies the store directory structure exists.
func checkStoreStructure(store *certstore.Store) CheckResult {
	result := CheckResult{
		Name:   "Store directory structure",
		Status: "pass",
	}

	basePath := store.BasePath()
	requiredDirs := []string{
		filepath.Join(basePath, "certs"),
		filepath.Join(basePath, "certs", "user"),
		filepath.Join(basePath, "certs", "bundles"),
		filepath.Join(basePath, "logs"),
	}

	for _, dir := range requiredDirs {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			result.Status = "fail"
			result.Issues = append(result.Issues, fmt.Sprintf("Directory does not exist: %s", dir))
		} else if err != nil {
			result.Status = "fail"
			result.Issues = append(result.Issues, fmt.Sprintf("Cannot access directory: %s (%v)", dir, err))
		} else if !info.IsDir() {
			result.Status = "fail"
			result.Issues = append(result.Issues, fmt.Sprintf("Path exists but is not a directory: %s", dir))
		}
	}

	if result.Status == "fail" {
		result.Suggestions = append(result.Suggestions, "Run 'verifi init --force' to recreate the store structure")
	}

	return result
}

// checkMetadata verifies the metadata file is valid.
func checkMetadata(store *certstore.Store) CheckResult {
	result := CheckResult{
		Name:   "Metadata integrity",
		Status: "pass",
	}

	metadata, err := store.GetMetadata()
	if err != nil {
		result.Status = "fail"
		result.Issues = append(result.Issues, fmt.Sprintf("Cannot read metadata: %v", err))
		result.Suggestions = append(result.Suggestions, "Run 'verifi init --force' to recreate metadata")
		return result
	}

	// Check schema version
	if metadata.Version != "1" {
		result.Status = "fail"
		result.Issues = append(result.Issues, fmt.Sprintf("Unknown metadata schema version: %s", metadata.Version))
		result.Suggestions = append(result.Suggestions, "Update verifi to the latest version")
	}

	// Check Mozilla bundle info is present
	if metadata.MozillaBundle.SHA256 == "" {
		result.Status = "fail"
		result.Issues = append(result.Issues, "Mozilla bundle info missing from metadata")
	}

	// Check combined bundle info is present
	if metadata.CombinedBundle.SHA256 == "" {
		result.Status = "fail"
		result.Issues = append(result.Issues, "Combined bundle info missing from metadata")
	}

	if result.Status == "fail" && len(result.Suggestions) == 0 {
		result.Suggestions = append(result.Suggestions, "Run 'verifi bundle reset' or 'verifi init --force'")
	}

	return result
}

// checkMozillaBundle verifies the Mozilla bundle exists and is valid.
func checkMozillaBundle(store *certstore.Store) CheckResult {
	result := CheckResult{
		Name:   "Mozilla CA bundle",
		Status: "pass",
	}

	mozillaPath := filepath.Join(store.BasePath(), "certs", "bundles", "mozilla-ca-bundle.pem")

	// Check file exists
	data, err := os.ReadFile(mozillaPath)
	if err != nil {
		result.Status = "fail"
		result.Issues = append(result.Issues, fmt.Sprintf("Cannot read Mozilla bundle: %v", err))
		result.Suggestions = append(result.Suggestions, "Run 'verifi bundle reset' to restore the embedded bundle")
		return result
	}

	// Check PEM format and count certificates
	certCount := 0
	remaining := data
	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			// Try to parse to verify it's valid
			if _, err := x509.ParseCertificate(block.Bytes); err == nil {
				certCount++
			}
		}
		remaining = rest
	}

	if certCount == 0 {
		result.Status = "fail"
		result.Issues = append(result.Issues, "No valid certificates found in Mozilla bundle")
		result.Suggestions = append(result.Suggestions, "Run 'verifi bundle reset' to restore the embedded bundle")
		return result
	}

	if certCount < 100 {
		result.Status = "warn"
		result.Issues = append(result.Issues, fmt.Sprintf("Mozilla bundle has only %d certificates (expected 100+)", certCount))
		result.Suggestions = append(result.Suggestions, "Consider running 'verifi bundle update' or 'verifi bundle reset'")
	}

	// Verify hash matches metadata
	metadata, err := store.GetMetadata()
	if err == nil {
		actualSHA := computeSHA256(data)
		if actualSHA != metadata.MozillaBundle.SHA256 {
			result.Status = "fail"
			result.Issues = append(result.Issues, "Mozilla bundle SHA256 hash mismatch")
			result.Suggestions = append(result.Suggestions, "Bundle file has been modified outside of verifi")
			result.Suggestions = append(result.Suggestions, "Run 'verifi bundle reset' to restore")
		}
	}

	return result
}

// checkCombinedBundle verifies the combined bundle exists and is valid.
func checkCombinedBundle(store *certstore.Store) CheckResult {
	result := CheckResult{
		Name:   "Combined certificate bundle",
		Status: "pass",
	}

	combinedPath := store.CombinedBundlePath()

	// Check file exists
	data, err := os.ReadFile(combinedPath)
	if err != nil {
		result.Status = "fail"
		result.Issues = append(result.Issues, fmt.Sprintf("Cannot read combined bundle: %v", err))
		result.Suggestions = append(result.Suggestions, "The combined bundle is missing or unreadable")
		result.Suggestions = append(result.Suggestions, "Run 'verifi init --force' to recreate")
		return result
	}

	// Check PEM format and count certificates
	certCount := 0
	remaining := data
	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			if _, err := x509.ParseCertificate(block.Bytes); err == nil {
				certCount++
			}
		}
		remaining = rest
	}

	if certCount == 0 {
		result.Status = "fail"
		result.Issues = append(result.Issues, "No valid certificates found in combined bundle")
		result.Suggestions = append(result.Suggestions, "Run 'verifi init --force' to recreate the bundle")
		return result
	}

	// Verify hash matches metadata
	metadata, err := store.GetMetadata()
	if err == nil {
		actualSHA := computeSHA256(data)
		if actualSHA != metadata.CombinedBundle.SHA256 {
			result.Status = "fail"
			result.Issues = append(result.Issues, "Combined bundle SHA256 hash mismatch")
			result.Suggestions = append(result.Suggestions, "Bundle file has been modified outside of verifi")
			result.Suggestions = append(result.Suggestions, "Remove and re-add user certificates, or run 'verifi init --force'")
		}
	}

	return result
}

// checkUserCertificates verifies user certificates exist and are valid.
func checkUserCertificates(ctx context.Context, store *certstore.Store) CheckResult {
	result := CheckResult{
		Name:   "User certificates",
		Status: "pass",
	}

	certs, err := store.ListCerts()
	if err != nil {
		result.Status = "warn"
		result.Issues = append(result.Issues, fmt.Sprintf("Cannot list certificates: %v", err))
		return result
	}

	if len(certs) == 0 {
		// No user certs is not an error
		result.Issues = append(result.Issues, "No user certificates in store")
		return result
	}

	now := time.Now()
	expiredCount := 0
	missingCount := 0

	for _, cert := range certs {
		certPath := filepath.Join(store.BasePath(), "certs", cert.Path)

		// Check if file exists
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			result.Status = "fail"
			result.Issues = append(result.Issues, fmt.Sprintf("Certificate file missing: %s", cert.Name))
			missingCount++
			continue
		}

		// Check if expired
		if now.After(cert.Expires) {
			result.Status = "warn"
			result.Issues = append(result.Issues, fmt.Sprintf("Certificate expired: %s (expired %s)", cert.Name, cert.Expires.Format("2006-01-02")))
			expiredCount++
		}
	}

	if missingCount > 0 {
		result.Suggestions = append(result.Suggestions, fmt.Sprintf("Remove missing certificates or restore files (%d missing)", missingCount))
	}

	if expiredCount > 0 {
		result.Suggestions = append(result.Suggestions, fmt.Sprintf("Remove or update expired certificates (%d expired)", expiredCount))
	}

	return result
}

// checkEnvFile verifies the env.sh file exists and contains correct variables.
func checkEnvFile(store *certstore.Store) CheckResult {
	result := CheckResult{
		Name:   "Environment file (env.sh)",
		Status: "pass",
	}

	envPath := shell.EnvFilePath(store.BasePath())

	// Check file exists
	data, err := os.ReadFile(envPath)
	if os.IsNotExist(err) {
		result.Status = "warn"
		result.Issues = append(result.Issues, "env.sh file does not exist")
		result.Suggestions = append(result.Suggestions, "Run 'verifi env' to generate env.sh")
		return result
	} else if err != nil {
		result.Status = "fail"
		result.Issues = append(result.Issues, fmt.Sprintf("Cannot read env.sh: %v", err))
		result.Suggestions = append(result.Suggestions, "Run 'verifi env' to regenerate env.sh")
		return result
	}

	content := string(data)

	// Check for required environment variables
	requiredVars := []string{
		"SSL_CERT_FILE",
		"REQUESTS_CA_BUNDLE",
		"NODE_EXTRA_CA_CERTS",
		"CURL_CA_BUNDLE",
		"AWS_CA_BUNDLE",
		"GIT_SSL_CAINFO",
	}

	missingVars := []string{}
	for _, varName := range requiredVars {
		if !strings.Contains(content, varName) {
			missingVars = append(missingVars, varName)
		}
	}

	if len(missingVars) > 0 {
		result.Status = "warn"
		result.Issues = append(result.Issues, fmt.Sprintf("env.sh is missing %d required variables", len(missingVars)))
		if doctorVerbose {
			result.Issues = append(result.Issues, fmt.Sprintf("Missing: %s", strings.Join(missingVars, ", ")))
		}
		result.Suggestions = append(result.Suggestions, "Run 'verifi env' to regenerate env.sh")
	}

	// Check if env.sh points to combined bundle
	combinedPath := store.CombinedBundlePath()
	if !strings.Contains(content, combinedPath) && !strings.Contains(content, filepath.ToSlash(combinedPath)) {
		result.Status = "warn"
		result.Issues = append(result.Issues, "env.sh may not point to current combined bundle")
		result.Suggestions = append(result.Suggestions, "Run 'verifi env' to regenerate env.sh")
	}

	return result
}

// checkFilePermissions verifies files are readable.
func checkFilePermissions(store *certstore.Store) CheckResult {
	result := CheckResult{
		Name:   "File permissions",
		Status: "pass",
	}

	files := []string{
		filepath.Join(store.BasePath(), "certs", "metadata.json"),
		filepath.Join(store.BasePath(), "certs", "bundles", "mozilla-ca-bundle.pem"),
		store.CombinedBundlePath(),
	}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			// File doesn't exist - will be caught by other checks
			continue
		}

		mode := info.Mode()
		if mode.Perm()&0400 == 0 {
			result.Status = "fail"
			result.Issues = append(result.Issues, fmt.Sprintf("File is not readable: %s", file))
		}
	}

	if result.Status == "fail" {
		result.Suggestions = append(result.Suggestions, "Fix file permissions: chmod 644 <file>")
	}

	return result
}
