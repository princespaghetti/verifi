package certstore

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

// CertMetadata contains extracted certificate information
type CertMetadata struct {
	Subject     string
	Fingerprint string
	Expires     time.Time
}

// ValidateCert validates a PEM-encoded certificate and extracts metadata.
// If force is false, expired certificates will return an error.
// If force is true, expired certificates are allowed but still validated for format.
func ValidateCert(data []byte, force bool) (*x509.Certificate, *CertMetadata, error) {
	// Decode PEM block
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, &verifierrors.VerifiError{
			Op:  "validate certificate",
			Err: verifierrors.ErrInvalidPEM,
		}
	}

	// Only accept CERTIFICATE blocks
	if block.Type != "CERTIFICATE" {
		return nil, nil, &verifierrors.VerifiError{
			Op:  "validate certificate",
			Err: fmt.Errorf("invalid PEM block type: %s (expected CERTIFICATE)", block.Type),
		}
	}

	// Parse x509 certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, &verifierrors.VerifiError{
			Op:  "parse certificate",
			Err: fmt.Errorf("invalid x509 certificate: %w", err),
		}
	}

	// Check expiry unless force is enabled
	now := time.Now()
	if !force && now.After(cert.NotAfter) {
		return nil, nil, &verifierrors.VerifiError{
			Op:  "validate certificate",
			Err: verifierrors.ErrCertExpired,
		}
	}

	// Compute SHA256 fingerprint of the DER-encoded certificate
	hash := sha256.Sum256(cert.Raw)
	fingerprint := "sha256:" + hex.EncodeToString(hash[:])

	// Extract metadata
	metadata := &CertMetadata{
		Subject:     cert.Subject.String(),
		Fingerprint: fingerprint,
		Expires:     cert.NotAfter,
	}

	return cert, metadata, nil
}
