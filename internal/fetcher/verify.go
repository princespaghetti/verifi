package fetcher

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"regexp"
	"time"
)

const (
	// MinCertCount is the minimum number of certificates expected in a valid Mozilla bundle.
	MinCertCount = 100

	// MaxDegradationPercent is the maximum allowed degradation in cert count (20%).
	MaxDegradationPercent = 20
)

// BundleVerificationResult contains the results of bundle verification.
type BundleVerificationResult struct {
	CertCount       int
	MozillaDate     time.Time
	IsValid         bool
	Warning         string
	HasDateInHeader bool
}

// VerifyBundle verifies that a downloaded Mozilla CA bundle is valid.
// It checks:
// - Valid PEM format
// - Contains >= MinCertCount certificates
// - Warns if cert count dropped significantly from current count
func VerifyBundle(bundleData []byte, currentCertCount int) (*BundleVerificationResult, error) {
	result := &BundleVerificationResult{}

	// Count certificates in bundle
	certCount := CountCertificates(bundleData)
	result.CertCount = certCount

	// Check minimum cert count
	if certCount < MinCertCount {
		result.IsValid = false
		return result, fmt.Errorf("bundle contains only %d certificates, expected at least %d", certCount, MinCertCount)
	}

	// Try to parse Mozilla date from header
	if mozillaDate, found := parseMozillaDate(bundleData); found {
		result.MozillaDate = mozillaDate
		result.HasDateInHeader = true
	}

	// Check for degradation if we have a current cert count to compare
	if currentCertCount > 0 {
		degradation := float64(currentCertCount-certCount) / float64(currentCertCount) * 100
		if degradation > MaxDegradationPercent {
			result.Warning = fmt.Sprintf("New bundle has %d fewer certificates (%.1f%% decrease). This may indicate a problem.",
				currentCertCount-certCount, degradation)
		}
	}

	result.IsValid = true
	return result, nil
}

// CountCertificates counts the number of valid certificates in a PEM bundle.
func CountCertificates(pemData []byte) int {
	count := 0
	remaining := pemData

	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}

		// Only count CERTIFICATE blocks
		if block.Type == "CERTIFICATE" {
			// Try to parse to verify it's a valid certificate
			if _, err := x509.ParseCertificate(block.Bytes); err == nil {
				count++
			}
		}

		remaining = rest
	}

	return count
}

// parseMozillaDate attempts to extract the Mozilla bundle date from the header comments.
// The header typically contains a line like:
// ## Certificate data from Mozilla as of: Tue Sep  9 03:12:01 2025 GMT
func parseMozillaDate(bundleData []byte) (time.Time, bool) {
	// Look for the Mozilla date header in the first 1KB of the file
	header := bundleData
	if len(header) > 1024 {
		header = header[:1024]
	}

	// Regular expression to match the Mozilla date format
	// Example: "Tue Sep  9 03:12:01 2025 GMT"
	dateRegex := regexp.MustCompile(`Certificate data from Mozilla as of:\s+([A-Za-z]{3}\s+[A-Za-z]{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s+\d{4}\s+GMT)`)

	matches := dateRegex.FindSubmatch(header)
	if len(matches) < 2 {
		return time.Time{}, false
	}

	dateStr := string(matches[1])

	// Parse the date string
	// Format: "Mon Jan 2 15:04:05 2006 MST"
	parsedDate, err := time.Parse("Mon Jan _2 15:04:05 2006 MST", dateStr)
	if err != nil {
		return time.Time{}, false
	}

	return parsedDate, true
}

// ExtractMozillaDateString extracts the Mozilla date as a simple YYYY-MM-DD string.
// This is useful for the Version field in metadata.
func ExtractMozillaDateString(bundleData []byte) string {
	if date, found := parseMozillaDate(bundleData); found {
		return date.Format("2006-01-02")
	}
	return ""
}

// ValidatePEMFormat checks if the data contains valid PEM blocks.
func ValidatePEMFormat(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	// Check if it starts with a PEM header
	limit := 100
	if len(data) < limit {
		limit = len(data)
	}
	if !bytes.Contains(data[:limit], []byte("-----BEGIN")) {
		return fmt.Errorf("data does not appear to be in PEM format")
	}

	// Try to decode at least one block
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("failed to decode PEM data")
	}

	return nil
}

// ComputeSHA256 computes the SHA256 hash of data and returns it as a hex string.
func ComputeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
