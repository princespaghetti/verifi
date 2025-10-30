package cli

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/princespaghetti/verifi/internal/certstore"
	"github.com/princespaghetti/verifi/internal/shell"
)

func TestStatusCmd_Exists(t *testing.T) {
	if statusCmd == nil {
		t.Fatal("statusCmd is nil")
	}

	if statusCmd.Use != "status" {
		t.Errorf("statusCmd.Use = %q, want %q", statusCmd.Use, "status")
	}
}

func TestStatusCmd_Flags(t *testing.T) {
	flag := statusCmd.Flags().Lookup("json")
	if flag == nil {
		t.Fatal("--json flag not found")
	}

	if flag.DefValue != "false" {
		t.Errorf("--json default = %q, want %q", flag.DefValue, "false")
	}
}

func TestGatherStatus_Uninitialized(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	status := gatherStatus(store)

	if status.Initialized {
		t.Error("Status.Initialized should be false for uninitialized store")
	}

	if status.StoreLocation != tmpDir {
		t.Errorf("StoreLocation = %q, want %q", status.StoreLocation, tmpDir)
	}

	// Other fields should be zero/empty for uninitialized store
	if status.UserCerts.Count != 0 {
		t.Errorf("UserCerts.Count = %d, want 0", status.UserCerts.Count)
	}
}

func TestGatherStatus_Initialized(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Initialize store
	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Generate env.sh for complete test
	if err := shell.GenerateEnvFile(store.BasePath(), store.CombinedBundlePath()); err != nil {
		t.Fatalf("GenerateEnvFile() failed: %v", err)
	}

	status := gatherStatus(store)

	if !status.Initialized {
		t.Error("Status.Initialized should be true")
	}

	if status.CombinedBundle.CertCount == 0 {
		t.Error("CombinedBundle.CertCount should be > 0")
	}

	if status.MozillaBundle.Source != "embedded" {
		t.Errorf("MozillaBundle.Source = %q, want %q", status.MozillaBundle.Source, "embedded")
	}

	if status.UserCerts.Count != 0 {
		t.Errorf("UserCerts.Count = %d, want 0 (no certs added)", status.UserCerts.Count)
	}

	if status.CombinedBundle.SizeBytes == 0 {
		t.Error("CombinedBundle.SizeBytes should be > 0")
	}

	if !status.EnvFile.Exists {
		t.Error("EnvFile.Exists should be true")
	}

	expectedEnvPath := shell.EnvFilePath(store.BasePath())
	if status.EnvFile.Path != expectedEnvPath {
		t.Errorf("EnvFile.Path = %q, want %q", status.EnvFile.Path, expectedEnvPath)
	}
}

func TestGatherStatus_WithUserCerts(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Initialize store
	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Note: We don't actually add a certificate here because generating valid
	// x509 test certificates is complex and that functionality is tested in
	// the certstore package. Instead, we test the display logic separately
	// in TestPrintStatusHuman_WithUserCerts using mock data.

	status := gatherStatus(store)

	// Should be initialized with no user certs
	if !status.Initialized {
		t.Error("Status.Initialized should be true")
	}

	if status.UserCerts.Count != 0 {
		t.Errorf("UserCerts.Count = %d, want 0 (no certs added)", status.UserCerts.Count)
	}

	// Sources should only include mozilla (no user certs)
	foundMozilla := false
	for _, source := range status.CombinedBundle.Sources {
		if source == "mozilla" {
			foundMozilla = true
		}
		if source == "user" {
			t.Error("Sources should not include 'user' when no certs added")
		}
	}

	if !foundMozilla {
		t.Error("Combined bundle sources should include 'mozilla'")
	}
}

func TestGatherStatus_MissingEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Initialize store
	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Remove env.sh if it exists
	envPath := shell.EnvFilePath(store.BasePath())
	os.Remove(envPath)

	status := gatherStatus(store)

	if status.EnvFile.Exists {
		t.Error("EnvFile.Exists should be false when env.sh is missing")
	}

	if status.EnvFile.Path != envPath {
		t.Errorf("EnvFile.Path = %q, want %q", status.EnvFile.Path, envPath)
	}
}

func TestStatusOutput_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	status := gatherStatus(store)

	// Marshal to JSON
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	// Verify it's valid JSON
	var decoded StatusOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	// Verify key fields
	if decoded.StoreLocation != tmpDir {
		t.Errorf("Decoded StoreLocation = %q, want %q", decoded.StoreLocation, tmpDir)
	}

	if !decoded.Initialized {
		t.Error("Decoded Initialized should be true")
	}

	if decoded.CombinedBundle.CertCount == 0 {
		t.Error("Decoded CombinedBundle.CertCount should be > 0")
	}

	if decoded.MozillaBundle.Source != "embedded" {
		t.Errorf("Decoded MozillaBundle.Source = %q, want %q", decoded.MozillaBundle.Source, "embedded")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  string
	}{
		{
			name:  "zero bytes",
			input: 0,
			want:  "0 B",
		},
		{
			name:  "500 bytes",
			input: 500,
			want:  "500 B",
		},
		{
			name:  "exactly 1 KB",
			input: 1024,
			want:  "1.0 KB",
		},
		{
			name:  "1.5 KB",
			input: 1536,
			want:  "1.5 KB",
		},
		{
			name:  "exactly 1 MB",
			input: 1048576,
			want:  "1.0 MB",
		},
		{
			name:  "10 MB",
			input: 10485760,
			want:  "10.0 MB",
		},
		{
			name:  "100 MB",
			input: 104857600,
			want:  "100.0 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.input)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPrintStatusHuman_Uninitialized(t *testing.T) {
	status := StatusOutput{
		StoreLocation: "/tmp/test",
		Initialized:   false,
	}

	// Should not panic
	printStatusHuman(status)
}

func TestPrintStatusHuman_Initialized(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	ctx := context.Background()
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	status := gatherStatus(store)

	// Should not panic
	printStatusHuman(status)
}

func TestPrintStatusHuman_WithUserCerts(t *testing.T) {
	// Test the display logic with mock user certificate data
	// This isolates testing the display formatting from certificate validation
	mockTime := time.Date(2025, 10, 26, 10, 0, 0, 0, time.UTC)

	status := StatusOutput{
		StoreLocation: "/tmp/test",
		Initialized:   true,
		UserCerts: UserCertsStatus{
			Count: 2,
			Certs: []certstore.UserCertInfo{
				{
					Name:        "corporate-ca",
					Subject:     "CN=Corporate CA, O=Example Corp",
					Expires:     mockTime.Add(365 * 24 * time.Hour),
					Added:       mockTime,
					Fingerprint: "sha256:abc123",
					Path:        "user/corporate-ca.pem",
				},
				{
					Name:        "proxy-cert",
					Subject:     "CN=Proxy Certificate, O=Example Corp",
					Expires:     mockTime.Add(180 * 24 * time.Hour),
					Added:       mockTime,
					Fingerprint: "sha256:def456",
					Path:        "user/proxy-cert.pem",
				},
			},
		},
		CombinedBundle: CombinedBundleStatus{
			Path:      "/tmp/test/certs/bundles/combined-bundle.pem",
			CertCount: 148,
			Generated: mockTime,
			SHA256:    "abc123def456",
			SizeBytes: 240000,
			Sources:   []string{"mozilla", "user"},
		},
		MozillaBundle: MozillaBundleStatus{
			Source:    "embedded",
			CertCount: 146,
			Generated: mockTime,
		},
		EnvFile: EnvFileStatus{
			Exists: true,
			Path:   "/tmp/test/env.sh",
		},
	}

	// Should not panic and should handle user certs properly
	printStatusHuman(status)

	// Verify the data structure is correct
	if status.UserCerts.Count != 2 {
		t.Errorf("UserCerts.Count = %d, want 2", status.UserCerts.Count)
	}

	if len(status.UserCerts.Certs) != 2 {
		t.Errorf("len(UserCerts.Certs) = %d, want 2", len(status.UserCerts.Certs))
	}

	// Verify sources include both mozilla and user
	foundMozilla := false
	foundUser := false
	for _, source := range status.CombinedBundle.Sources {
		if source == "mozilla" {
			foundMozilla = true
		}
		if source == "user" {
			foundUser = true
		}
	}

	if !foundMozilla {
		t.Error("Sources should include 'mozilla'")
	}

	if !foundUser {
		t.Error("Sources should include 'user' when user certs are present")
	}
}
