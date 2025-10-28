package certstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

func TestNewStore_DefaultPath(t *testing.T) {
	// NewStore with empty path should use ~/.verifi
	store, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore(\"\") failed: %v", err)
	}

	if store.basePath == "" {
		t.Error("basePath should not be empty")
	}

	// Should contain .verifi
	if !strings.Contains(store.basePath, ".verifi") {
		t.Errorf("basePath = %q, should contain '.verifi'", store.basePath)
	}

	// Should be an absolute path
	if !filepath.IsAbs(store.basePath) {
		t.Errorf("basePath = %q, should be absolute", store.basePath)
	}

	t.Logf("Default basePath: %s", store.basePath)
}

func TestNewStore_CustomPath(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom-verifi")

	store, err := NewStore(customPath)
	if err != nil {
		t.Fatalf("NewStore(%q) failed: %v", customPath, err)
	}

	if store.basePath != customPath {
		t.Errorf("basePath = %q, want %q", store.basePath, customPath)
	}
}

func TestInit_Success(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify directory structure was created
	expectedDirs := []string{
		filepath.Join(tmpDir, "certs", "user"),
		filepath.Join(tmpDir, "certs", "bundles"),
		filepath.Join(tmpDir, "logs"),
	}

	for _, dir := range expectedDirs {
		if info, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory not created: %s", dir)
		} else if !info.IsDir() {
			t.Errorf("Path exists but is not a directory: %s", dir)
		}
	}

	// Verify Mozilla bundle was written
	mozillaPath := filepath.Join(tmpDir, "certs", "bundles", "mozilla-ca-bundle.pem")
	if _, err := os.Stat(mozillaPath); os.IsNotExist(err) {
		t.Error("Mozilla bundle was not written")
	}

	// Verify combined bundle was created
	combinedPath := filepath.Join(tmpDir, "certs", "bundles", "combined-bundle.pem")
	if _, err := os.Stat(combinedPath); os.IsNotExist(err) {
		t.Error("Combined bundle was not created")
	}

	// Verify metadata was created
	metadataPath := filepath.Join(tmpDir, "certs", "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("Metadata was not created")
	}

	// Verify IsInitialized returns true
	if !store.IsInitialized() {
		t.Error("IsInitialized() should return true after Init()")
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	ctx := context.Background()

	// Initialize once
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("First Init() failed: %v", err)
	}

	// Try to initialize again without force
	err = store.Init(ctx, false)
	if err == nil {
		t.Error("Second Init() should fail when already initialized")
	}
}

func TestInit_Force(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	ctx := context.Background()

	// Initialize once
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("First Init() failed: %v", err)
	}

	// Write a marker file to verify reinit
	markerPath := filepath.Join(tmpDir, "certs", "user", "marker.txt")
	if err := os.WriteFile(markerPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write marker: %v", err)
	}

	// Reinitialize with force - marker should still exist (force doesn't delete, just overwrites)
	if err := store.Init(ctx, true); err != nil {
		t.Fatalf("Init with force failed: %v", err)
	}

	// Marker should still exist (Init doesn't clean up existing files)
	if _, err := os.Stat(markerPath); err != nil {
		t.Logf("Marker file state: %v", err)
	}

	// Metadata should be recreated
	if !store.IsInitialized() {
		t.Error("Store should still be initialized after force reinit")
	}
}

func TestInit_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = store.Init(ctx, false)
	if err == nil {
		t.Error("Init() should fail with cancelled context")
	}

	if err != context.Canceled {
		t.Logf("Error type: %v", err)
	}
}

func TestIsInitialized_False(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	if store.IsInitialized() {
		t.Error("IsInitialized() should return false before Init()")
	}
}

func TestBasePath(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	if store.BasePath() != tmpDir {
		t.Errorf("BasePath() = %q, want %q", store.BasePath(), tmpDir)
	}
}

func TestCombinedBundlePath(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "certs", "bundles", "combined-bundle.pem")
	if store.CombinedBundlePath() != expected {
		t.Errorf("CombinedBundlePath() = %q, want %q", store.CombinedBundlePath(), expected)
	}
}

func TestRebuildBundle_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Initialize the store first
	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Create a cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to rebuild with cancelled context
	metadata := NewMetadata()
	err = store.rebuildBundle(cancelledCtx, metadata)
	if err == nil {
		t.Error("rebuildBundle() should fail with cancelled context")
	}
}

func TestAddCert_NotInitialized(t *testing.T) {
	// This test is already in bundle_test.go but worth having here too
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Try to add cert without initializing
	ctx := context.Background()
	err = store.AddCert(ctx, "nonexistent.pem", "test", false)

	if err == nil {
		t.Error("AddCert() should fail when store not initialized")
	}

	if !verifierrors.IsError(err, verifierrors.ErrStoreNotInit) {
		t.Errorf("Error should be ErrStoreNotInit, got: %v", err)
	}
}

func TestAddCert_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Create a cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to add cert with cancelled context
	err = store.AddCert(cancelledCtx, "test.pem", "test", false)
	if err == nil {
		t.Error("AddCert() should fail with cancelled context")
	}

	if err != context.Canceled {
		t.Logf("Error: %v", err)
	}
}

func TestAddCert_InvalidCertPath(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Try to add cert from non-existent file
	err = store.AddCert(ctx, "/nonexistent/path/cert.pem", "test", false)
	if err == nil {
		t.Error("AddCert() should fail with non-existent cert path")
	}
}

func TestListCerts_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Try to list certs without initializing
	certs, err := store.ListCerts()
	if err == nil {
		t.Error("ListCerts() should fail when store not initialized")
	}

	if certs != nil {
		t.Errorf("ListCerts() should return nil certs on error, got %v", certs)
	}

	if !verifierrors.IsError(err, verifierrors.ErrStoreNotInit) {
		t.Errorf("Error should be ErrStoreNotInit, got: %v", err)
	}
}

func TestListCerts_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// List certs - should be empty
	certs, err := store.ListCerts()
	if err != nil {
		t.Fatalf("ListCerts() failed: %v", err)
	}

	if len(certs) != 0 {
		t.Errorf("ListCerts() returned %d certs, want 0", len(certs))
	}
}

func TestCountCertificates(t *testing.T) {
	// Test the countCertificates helper function
	tests := []struct {
		name string
		pem  string
		want int
	}{
		{
			name: "empty",
			pem:  "",
			want: 0,
		},
		{
			name: "no certificates",
			pem:  "not a pem file",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countCertificates([]byte(tt.pem))
			if got != tt.want {
				t.Errorf("countCertificates() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStore_RebuildBundle_WithUserCerts(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Initialize
	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Add a certificate using the test helper from validate_test.go
	certPEM := generateTestCert(t, "Test CA", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))
	certPath := filepath.Join(tmpDir, "test-cert.pem")
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write test cert: %v", err)
	}

	if err := store.AddCert(ctx, certPath, "test", false); err != nil {
		t.Fatalf("AddCert() failed: %v", err)
	}

	// Read the combined bundle
	bundlePath := store.CombinedBundlePath()
	bundleData, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("Failed to read bundle: %v", err)
	}

	// Count certificates - should be Mozilla certs + 1
	certCount := countCertificates(bundleData)
	if certCount < 2 {
		t.Errorf("Bundle should contain at least 2 certificates (Mozilla + test), got %d", certCount)
	}

	// Verify metadata shows user source
	metadata, err := store.readMetadata()
	if err != nil {
		t.Fatalf("readMetadata() failed: %v", err)
	}

	hasUser := false
	for _, src := range metadata.CombinedBundle.Sources {
		if src == "user" {
			hasUser = true
			break
		}
	}

	if !hasUser {
		t.Errorf("Combined bundle sources should include 'user', got: %v", metadata.CombinedBundle.Sources)
	}
}
