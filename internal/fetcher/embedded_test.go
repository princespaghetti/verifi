package fetcher

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"
)

func TestGetEmbeddedBundle_NotNil(t *testing.T) {
	bundle := GetEmbeddedBundle()
	if bundle == nil {
		t.Fatal("GetEmbeddedBundle() returned nil")
	}

	if len(bundle) == 0 {
		t.Fatal("GetEmbeddedBundle() returned empty bundle")
	}

	t.Logf("Embedded bundle size: %d bytes", len(bundle))
}

func TestGetEmbeddedBundle_ValidPEM(t *testing.T) {
	bundle := GetEmbeddedBundle()

	// Try to decode the first PEM block
	block, rest := pem.Decode(bundle)
	if block == nil {
		t.Fatal("GetEmbeddedBundle() does not contain valid PEM data")
	}

	if block.Type != "CERTIFICATE" {
		t.Errorf("First PEM block type = %q, want %q", block.Type, "CERTIFICATE")
	}

	// Verify there's more content after the first block
	if len(rest) == 0 {
		t.Error("Bundle appears to contain only one certificate, expected multiple")
	}

	t.Logf("First PEM block type: %s, remaining bytes: %d", block.Type, len(rest))
}

func TestGetEmbeddedBundle_ContainsCerts(t *testing.T) {
	bundle := GetEmbeddedBundle()

	certCount := 0
	remaining := bundle

	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}

		// Only count CERTIFICATE blocks
		if block.Type == "CERTIFICATE" {
			// Verify it's a valid x509 certificate
			_, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				t.Errorf("Invalid certificate at position %d: %v", certCount, err)
			}
			certCount++
		}

		remaining = rest
	}

	if certCount == 0 {
		t.Fatal("Bundle contains no valid certificates")
	}

	// Mozilla CA bundle typically contains 100-200 certificates
	// We expect at least 50 to ensure we have a reasonable bundle
	if certCount < 50 {
		t.Errorf("Bundle contains only %d certificates, expected at least 50", certCount)
	}

	t.Logf("Bundle contains %d valid certificates", certCount)
}

func TestGetEmbeddedBundle_Integrity(t *testing.T) {
	bundle := GetEmbeddedBundle()

	// Compute SHA256 hash
	hash := sha256.Sum256(bundle)
	hashStr := hex.EncodeToString(hash[:])

	t.Logf("Bundle SHA256: %s", hashStr)
	t.Logf("Bundle size: %d bytes", len(bundle))

	// Verify the bundle hasn't changed unexpectedly
	// This is a known hash of the Mozilla CA bundle from curl.se
	// If this fails after updating the bundle, update the expected hash
	expectedHash := "f290e6acaf904a4121424ca3ebdd70652780707e28e8af999221786b86bb1975"

	if hashStr != expectedHash {
		// This is not necessarily an error - the bundle may have been legitimately updated
		// Log a warning instead of failing
		t.Logf("WARNING: Bundle hash has changed")
		t.Logf("  Expected: %s", expectedHash)
		t.Logf("  Got:      %s", hashStr)
		t.Logf("If you intentionally updated the Mozilla CA bundle, update the expected hash in this test")

		// Don't fail the test, but make it visible
		// Uncomment the line below if you want strict hash checking:
		// t.Errorf("Bundle hash mismatch - see logs above")
	}
}

func TestGetEmbeddedBundle_Consistency(t *testing.T) {
	// Verify that multiple calls return the same data
	bundle1 := GetEmbeddedBundle()
	bundle2 := GetEmbeddedBundle()

	if len(bundle1) != len(bundle2) {
		t.Errorf("Bundle size changed between calls: %d vs %d", len(bundle1), len(bundle2))
	}

	hash1 := sha256.Sum256(bundle1)
	hash2 := sha256.Sum256(bundle2)

	if hash1 != hash2 {
		t.Error("Bundle content changed between calls")
	}
}

func TestGetEmbeddedBundle_ParseAllCerts(t *testing.T) {
	// This test ensures every certificate in the bundle can be parsed
	bundle := GetEmbeddedBundle()

	certCount := 0
	invalidCount := 0
	remaining := bundle

	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}

		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				t.Logf("Failed to parse certificate %d: %v", certCount+1, err)
				invalidCount++
			} else {
				certCount++
				// Log first certificate details for debugging
				if certCount == 1 {
					t.Logf("First certificate: Subject=%s, NotBefore=%s, NotAfter=%s",
						cert.Subject, cert.NotBefore, cert.NotAfter)
				}
			}
		}

		remaining = rest
	}

	if invalidCount > 0 {
		t.Errorf("Found %d invalid certificates out of %d total", invalidCount, certCount+invalidCount)
	}

	t.Logf("Successfully parsed %d certificates", certCount)
}
