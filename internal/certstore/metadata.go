package certstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

// Metadata tracks the certificate store state, including bundle information
// and user-added certificates.
type Metadata struct {
	Version        string         `json:"version"`
	CombinedBundle BundleInfo     `json:"combined_bundle"`
	MozillaBundle  BundleInfo     `json:"mozilla_bundle"`
	UserCerts      []UserCertInfo `json:"user_certs"`
}

// BundleInfo contains information about a certificate bundle.
type BundleInfo struct {
	Generated time.Time `json:"generated"`
	SHA256    string    `json:"sha256"`
	CertCount int       `json:"cert_count"`
	Sources   []string  `json:"sources,omitempty"`
	Version   string    `json:"version,omitempty"`
	Source    string    `json:"source,omitempty"`
}

// UserCertInfo contains information about a user-added certificate.
type UserCertInfo struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Added       time.Time `json:"added"`
	Fingerprint string    `json:"fingerprint"`
	Subject     string    `json:"subject"`
	Expires     time.Time `json:"expires"`
}

const (
	// currentSchemaVersion is the current metadata schema version.
	currentSchemaVersion = "1"
)

// NewMetadata creates a new metadata instance with default values.
func NewMetadata() *Metadata {
	return &Metadata{
		Version:   currentSchemaVersion,
		UserCerts: []UserCertInfo{},
	}
}

// readMetadata reads and parses the metadata.json file.
func (s *Store) readMetadata() (*Metadata, error) {
	data, err := s.fs.ReadFile(s.metadataPath())
	if err != nil {
		return nil, &verifierrors.VerifiError{
			Op:   "read metadata",
			Path: s.metadataPath(),
			Err:  err,
		}
	}

	var m Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, &verifierrors.VerifiError{
			Op:   "parse metadata",
			Path: s.metadataPath(),
			Err:  err,
		}
	}

	// Migrate if needed
	if m.Version != currentSchemaVersion {
		if err := migrateMetadata(&m); err != nil {
			return nil, fmt.Errorf("migrate metadata: %w", err)
		}
	}

	return &m, nil
}

// writeMetadata writes the metadata to metadata.json using atomic rename.
func (s *Store) writeMetadata(m *Metadata) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return &verifierrors.VerifiError{
			Op:  "marshal metadata",
			Err: err,
		}
	}

	// Write to temp file
	tempPath := s.metadataPath() + ".tmp"
	if err := s.fs.WriteFile(tempPath, data, 0644); err != nil {
		return &verifierrors.VerifiError{
			Op:   "write temp metadata",
			Path: tempPath,
			Err:  err,
		}
	}

	// Atomic rename (os.Rename is atomic on POSIX systems)
	if err := s.fs.Rename(tempPath, s.metadataPath()); err != nil {
		_ = s.fs.Remove(tempPath)
		return &verifierrors.VerifiError{
			Op:   "rename metadata",
			Path: s.metadataPath(),
			Err:  err,
		}
	}

	return nil
}

// UpdateMetadata updates the metadata using the provided function.
// It acquires a lock before reading/writing to ensure concurrent safety.
func (s *Store) UpdateMetadata(ctx context.Context, fn func(*Metadata) error) error {
	lock := NewFileLock(s.metadataPath())
	if err := lock.Lock(ctx); err != nil {
		return fmt.Errorf("failed to lock metadata: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	// Read current metadata
	metadata, err := s.readMetadata()
	if err != nil {
		return err
	}

	// Apply changes
	if err := fn(metadata); err != nil {
		return err
	}

	// Write updated metadata
	return s.writeMetadata(metadata)
}

// computeSHA256 computes the SHA256 hash of the given data.
func computeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// migrateMetadata handles schema version migrations.
func migrateMetadata(m *Metadata) error {
	// Currently only v1 exists, so no migrations needed yet
	// Future versions would add migration logic here
	m.Version = currentSchemaVersion
	return nil
}
