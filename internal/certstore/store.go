package certstore

import (
	"context"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	verifierrors "github.com/princespaghetti/verifi/internal/errors"
	"github.com/princespaghetti/verifi/internal/fetcher"
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
			return verifierrors.ErrStoreAlreadyInit
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
	certCount := fetcher.CountCertificates(embeddedBundle)

	// Create initial metadata
	metadata := NewMetadata()
	metadata.MozillaBundle = BundleInfo{
		Generated: time.Now(),
		SHA256:    fetcher.ComputeSHA256(embeddedBundle),
		CertCount: certCount,
		Source:    "embedded",
	}

	// Create combined bundle (initially just the Mozilla bundle)
	if err := s.RebuildBundle(ctx, metadata); err != nil {
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

// RebuildBundle rebuilds the combined certificate bundle from Mozilla bundle and user certs.
// It should be called within an UpdateMetadata callback to ensure proper locking.
func (s *Store) RebuildBundle(ctx context.Context, metadata *Metadata) error {
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

	// Append user certs
	userCerts, err := s.readUserCerts(ctx)
	if err != nil {
		return err
	}

	// Concatenate user certificates to the bundle
	for _, certData := range userCerts {
		combined = append(combined, certData...)
	}

	// Write to temp file
	if err := s.fs.WriteFile(tempPath, combined, 0644); err != nil {
		return &verifierrors.VerifiError{
			Op:   "write temp bundle",
			Path: tempPath,
			Err:  err,
		}
	}

	// Atomic rename (os.Rename is atomic on POSIX systems)
	if err := s.fs.Rename(tempPath, bundlePath); err != nil {
		_ = s.fs.Remove(tempPath)
		return &verifierrors.VerifiError{
			Op:   "rename bundle",
			Path: bundlePath,
			Err:  err,
		}
	}

	// Update metadata - include sources based on what's in the bundle
	sources := []string{"mozilla"}
	if len(userCerts) > 0 {
		sources = append(sources, "user")
	}

	metadata.CombinedBundle = BundleInfo{
		Generated: time.Now(),
		SHA256:    fetcher.ComputeSHA256(combined),
		CertCount: fetcher.CountCertificates(combined),
		Sources:   sources,
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

// AddCert adds a certificate to the user certificate store.
// The certificate is validated before being added. If force is true, expired certificates are allowed.
func (s *Store) AddCert(ctx context.Context, certPath, name string, force bool) error {
	// Check if store is initialized
	if !s.IsInitialized() {
		return &verifierrors.VerifiError{
			Op:  "add certificate",
			Err: verifierrors.ErrStoreNotInit,
		}
	}

	// Validate certificate name (no path separators allowed)
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return &verifierrors.VerifiError{
			Op:  "add certificate",
			Err: fmt.Errorf("certificate name must not contain path separators or '..'"),
		}
	}

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Read certificate file
	certData, err := s.fs.ReadFile(certPath)
	if err != nil {
		return &verifierrors.VerifiError{
			Op:   "read certificate",
			Path: certPath,
			Err:  err,
		}
	}

	// Validate certificate
	_, metadata, err := ValidateCert(certData, force)
	if err != nil {
		return err
	}

	// Check context again before writing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Write certificate to user directory with atomic rename
	destPath := s.userCertPath(name)
	tempPath := destPath + ".tmp"

	if err := s.fs.WriteFile(tempPath, certData, 0644); err != nil {
		return &verifierrors.VerifiError{
			Op:   "write certificate",
			Path: tempPath,
			Err:  err,
		}
	}

	if err := s.fs.Rename(tempPath, destPath); err != nil {
		_ = s.fs.Remove(tempPath)
		return &verifierrors.VerifiError{
			Op:   "rename certificate",
			Path: destPath,
			Err:  err,
		}
	}

	// Update metadata with file locking
	updateErr := s.UpdateMetadata(ctx, func(md *Metadata) error {
		// Check if certificate with this name already exists
		for i, existing := range md.UserCerts {
			if existing.Name == name {
				// Replace existing certificate
				md.UserCerts[i] = UserCertInfo{
					Name:        name,
					Path:        "user/" + name + ".pem",
					Added:       time.Now(),
					Fingerprint: metadata.Fingerprint,
					Subject:     metadata.Subject,
					Expires:     metadata.Expires,
				}
				return nil
			}
		}

		// Add new certificate
		md.UserCerts = append(md.UserCerts, UserCertInfo{
			Name:        name,
			Path:        "user/" + name + ".pem",
			Added:       time.Now(),
			Fingerprint: metadata.Fingerprint,
			Subject:     metadata.Subject,
			Expires:     metadata.Expires,
		})

		return nil
	})

	if updateErr != nil {
		// Rollback: remove the certificate file
		_ = s.fs.Remove(destPath)
		return updateErr
	}

	// Rebuild the combined bundle with the new certificate
	// We need to do this outside the UpdateMetadata function to avoid nesting locks
	rebuildErr := s.UpdateMetadata(ctx, func(md *Metadata) error {
		return s.RebuildBundle(ctx, md)
	})

	if rebuildErr != nil {
		return &verifierrors.VerifiError{
			Op:  "rebuild bundle after adding certificate",
			Err: rebuildErr,
		}
	}

	return nil
}

// ListCerts returns the list of user certificates from metadata.
func (s *Store) ListCerts() ([]UserCertInfo, error) {
	if !s.IsInitialized() {
		return nil, &verifierrors.VerifiError{
			Op:  "list certificates",
			Err: verifierrors.ErrStoreNotInit,
		}
	}

	metadata, err := s.readMetadata()
	if err != nil {
		return nil, err
	}

	return metadata.UserCerts, nil
}

// GetMetadata returns the current metadata from the store.
// Returns an error if the store is not initialized.
func (s *Store) GetMetadata() (*Metadata, error) {
	if !s.IsInitialized() {
		return nil, &verifierrors.VerifiError{
			Op:  "get metadata",
			Err: verifierrors.ErrStoreNotInit,
		}
	}

	return s.readMetadata()
}

// GetCertInfo retrieves detailed information about a specific user certificate by name.
// Returns ErrCertNotFound if the certificate doesn't exist.
func (s *Store) GetCertInfo(name string) (*UserCertInfo, error) {
	if !s.IsInitialized() {
		return nil, &verifierrors.VerifiError{
			Op:  "get certificate info",
			Err: verifierrors.ErrStoreNotInit,
		}
	}

	metadata, err := s.readMetadata()
	if err != nil {
		return nil, err
	}

	// Find the certificate by name
	for _, cert := range metadata.UserCerts {
		if cert.Name == name {
			return &cert, nil
		}
	}

	return nil, &verifierrors.VerifiError{
		Op:   "get certificate info",
		Path: name,
		Err:  verifierrors.ErrCertNotFound,
	}
}

// RemoveCert removes a user certificate by name.
// The certificate file is deleted and the combined bundle is rebuilt.
func (s *Store) RemoveCert(ctx context.Context, name string) error {
	if !s.IsInitialized() {
		return &verifierrors.VerifiError{
			Op:  "remove certificate",
			Err: verifierrors.ErrStoreNotInit,
		}
	}

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Update metadata with file locking
	updateErr := s.UpdateMetadata(ctx, func(md *Metadata) error {
		// Find and remove the certificate from metadata
		found := false
		newCerts := make([]UserCertInfo, 0, len(md.UserCerts))
		for _, cert := range md.UserCerts {
			if cert.Name == name {
				found = true
				continue
			}
			newCerts = append(newCerts, cert)
		}

		if !found {
			return &verifierrors.VerifiError{
				Op:   "remove certificate",
				Path: name,
				Err:  verifierrors.ErrCertNotFound,
			}
		}

		md.UserCerts = newCerts
		return nil
	})

	if updateErr != nil {
		return updateErr
	}

	// Remove the physical certificate file
	certPath := s.userCertPath(name)
	_ = s.fs.Remove(certPath) // Ignore error - file may not exist, which is okay

	// Rebuild the combined bundle
	rebuildErr := s.UpdateMetadata(ctx, func(md *Metadata) error {
		return s.RebuildBundle(ctx, md)
	})

	if rebuildErr != nil {
		return &verifierrors.VerifiError{
			Op:  "rebuild bundle after removing certificate",
			Err: rebuildErr,
		}
	}

	return nil
}

// ResetMozillaBundle resets the Mozilla CA bundle to the embedded version.
// The combined bundle is rebuilt after the reset.
func (s *Store) ResetMozillaBundle(ctx context.Context) error {
	if !s.IsInitialized() {
		return &verifierrors.VerifiError{
			Op:  "reset mozilla bundle",
			Err: verifierrors.ErrStoreNotInit,
		}
	}

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Get embedded bundle
	embeddedBundle := fetcher.GetEmbeddedBundle()

	// Write to temp file then atomic rename
	mozillaPath := s.mozillaBundlePath()
	tempPath := mozillaPath + ".tmp"

	if err := s.fs.WriteFile(tempPath, embeddedBundle, 0644); err != nil {
		return &verifierrors.VerifiError{
			Op:   "write mozilla bundle temp file",
			Path: tempPath,
			Err:  err,
		}
	}

	if err := s.fs.Rename(tempPath, mozillaPath); err != nil {
		_ = s.fs.Remove(tempPath)
		return &verifierrors.VerifiError{
			Op:   "rename mozilla bundle",
			Path: mozillaPath,
			Err:  err,
		}
	}

	// Update metadata with locking
	updateErr := s.UpdateMetadata(ctx, func(md *Metadata) error {
		certCount := fetcher.CountCertificates(embeddedBundle)
		md.MozillaBundle = BundleInfo{
			Generated: time.Now(),
			SHA256:    fetcher.ComputeSHA256(embeddedBundle),
			CertCount: certCount,
			Source:    "embedded",
			Version:   "", // No version for embedded bundle
		}

		// Rebuild combined bundle with reset Mozilla bundle
		return s.RebuildBundle(ctx, md)
	})

	if updateErr != nil {
		return updateErr
	}

	return nil
}
