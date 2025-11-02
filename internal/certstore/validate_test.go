package certstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

// Helper function to generate a test certificate
func generateTestCert(t *testing.T, subject string, notBefore, notAfter time.Time) []byte {
	t.Helper()

	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: subject,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return certPEM
}

// Test constants
const (
	invalidPEM = `This is not a valid PEM certificate`

	nonCertPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAz8JKWkE7rLGxqrLqWDHj8qVzqNPVp3E0Qr0qPLc2x0tH7PGG
-----END RSA PRIVATE KEY-----`
)

func TestValidateCert(t *testing.T) {
	// Generate test certificates
	validCert := generateTestCert(t, "Test CA", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))
	expiredCert := generateTestCert(t, "Expired CA", time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))

	tests := []struct {
		name      string
		certPEM   []byte
		force     bool
		wantErr   bool
		errType   error
		checkMeta bool // whether to check metadata fields
	}{
		{
			name:      "valid certificate",
			certPEM:   validCert,
			force:     false,
			wantErr:   false,
			checkMeta: true,
		},
		{
			name:    "expired certificate without force",
			certPEM: expiredCert,
			force:   false,
			wantErr: true,
			errType: verifierrors.ErrCertExpired,
		},
		{
			name:      "expired certificate with force",
			certPEM:   expiredCert,
			force:     true,
			wantErr:   false,
			checkMeta: true,
		},
		{
			name:    "invalid PEM format",
			certPEM: []byte(invalidPEM),
			force:   false,
			wantErr: true,
			errType: verifierrors.ErrInvalidPEM,
		},
		{
			name:    "non-certificate PEM block",
			certPEM: []byte(nonCertPEM),
			force:   false,
			wantErr: true,
		},
		{
			name:    "empty input",
			certPEM: []byte(""),
			force:   false,
			wantErr: true,
			errType: verifierrors.ErrInvalidPEM,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, metadata, err := ValidateCert(tt.certPEM, tt.force)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateCert() expected error but got nil")
					return
				}

				// Check for specific error type if specified
				if tt.errType != nil {
					if !errors.Is(err, tt.errType) {
						t.Errorf("ValidateCert() error = %v, want error type %v", err, tt.errType)
					}
				}

				// Cert and metadata should be nil on error
				if cert != nil {
					t.Errorf("ValidateCert() cert should be nil on error, got %v", cert)
				}
				if metadata != nil {
					t.Errorf("ValidateCert() metadata should be nil on error, got %v", metadata)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateCert() unexpected error = %v", err)
					return
				}

				// Cert and metadata should not be nil on success
				if cert == nil {
					t.Errorf("ValidateCert() cert should not be nil on success")
					return
				}
				if metadata == nil {
					t.Errorf("ValidateCert() metadata should not be nil on success")
					return
				}

				// Check metadata fields if requested
				if tt.checkMeta {
					// Subject should not be empty
					if metadata.Subject == "" {
						t.Errorf("ValidateCert() metadata.Subject is empty")
					}

					// Fingerprint should start with "sha256:"
					if !strings.HasPrefix(metadata.Fingerprint, "sha256:") {
						t.Errorf("ValidateCert() metadata.Fingerprint = %v, want prefix 'sha256:'", metadata.Fingerprint)
					}

					// Fingerprint should be 71 characters (sha256: + 64 hex chars)
					if len(metadata.Fingerprint) != 71 {
						t.Errorf("ValidateCert() metadata.Fingerprint length = %d, want 71", len(metadata.Fingerprint))
					}

					// Expires should not be zero time
					if metadata.Expires.IsZero() {
						t.Errorf("ValidateCert() metadata.Expires is zero")
					}
				}
			}
		})
	}
}

func TestValidateCert_Fingerprint(t *testing.T) {
	// Test that the same certificate always produces the same fingerprint
	validCert := generateTestCert(t, "Test CA", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))

	_, meta1, err := ValidateCert(validCert, false)
	if err != nil {
		t.Fatalf("ValidateCert() error = %v", err)
	}

	_, meta2, err := ValidateCert(validCert, false)
	if err != nil {
		t.Fatalf("ValidateCert() error = %v", err)
	}

	if meta1.Fingerprint != meta2.Fingerprint {
		t.Errorf("ValidateCert() fingerprints should match, got %v and %v", meta1.Fingerprint, meta2.Fingerprint)
	}
}
