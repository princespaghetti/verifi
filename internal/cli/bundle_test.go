package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/princespaghetti/verifi/internal/certstore"
	"github.com/princespaghetti/verifi/internal/fetcher"
)

// validTestCert is a real self-signed certificate for testing
const validTestCert = `-----BEGIN CERTIFICATE-----
MIICoDCCAYgCCQD0S8sg5vCG5DANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDDAdU
ZXN0IENBMB4XDTI1MTAyOTIzMTgwMloXDTI2MTAyOTIzMTgwMlowEjEQMA4GA1UE
AwwHVGVzdCBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKlR1m2P
1pXdb23Y2KVkltvgLLEaA2KWrjQipFd6cWoG+9rDKv7BLzu2zozWW589yzmgo3NU
Gff0xF/vx7XCYwZxTTjnHgYS0FwotSdIyFThtFVYJZo188ipl63s2MOSEzJCsWoJ
YA+toUJi1O5yuJ9iFix8JCgtsp8RcRUm3MUQgCu5mr5i/6gDbk9gNF3dWausYqKx
UFVs6KXlkd6aPNWMdmZHU+9ibnQO2spNBq+gdmEWprdERtmgE2wfv08JSTIgfo7V
x+UowB8wYoM4+o3/7AEgG/g5vHbVJpRqrgR6v+kLoW45il25WDfvzPQtpTD6/PGz
6dE1L3uQnLU0XaECAwEAATANBgkqhkiG9w0BAQsFAAOCAQEACm+ZaiddI+X1xT+Y
QSBZ1/Ft/UL2d3+p1YyRV03ESB3QGQu5/zGvXrem/dFqAhgSQwjjBNR0s0uz3BC/
XNBhYyzpIvvIb3YsDhO08VS8soEuYsREPfO/eQCKrmTsGUsbaQV1M/79ghsGkpD2
lSufAR8kyscmp6FRvmpNCWigneDuHFrDBNanMtd8PLMxOcCFwH/kjObH61LbHS9z
uWC0tivgAd6n3qCGjpplw2VY/cN0XAHLFzyS5CAu6N4lZvLWcPqKLGJO1vevTaml
VZ5il3bOgM9OVuouB7Yx97EowRVDHifb3GCaI3NyKLL7JIizrS8WfWG1emHV996m
yxiEgg==
-----END CERTIFICATE-----
`

func createTestBundle(certCount int) []byte {
	var bundle strings.Builder
	bundle.WriteString(`## Certificate data from Mozilla as of: Tue Sep  9 03:12:01 2025 GMT
##
## This is a bundle of X.509 certificates
##
`)
	for i := 0; i < certCount; i++ {
		bundle.WriteString(validTestCert)
		bundle.WriteString("\n")
	}
	return []byte(bundle.String())
}

func initTestStore(t *testing.T) (*certstore.Store, string) {
	tempDir := t.TempDir()
	store, err := certstore.NewStore(tempDir)
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, false)
	require.NoError(t, err)

	return store, tempDir
}

func TestBundleInfoCmd_NotInitialized(t *testing.T) {
	// Save and restore os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create temp dir but don't initialize store
	tempDir := t.TempDir()
	_, err := certstore.NewStore(tempDir)
	require.NoError(t, err)

	// This test verifies the command fails gracefully when store is not initialized
	// We can't easily test the CLI exit behavior, but we verify the store check works
	store, _ := certstore.NewStore(tempDir)
	assert.False(t, store.IsInitialized())
}

func TestBundleInfoOutput_JSONFormat(t *testing.T) {
	store, _ := initTestStore(t)
	metadata, err := store.GetMetadata()
	require.NoError(t, err)

	// Create BundleInfoOutput
	mozillaBundlePath := filepath.Join(store.BasePath(), "certs", "bundles", "mozilla-ca-bundle.pem")
	var sizeBytes int64
	if info, err := os.Stat(mozillaBundlePath); err == nil {
		sizeBytes = info.Size()
	}

	output := BundleInfoOutput{
		Source:    metadata.MozillaBundle.Source,
		Version:   metadata.MozillaBundle.Version,
		CertCount: metadata.MozillaBundle.CertCount,
		SHA256:    metadata.MozillaBundle.SHA256,
		Generated: metadata.MozillaBundle.Generated,
		SizeBytes: sizeBytes,
		FilePath:  mozillaBundlePath,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(output)
	require.NoError(t, err)

	// Verify JSON structure
	var decoded BundleInfoOutput
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, output.Source, decoded.Source)
	assert.Equal(t, output.CertCount, decoded.CertCount)
	assert.Equal(t, output.SHA256, decoded.SHA256)
}

func TestPrintBundleInfoHuman(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	output := BundleInfoOutput{
		Source:    "embedded",
		Version:   "2025-09-09",
		CertCount: 137,
		SHA256:    "abc123def456",
		Generated: time.Date(2025, 10, 26, 10, 5, 0, 0, time.UTC),
		SizeBytes: 204800,
		FilePath:  "/home/user/.verifi/certs/bundles/mozilla-ca-bundle.pem",
	}

	printBundleInfoHuman(output)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	captured := buf.String()

	// Verify output contains expected information
	assert.Contains(t, captured, "Mozilla CA Bundle Information")
	assert.Contains(t, captured, "Source")
	assert.Contains(t, captured, "embedded")
	assert.Contains(t, captured, "Version")
	assert.Contains(t, captured, "2025-09-09")
	assert.Contains(t, captured, "Certificates")
	assert.Contains(t, captured, "137")
	assert.Contains(t, captured, "abc123def456")
	assert.Contains(t, captured, "200.0 KB")
}

func TestBundleUpdate_VerificationFlow(t *testing.T) {
	store, _ := initTestStore(t)

	// Get initial metadata
	metadata, err := store.GetMetadata()
	require.NoError(t, err)
	initialCertCount := metadata.MozillaBundle.CertCount

	// Create a new bundle with more certificates
	newBundle := createTestBundle(initialCertCount + 10)

	// Verify the new bundle
	verifyResult, err := fetcher.VerifyBundle(newBundle, initialCertCount)
	require.NoError(t, err)
	assert.True(t, verifyResult.IsValid)
	assert.Greater(t, verifyResult.CertCount, initialCertCount)
	assert.Empty(t, verifyResult.Warning) // No degradation

	// Write new bundle atomically
	mozillaBundlePath := filepath.Join(store.BasePath(), "certs", "bundles", "mozilla-ca-bundle.pem")
	tempPath := mozillaBundlePath + ".tmp"

	err = os.WriteFile(tempPath, newBundle, 0644)
	require.NoError(t, err)

	err = os.Rename(tempPath, mozillaBundlePath)
	require.NoError(t, err)

	// Update metadata
	ctx := context.Background()
	updateErr := store.UpdateMetadata(ctx, func(md *certstore.Metadata) error {
		mozillaDateStr := fetcher.ExtractMozillaDateString(newBundle)

		md.MozillaBundle = certstore.BundleInfo{
			Generated: time.Now(),
			SHA256:    fetcher.ComputeSHA256(newBundle),
			CertCount: verifyResult.CertCount,
			Source:    "https://curl.se/ca/cacert.pem",
			Version:   mozillaDateStr,
		}

		return store.RebuildBundle(ctx, md)
	})

	require.NoError(t, updateErr)

	// Verify metadata was updated
	updatedMetadata, err := store.GetMetadata()
	require.NoError(t, err)
	assert.Equal(t, verifyResult.CertCount, updatedMetadata.MozillaBundle.CertCount)
	assert.Equal(t, "2025-09-09", updatedMetadata.MozillaBundle.Version)
	assert.Equal(t, "https://curl.se/ca/cacert.pem", updatedMetadata.MozillaBundle.Source)
}

func TestBundleUpdate_DegradationWarning(t *testing.T) {
	store, _ := initTestStore(t)

	// Get initial metadata
	metadata, err := store.GetMetadata()
	require.NoError(t, err)
	initialCertCount := metadata.MozillaBundle.CertCount

	// Create a bundle with significant degradation (50% fewer certs)
	degradedCount := initialCertCount / 2
	if degradedCount < fetcher.MinCertCount {
		degradedCount = fetcher.MinCertCount
	}
	degradedBundle := createTestBundle(degradedCount)

	// Verify the degraded bundle
	verifyResult, err := fetcher.VerifyBundle(degradedBundle, initialCertCount)
	require.NoError(t, err)
	assert.True(t, verifyResult.IsValid)

	// Should have a warning about degradation
	if initialCertCount > fetcher.MinCertCount {
		assert.NotEmpty(t, verifyResult.Warning)
		assert.Contains(t, verifyResult.Warning, "fewer certificates")
	}
}

func TestBundleUpdate_InvalidBundle(t *testing.T) {
	store, _ := initTestStore(t)

	metadata, err := store.GetMetadata()
	require.NoError(t, err)
	currentCertCount := metadata.MozillaBundle.CertCount

	tests := []struct {
		name        string
		bundleData  []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "too few certificates",
			bundleData:  createTestBundle(50), // Less than MinCertCount
			expectError: true,
			errorMsg:    "expected at least",
		},
		{
			name:        "empty bundle",
			bundleData:  []byte{},
			expectError: true,
			errorMsg:    "expected at least",
		},
		{
			name:        "invalid PEM",
			bundleData:  []byte("not a certificate bundle"),
			expectError: true,
			errorMsg:    "expected at least",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fetcher.VerifyBundle(tt.bundleData, currentCertCount)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestComputeSHA256_Consistency(t *testing.T) {
	// Test that computeSHA256 wrapper works correctly
	testData := []byte("test data for sha256")

	// Compute using fetcher
	hash := fetcher.ComputeSHA256(testData)

	// SHA256 should be 64 hex characters
	assert.Len(t, hash, 64)

	// Verify it's a valid hex string
	assert.Regexp(t, "^[a-f0-9]{64}$", hash)
}

func TestBundleUpdate_MozillaDateExtraction(t *testing.T) {
	bundleWithDate := createTestBundle(100)

	// Extract date
	dateStr := fetcher.ExtractMozillaDateString(bundleWithDate)
	assert.Equal(t, "2025-09-09", dateStr)
}

func TestBundleUpdate_AtomicReplacement(t *testing.T) {
	store, _ := initTestStore(t)

	// Get original bundle path and content
	mozillaBundlePath := filepath.Join(store.BasePath(), "certs", "bundles", "mozilla-ca-bundle.pem")
	originalData, err := os.ReadFile(mozillaBundlePath)
	require.NoError(t, err)

	// Create new bundle
	newBundle := createTestBundle(120)

	// Write atomically using temp file
	tempPath := mozillaBundlePath + ".tmp"

	err = os.WriteFile(tempPath, newBundle, 0644)
	require.NoError(t, err)

	// Verify temp file exists
	_, err = os.Stat(tempPath)
	require.NoError(t, err)

	// Verify original file is unchanged
	currentData, err := os.ReadFile(mozillaBundlePath)
	require.NoError(t, err)
	assert.Equal(t, originalData, currentData)

	// Atomic rename
	err = os.Rename(tempPath, mozillaBundlePath)
	require.NoError(t, err)

	// Verify temp file is gone
	_, err = os.Stat(tempPath)
	assert.True(t, os.IsNotExist(err))

	// Verify new bundle is in place
	finalData, err := os.ReadFile(mozillaBundlePath)
	require.NoError(t, err)
	assert.Equal(t, newBundle, finalData)
}

func TestBundleUpdate_CombinedBundleRebuild(t *testing.T) {
	store, _ := initTestStore(t)

	// Add a user certificate
	ctx := context.Background()
	certPath := filepath.Join(t.TempDir(), "test-cert.pem")
	err := os.WriteFile(certPath, []byte(validTestCert), 0644)
	require.NoError(t, err)

	err = store.AddCert(ctx, certPath, "test-cert", false)
	require.NoError(t, err)

	// Get combined bundle before update
	combinedBundlePath := store.CombinedBundlePath()
	beforeData, err := os.ReadFile(combinedBundlePath)
	require.NoError(t, err)

	// Update Mozilla bundle
	newMozillaBundle := createTestBundle(110)
	mozillaBundlePath := filepath.Join(store.BasePath(), "certs", "bundles", "mozilla-ca-bundle.pem")
	err = os.WriteFile(mozillaBundlePath, newMozillaBundle, 0644)
	require.NoError(t, err)

	// Rebuild combined bundle
	err = store.UpdateMetadata(ctx, func(md *certstore.Metadata) error {
		return store.RebuildBundle(ctx, md)
	})
	require.NoError(t, err)

	// Verify combined bundle was updated
	afterData, err := os.ReadFile(combinedBundlePath)
	require.NoError(t, err)
	assert.NotEqual(t, beforeData, afterData)

	// Verify combined bundle contains both Mozilla and user certs
	assert.Contains(t, string(afterData), "BEGIN CERTIFICATE")

	// Get updated metadata
	updatedMetadata, err := store.GetMetadata()
	require.NoError(t, err)
	assert.Contains(t, updatedMetadata.CombinedBundle.Sources, "mozilla")
	assert.Contains(t, updatedMetadata.CombinedBundle.Sources, "user")
}

// mockHTTPClient for testing HTTP downloads
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestBundleUpdate_HTTPDownload(t *testing.T) {
	bundleData := createTestBundle(150)

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(bundleData)),
			}, nil
		},
	}

	f := fetcher.NewFetcher(mockClient)
	ctx := context.Background()

	result, err := f.FetchMozillaBundle(ctx, "https://curl.se/ca/cacert.pem")
	require.NoError(t, err)
	assert.Equal(t, bundleData, result)

	// Verify bundle
	verifyResult, err := fetcher.VerifyBundle(result, 0)
	require.NoError(t, err)
	assert.True(t, verifyResult.IsValid)
	assert.Equal(t, 150, verifyResult.CertCount)
}
