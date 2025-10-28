package certstore

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os/user"
	"path/filepath"
	"time"

	"github.com/princespaghetti/verifi/internal/fetcher"

	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

// Store represents the certificate store and provides operations for managing certificates.
type Store struct {
	basePath string
	fs       FileSystem
}

// NewStore creates a new Store instance with the given base path.
// If basePath is empty, it defaults to ~/.verifi
func NewStore(basePath string) (*Store, error) {
	if basePath == "" {
		usr, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("get user home directory: %w", err)
		}
		basePath = filepath.Join(usr.HomeDir, ".verifi")
	}

	return &Store{
		basePath: basePath,
		fs:       &OSFileSystem{},
	}, nil
}

// Init initializes the certificate store by creating the directory structure
// and extracting the embedded Mozilla CA bundle.
func (s *Store) Init(ctx context.Context, force bool) error {
	// Check if already initialized
	if !force {
		if _, err := s.fs.Stat(s.metadataPath()); err == nil {
			return verifierrors.ErrStoreNotInit // Actually means already initialized
		}
	}

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Create directory structure
	if err := s.createDirectories(); err != nil {
		return err
	}

	// Write embedded Mozilla bundle
	mozillaPath := s.mozillaBundlePath()
	embeddedBundle := fetcher.GetEmbeddedBundle()
	if err := s.fs.WriteFile(mozillaPath, embeddedBundle, 0644); err != nil {
		return &verifierrors.VerifiError{
			Op:   "write mozilla bundle",
			Path: mozillaPath,
			Err:  err,
		}
	}

	// Count certificates in Mozilla bundle
	certCount := countCertificates(embeddedBundle)

	// Create initial metadata
	metadata := NewMetadata()
	metadata.MozillaBundle = BundleInfo{
		Generated: time.Now(),
		SHA256:    computeSHA256(embeddedBundle),
		CertCount: certCount,
		Source:    "embedded",
	}

	// Create combined bundle (initially just the Mozilla bundle)
	if err := s.rebuildBundle(ctx, metadata); err != nil {
		return err
	}

	// Write metadata
	if err := s.writeMetadata(metadata); err != nil {
		return err
	}

	return nil
}

// IsInitialized returns true if the store has been initialized.
func (s *Store) IsInitialized() bool {
	_, err := s.fs.Stat(s.metadataPath())
	return err == nil
}

// BasePath returns the base path of the store.
func (s *Store) BasePath() string {
	return s.basePath
}

// CombinedBundlePath returns the path to the combined certificate bundle.
func (s *Store) CombinedBundlePath() string {
	return filepath.Join(s.basePath, "certs", "bundles", "combined-bundle.pem")
}

// createDirectories creates the directory structure for the certificate store.
func (s *Store) createDirectories() error {
	dirs := []string{
		filepath.Join(s.basePath, "certs", "user"),
		filepath.Join(s.basePath, "certs", "bundles"),
		filepath.Join(s.basePath, "logs"),
	}

	for _, dir := range dirs {
		if err := s.fs.MkdirAll(dir, 0755); err != nil {
			return &verifierrors.VerifiError{
				Op:   "create directory",
				Path: dir,
				Err:  err,
			}
		}
	}

	return nil
}

// rebuildBundle rebuilds the combined certificate bundle from Mozilla bundle and user certs.
func (s *Store) rebuildBundle(ctx context.Context, metadata *Metadata) error {
	bundlePath := s.CombinedBundlePath()
	tempPath := bundlePath + ".tmp"

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Read Mozilla bundle
	mozillaData, err := s.fs.ReadFile(s.mozillaBundlePath())
	if err != nil {
		return &verifierrors.VerifiError{
			Op:   "read mozilla bundle",
			Path: s.mozillaBundlePath(),
			Err:  err,
		}
	}

	// Start with Mozilla bundle
	combined := mozillaData

	// Append user certs (Phase 3 will implement this)
	// For now, combined bundle is just the Mozilla bundle

	// Write to temp file
	if err := s.fs.WriteFile(tempPath, combined, 0644); err != nil {
		return &verifierrors.VerifiError{
			Op:   "write temp bundle",
			Path: tempPath,
			Err:  err,
		}
	}

	// Atomic rename (simulated by writing directly for now)
	if err := s.fs.WriteFile(bundlePath, combined, 0644); err != nil {
		s.fs.Remove(tempPath)
		return &verifierrors.VerifiError{
			Op:   "write combined bundle",
			Path: bundlePath,
			Err:  err,
		}
	}

	// Clean up temp file
	s.fs.Remove(tempPath)

	// Update metadata
	metadata.CombinedBundle = BundleInfo{
		Generated: time.Now(),
		SHA256:    computeSHA256(combined),
		CertCount: countCertificates(combined),
		Sources:   []string{"mozilla"},
	}

	return nil
}

// metadataPath returns the path to the metadata.json file.
func (s *Store) metadataPath() string {
	return filepath.Join(s.basePath, "certs", "metadata.json")
}

// mozillaBundlePath returns the path to the Mozilla CA bundle.
func (s *Store) mozillaBundlePath() string {
	return filepath.Join(s.basePath, "certs", "bundles", "mozilla-ca-bundle.pem")
}

// countCertificates counts the number of certificates in a PEM bundle.
func countCertificates(pemData []byte) int {
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
