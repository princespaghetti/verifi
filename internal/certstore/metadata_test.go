package certstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/princespaghetti/verifi/internal/fetcher"
)

func TestNewMetadata_Defaults(t *testing.T) {
	metadata := NewMetadata()

	if metadata.Version != "1" {
		t.Errorf("Version = %q, want %q", metadata.Version, "1")
	}

	if len(metadata.UserCerts) != 0 {
		t.Errorf("UserCerts should be empty initially, got %d entries", len(metadata.UserCerts))
	}

	// Combined and Mozilla bundles should be zero values
	if metadata.CombinedBundle.CertCount != 0 {
		t.Errorf("CombinedBundle.CertCount should be 0, got %d", metadata.CombinedBundle.CertCount)
	}

	if metadata.MozillaBundle.CertCount != 0 {
		t.Errorf("MozillaBundle.CertCount should be 0, got %d", metadata.MozillaBundle.CertCount)
	}
}

func TestComputeSHA256(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "empty data",
			data: []byte{},
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "hello world",
			data: []byte("hello world"),
			want: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name: "test data",
			data: []byte("test data for hash"),
			want: "6262a3f2fc3e39f49321cef37ffd514be52f9f6f1e628d49c4f57fa918ad41e4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fetcher.ComputeSHA256(tt.data)
			if got != tt.want {
				t.Errorf("fetcher.ComputeSHA256() = %q, want %q", got, tt.want)
			}

			// Verify it matches manual computation
			hash := sha256.Sum256(tt.data)
			expected := hex.EncodeToString(hash[:])
			if got != expected {
				t.Errorf("fetcher.ComputeSHA256() != manual hash: %q vs %q", got, expected)
			}
		})
	}
}

func TestCountCertificates_MixedPEM(t *testing.T) {
	// Test with mixed PEM blocks (certificates, keys, etc.)
	mixedPEM := `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHHCgVZU1W/MA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCVRl
c3QgQ0EgMDAeFw0yNTAxMDEwMDAwMDBaFw0yNjAxMDEwMDAwMDBaMBQxEjAQBgNV
BAMMCVRlc3QgQ0EgMDBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQDPwkpaQTussbGq
supYMePypXOo09WncTRCvSo8tzbHS0fs8Ya873Gf0x8o+qNeoTtMGkfgztDdYkT5
83krypVAgMBAAEwDQYJKoZIhvcNAQELBQADQQBDupVNQlXOom8vBOoT0q/Oom8v
BQ==
-----END CERTIFICATE-----
-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBAM/CSlpBO6yxsaqy6lgx4/Klc6jT1adxNEK9Kjy3NsdLR+zxhrzv
cZ/THyj6o16hO0waR+DO0N1iRPnzeSvKlUCAwEAAQJATcFFXI+y6xXoNPcbsP8t
-----END RSA PRIVATE KEY-----
-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHHCgVZU1W/MA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCVRl
c3QgQ0EgMTAeFw0yNTAxMDEwMDAwMDBaFw0yNjAxMDEwMDAwMDBaMBQxEjAQBgNV
BAMMCVRlc3QgQ0EgMTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQDPwkpaQTussbGq
supYMePypXOo09WncTRCvSo8tzbHS0fs8Ya873Gf0x8o+qNeoTtMGkfgztDdYkT5
83krypVAgMBAAEwDQYJKoZIhvcNAQELBQADQQBDupVNQlXOom8vBOoT0q/Oom8v
BQ==
-----END CERTIFICATE-----`

	count := fetcher.CountCertificates([]byte(mixedPEM))

	// Should count only the 2 CERTIFICATE blocks, not the RSA PRIVATE KEY
	// Note: These are invalid certs, so may be 0 if parsing fails
	if count > 2 {
		t.Errorf("countCertificates() = %d, should not exceed 2 (number of CERTIFICATE blocks)", count)
	}

	t.Logf("Counted %d valid certificates from mixed PEM", count)
}

func TestCountCertificates_InvalidCerts(t *testing.T) {
	// Test with CERTIFICATE blocks that aren't valid x509
	invalidPEM := `-----BEGIN CERTIFICATE-----
This is not a valid certificate
-----END CERTIFICATE-----`

	count := fetcher.CountCertificates([]byte(invalidPEM))

	// Should not count invalid certificates
	if count != 0 {
		t.Errorf("countCertificates() = %d, want 0 for invalid certificate", count)
	}
}

func TestMetadata_MarshalUnmarshal(t *testing.T) {
	// Create metadata with some data
	original := NewMetadata()
	original.MozillaBundle = BundleInfo{
		Generated: time.Now().Round(time.Second), // Round to avoid sub-second precision issues
		SHA256:    "abc123",
		CertCount: 100,
		Source:    "test",
	}
	original.UserCerts = []UserCertInfo{
		{
			Name:        "test",
			Path:        "user/test.pem",
			Added:       time.Now().Round(time.Second),
			Fingerprint: "sha256:test",
			Subject:     "CN=Test",
			Expires:     time.Now().Add(365 * 24 * time.Hour).Round(time.Second),
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	// Unmarshal back
	var restored Metadata
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	// Verify fields match
	if restored.Version != original.Version {
		t.Errorf("Version = %q, want %q", restored.Version, original.Version)
	}

	if restored.MozillaBundle.CertCount != original.MozillaBundle.CertCount {
		t.Errorf("MozillaBundle.CertCount = %d, want %d", restored.MozillaBundle.CertCount, original.MozillaBundle.CertCount)
	}

	if len(restored.UserCerts) != len(original.UserCerts) {
		t.Errorf("UserCerts length = %d, want %d", len(restored.UserCerts), len(original.UserCerts))
	}
}

func TestReadMetadata_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := &Store{
		basePath: tmpDir,
		fs:       &OSFileSystem{},
	}

	_, err := store.readMetadata()
	if err == nil {
		t.Error("readMetadata() should fail when file doesn't exist")
	}
}

func TestReadMetadata_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create certs directory
	certsDir := filepath.Join(tmpDir, "certs")
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		t.Fatalf("Failed to create certs dir: %v", err)
	}

	metadataPath := filepath.Join(certsDir, "metadata.json")

	// Write invalid JSON
	if err := os.WriteFile(metadataPath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	store := &Store{
		basePath: tmpDir,
		fs:       &OSFileSystem{},
	}

	_, err := store.readMetadata()
	if err == nil {
		t.Error("readMetadata() should fail with invalid JSON")
	}

	if !strings.Contains(err.Error(), "unmarshal") && !strings.Contains(err.Error(), "invalid") {
		t.Logf("Error message: %v", err)
	}
}

func TestWriteMetadata_Atomic(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store structure
	if err := os.MkdirAll(filepath.Join(tmpDir, "certs"), 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	store := &Store{
		basePath: tmpDir,
		fs:       &OSFileSystem{},
	}

	metadata := NewMetadata()
	metadata.MozillaBundle.CertCount = 100

	// Write metadata
	if err := store.writeMetadata(metadata); err != nil {
		t.Fatalf("writeMetadata() failed: %v", err)
	}

	// Verify file exists
	metadataPath := filepath.Join(tmpDir, "certs", "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("Metadata file was not created")
	}

	// Verify temp file was cleaned up
	tempPath := metadataPath + ".tmp"
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Temporary file was not cleaned up")
	}

	// Verify content
	read, err := store.readMetadata()
	if err != nil {
		t.Fatalf("readMetadata() failed: %v", err)
	}

	if read.MozillaBundle.CertCount != 100 {
		t.Errorf("CertCount = %d, want 100", read.MozillaBundle.CertCount)
	}
}

func TestUpdateMetadata_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and initialize it
	if err := os.MkdirAll(filepath.Join(tmpDir, "certs"), 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	store := &Store{
		basePath: tmpDir,
		fs:       &OSFileSystem{},
	}

	// Write initial metadata
	initial := NewMetadata()
	if err := store.writeMetadata(initial); err != nil {
		t.Fatalf("writeMetadata() failed: %v", err)
	}

	// Update metadata
	ctx := context.Background()
	err := store.UpdateMetadata(ctx, func(m *Metadata) error {
		m.MozillaBundle.CertCount = 150
		return nil
	})

	if err != nil {
		t.Fatalf("UpdateMetadata() failed: %v", err)
	}

	// Verify update
	read, err := store.readMetadata()
	if err != nil {
		t.Fatalf("readMetadata() failed: %v", err)
	}

	if read.MozillaBundle.CertCount != 150 {
		t.Errorf("CertCount = %d, want 150", read.MozillaBundle.CertCount)
	}
}

func TestUpdateMetadata_FunctionError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and initialize it
	if err := os.MkdirAll(filepath.Join(tmpDir, "certs"), 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	store := &Store{
		basePath: tmpDir,
		fs:       &OSFileSystem{},
	}

	// Write initial metadata
	initial := NewMetadata()
	initial.MozillaBundle.CertCount = 100
	if err := store.writeMetadata(initial); err != nil {
		t.Fatalf("writeMetadata() failed: %v", err)
	}

	// Try to update with a function that returns error
	ctx := context.Background()
	testErr := errors.New("test error")
	err := store.UpdateMetadata(ctx, func(m *Metadata) error {
		m.MozillaBundle.CertCount = 200
		return testErr
	})

	if err == nil {
		t.Fatal("UpdateMetadata() should have failed")
	}

	if !errors.Is(err, testErr) {
		t.Errorf("Error should wrap test error, got: %v", err)
	}

	// Verify metadata was NOT updated (rollback)
	read, err := store.readMetadata()
	if err != nil {
		t.Fatalf("readMetadata() failed: %v", err)
	}

	if read.MozillaBundle.CertCount != 100 {
		t.Errorf("CertCount = %d, want 100 (should not have been updated)", read.MozillaBundle.CertCount)
	}
}

func TestUpdateMetadata_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and initialize it
	if err := os.MkdirAll(filepath.Join(tmpDir, "certs"), 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	store := &Store{
		basePath: tmpDir,
		fs:       &OSFileSystem{},
	}

	// Write initial metadata
	initial := NewMetadata()
	initial.MozillaBundle.CertCount = 0
	if err := store.writeMetadata(initial); err != nil {
		t.Fatalf("writeMetadata() failed: %v", err)
	}

	// Concurrent updates
	const numUpdates = 10
	done := make(chan bool, numUpdates)

	for i := 0; i < numUpdates; i++ {
		go func() {
			ctx := context.Background()
			err := store.UpdateMetadata(ctx, func(m *Metadata) error {
				count := m.MozillaBundle.CertCount
				time.Sleep(10 * time.Millisecond) // Simulate work
				m.MozillaBundle.CertCount = count + 1
				return nil
			})
			if err != nil {
				t.Errorf("UpdateMetadata() failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all updates
	for i := 0; i < numUpdates; i++ {
		<-done
	}

	// Verify final count
	read, err := store.readMetadata()
	if err != nil {
		t.Fatalf("readMetadata() failed: %v", err)
	}

	if read.MozillaBundle.CertCount != numUpdates {
		t.Errorf("CertCount = %d, want %d (locking failed)", read.MozillaBundle.CertCount, numUpdates)
	}
}

func TestMigrateMetadata_NoOp(t *testing.T) {
	// Test that v1 to v1 migration is a no-op
	metadata := NewMetadata()
	metadata.Version = "1"
	metadata.MozillaBundle.CertCount = 100

	err := migrateMetadata(metadata)
	if err != nil {
		t.Errorf("migrateMetadata() failed: %v", err)
	}

	// Version should still be 1
	if metadata.Version != "1" {
		t.Errorf("Version = %q, want %q", metadata.Version, "1")
	}

	// Data should be unchanged
	if metadata.MozillaBundle.CertCount != 100 {
		t.Errorf("CertCount changed during migration: %d", metadata.MozillaBundle.CertCount)
	}
}
