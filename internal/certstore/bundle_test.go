package certstore

import (
	"context"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_AddCert_Integration(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create and initialize store
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Generate test certificate
	certPEM := generateTestCert(t, "Corporate CA", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))

	// Write certificate to temp file
	certPath := filepath.Join(tmpDir, "test-cert.pem")
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Add certificate
	if err := store.AddCert(ctx, certPath, "corporate", false); err != nil {
		t.Fatalf("AddCert() error = %v", err)
	}

	// Verify certificate was added to metadata
	certs, err := store.ListCerts()
	if err != nil {
		t.Fatalf("ListCerts() error = %v", err)
	}

	if len(certs) != 1 {
		t.Fatalf("ListCerts() returned %d certs, want 1", len(certs))
	}

	if certs[0].Name != "corporate" {
		t.Errorf("cert.Name = %v, want 'corporate'", certs[0].Name)
	}

	if certs[0].Subject == "" {
		t.Errorf("cert.Subject is empty")
	}

	// Verify certificate file was created
	userCertPath := store.userCertPath("corporate")
	if _, err := os.Stat(userCertPath); os.IsNotExist(err) {
		t.Errorf("User certificate file was not created at %s", userCertPath)
	}

	// Verify combined bundle contains user certificate
	bundlePath := store.CombinedBundlePath()
	bundleData, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("ReadFile(bundle) error = %v", err)
	}

	// Count certificates in bundle (should be Mozilla certs + 1 user cert)
	certCount := countCertificates(bundleData)
	if certCount < 2 { // At least Mozilla bundle + our cert
		t.Errorf("Bundle contains %d certificates, expected at least 2", certCount)
	}

	// Verify metadata was updated
	metadata, err := store.readMetadata()
	if err != nil {
		t.Fatalf("readMetadata() error = %v", err)
	}

	if len(metadata.CombinedBundle.Sources) != 2 {
		t.Errorf("Combined bundle sources = %v, want 2 sources", metadata.CombinedBundle.Sources)
	}

	hasUser := false
	for _, src := range metadata.CombinedBundle.Sources {
		if src == "user" {
			hasUser = true
			break
		}
	}
	if !hasUser {
		t.Errorf("Combined bundle sources missing 'user': %v", metadata.CombinedBundle.Sources)
	}
}

func TestStore_AddCert_Replace(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create and initialize store
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Add first certificate
	cert1 := generateTestCert(t, "Corporate CA v1", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))
	certPath1 := filepath.Join(tmpDir, "cert1.pem")
	if err := os.WriteFile(certPath1, cert1, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := store.AddCert(ctx, certPath1, "corporate", false); err != nil {
		t.Fatalf("AddCert() error = %v", err)
	}

	// Get first certificate's fingerprint
	certs1, _ := store.ListCerts()
	fingerprint1 := certs1[0].Fingerprint

	// Add second certificate with same name (should replace)
	cert2 := generateTestCert(t, "Corporate CA v2", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))
	certPath2 := filepath.Join(tmpDir, "cert2.pem")
	if err := os.WriteFile(certPath2, cert2, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := store.AddCert(ctx, certPath2, "corporate", false); err != nil {
		t.Fatalf("AddCert() error = %v", err)
	}

	// Verify only one certificate exists
	certs2, err := store.ListCerts()
	if err != nil {
		t.Fatalf("ListCerts() error = %v", err)
	}

	if len(certs2) != 1 {
		t.Fatalf("ListCerts() returned %d certs, want 1", len(certs2))
	}

	// Verify fingerprint changed (different certificate)
	fingerprint2 := certs2[0].Fingerprint
	if fingerprint1 == fingerprint2 {
		t.Errorf("Fingerprints should be different after replacement")
	}

	// Verify subject changed
	if certs2[0].Subject != "CN=Corporate CA v2" {
		t.Errorf("Subject = %v, want 'CN=Corporate CA v2'", certs2[0].Subject)
	}
}

func TestStore_AddCert_Multiple(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create and initialize store
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Add multiple certificates
	certNames := []string{"corporate", "proxy", "internal"}
	for _, name := range certNames {
		cert := generateTestCert(t, name+" CA", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))
		certPath := filepath.Join(tmpDir, name+".pem")
		if err := os.WriteFile(certPath, cert, 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		if err := store.AddCert(ctx, certPath, name, false); err != nil {
			t.Fatalf("AddCert(%s) error = %v", name, err)
		}
	}

	// Verify all certificates were added
	certs, err := store.ListCerts()
	if err != nil {
		t.Fatalf("ListCerts() error = %v", err)
	}

	if len(certs) != 3 {
		t.Fatalf("ListCerts() returned %d certs, want 3", len(certs))
	}

	// Verify all names are present
	names := make(map[string]bool)
	for _, cert := range certs {
		names[cert.Name] = true
	}

	for _, expectedName := range certNames {
		if !names[expectedName] {
			t.Errorf("Certificate '%s' not found in list", expectedName)
		}
	}
}

func TestStore_readUserCerts(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create and initialize store
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Initially should have no user certs
	userCerts, err := store.readUserCerts(ctx)
	if err != nil {
		t.Fatalf("readUserCerts() error = %v", err)
	}

	if len(userCerts) != 0 {
		t.Errorf("readUserCerts() returned %d certs, want 0", len(userCerts))
	}

	// Add certificates directly to user directory
	userDir := filepath.Join(tmpDir, "certs", "user")
	cert1 := generateTestCert(t, "Cert 1", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))
	cert2 := generateTestCert(t, "Cert 2", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))

	if err := os.WriteFile(filepath.Join(userDir, "cert1.pem"), cert1, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "cert2.pem"), cert2, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Should now find 2 certificates
	userCerts, err = store.readUserCerts(ctx)
	if err != nil {
		t.Fatalf("readUserCerts() error = %v", err)
	}

	if len(userCerts) != 2 {
		t.Errorf("readUserCerts() returned %d certs, want 2", len(userCerts))
	}

	// Verify they're valid PEM
	for i, certData := range userCerts {
		block, _ := pem.Decode(certData)
		if block == nil {
			t.Errorf("User cert %d is not valid PEM", i)
		}
	}
}

func TestStore_AddCert_NotInitialized(t *testing.T) {
	// Create store but don't initialize
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Try to add certificate
	ctx := context.Background()
	cert := generateTestCert(t, "Test CA", time.Now().Add(-24*time.Hour), time.Now().Add(365*24*time.Hour))
	certPath := filepath.Join(tmpDir, "cert.pem")
	if err := os.WriteFile(certPath, cert, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = store.AddCert(ctx, certPath, "test", false)
	if err == nil {
		t.Error("AddCert() should fail when store is not initialized")
	}
}
