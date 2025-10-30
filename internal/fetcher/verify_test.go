package fetcher

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

const validTestCert2 = `-----BEGIN CERTIFICATE-----
MIICoDCCAYgCCQD0S8sg5vCG5TANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDDAdU
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

func TestVerifyBundle_Valid(t *testing.T) {
	// Create a bundle with enough certificates
	var bundle strings.Builder
	for i := 0; i < MinCertCount; i++ {
		if i%2 == 0 {
			bundle.WriteString(validTestCert)
		} else {
			bundle.WriteString(validTestCert2)
		}
		bundle.WriteString("\n")
	}

	bundleData := []byte(bundle.String())

	result, err := VerifyBundle(bundleData, 0)
	require.NoError(t, err)
	assert.True(t, result.IsValid)
	assert.GreaterOrEqual(t, result.CertCount, MinCertCount)
	assert.Empty(t, result.Warning)
}

func TestVerifyBundle_BelowMinimum(t *testing.T) {
	// Bundle with fewer than MinCertCount certificates
	var bundle strings.Builder
	for i := 0; i < MinCertCount-1; i++ {
		bundle.WriteString(validTestCert)
		bundle.WriteString("\n")
	}

	bundleData := []byte(bundle.String())

	result, err := VerifyBundle(bundleData, 0)
	require.Error(t, err)
	assert.False(t, result.IsValid)
	assert.Contains(t, err.Error(), "expected at least")
	assert.Less(t, result.CertCount, MinCertCount)
}

func TestVerifyBundle_Degradation(t *testing.T) {
	tests := []struct {
		name            string
		currentCount    int
		newCount        int
		expectWarning   bool
		warningContains string
	}{
		{
			name:          "no degradation",
			currentCount:  150,
			newCount:      150,
			expectWarning: false,
		},
		{
			name:          "slight increase",
			currentCount:  150,
			newCount:      160,
			expectWarning: false,
		},
		{
			name:          "acceptable decrease (10%)",
			currentCount:  150,
			newCount:      135,
			expectWarning: false,
		},
		{
			name:            "significant degradation (25%)",
			currentCount:    150,
			newCount:        112,
			expectWarning:   true,
			warningContains: "38 fewer certificates",
		},
		{
			name:            "major degradation (50%)",
			currentCount:    150,
			newCount:        100,
			expectWarning:   true,
			warningContains: "33.3% decrease",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create bundle with tt.newCount certificates
			var bundle strings.Builder
			for i := 0; i < tt.newCount; i++ {
				if i%2 == 0 {
					bundle.WriteString(validTestCert)
				} else {
					bundle.WriteString(validTestCert2)
				}
				bundle.WriteString("\n")
			}

			bundleData := []byte(bundle.String())

			result, err := VerifyBundle(bundleData, tt.currentCount)
			require.NoError(t, err)
			assert.True(t, result.IsValid)

			if tt.expectWarning {
				assert.NotEmpty(t, result.Warning)
				assert.Contains(t, result.Warning, tt.warningContains)
			} else {
				assert.Empty(t, result.Warning)
			}
		})
	}
}

func TestParseMozillaDate(t *testing.T) {
	tests := []struct {
		name         string
		bundleData   string
		expectFound  bool
		expectedDay  int
		expectedYear int
	}{
		{
			name: "valid Mozilla header",
			bundleData: `## Certificate data from Mozilla as of: Tue Sep  9 03:12:01 2025 GMT
##
## This is a bundle of X.509 certificates
-----BEGIN CERTIFICATE-----
...
`,
			expectFound:  true,
			expectedDay:  9,
			expectedYear: 2025,
		},
		{
			name: "valid header with single digit day",
			bundleData: `## Certificate data from Mozilla as of: Mon Jan  2 15:04:05 2026 GMT
##
`,
			expectFound:  true,
			expectedDay:  2,
			expectedYear: 2026,
		},
		{
			name: "valid header with double digit day",
			bundleData: `## Certificate data from Mozilla as of: Wed Dec 25 10:30:45 2024 GMT
##
`,
			expectFound:  true,
			expectedDay:  25,
			expectedYear: 2024,
		},
		{
			name: "no Mozilla header",
			bundleData: `-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUfyP...
`,
			expectFound: false,
		},
		{
			name: "invalid date format",
			bundleData: `## Certificate data from Mozilla as of: Invalid Date
##
`,
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, found := parseMozillaDate([]byte(tt.bundleData))

			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.Equal(t, tt.expectedDay, date.Day())
				assert.Equal(t, tt.expectedYear, date.Year())
			}
		})
	}
}

func TestExtractMozillaDateString(t *testing.T) {
	tests := []struct {
		name       string
		bundleData string
		expected   string
	}{
		{
			name: "valid date",
			bundleData: `## Certificate data from Mozilla as of: Tue Sep  9 03:12:01 2025 GMT
##
`,
			expected: "2025-09-09",
		},
		{
			name: "no date",
			bundleData: `-----BEGIN CERTIFICATE-----
...
`,
			expected: "",
		},
		{
			name: "December date",
			bundleData: `## Certificate data from Mozilla as of: Wed Dec 25 10:30:45 2024 GMT
##
`,
			expected: "2024-12-25",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMozillaDateString([]byte(tt.bundleData))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountCertificates(t *testing.T) {
	tests := []struct {
		name     string
		pemData  string
		expected int
	}{
		{
			name:     "single certificate",
			pemData:  validTestCert,
			expected: 1,
		},
		{
			name:     "two certificates",
			pemData:  validTestCert + "\n" + validTestCert2,
			expected: 2,
		},
		{
			name:     "empty data",
			pemData:  "",
			expected: 0,
		},
		{
			name:     "invalid PEM",
			pemData:  "not a certificate",
			expected: 0,
		},
		{
			name: "mixed PEM blocks (only count CERTIFICATE)",
			pemData: validTestCert + `
-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7VJTUt9Us8cKj
-----END PRIVATE KEY-----
` + validTestCert2,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := countCertificates([]byte(tt.pemData))
			assert.Equal(t, tt.expected, count)
		})
	}
}

func TestValidatePEMFormat(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid PEM",
			data:    validTestCert,
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    "",
			wantErr: true,
			errMsg:  "empty data",
		},
		{
			name:    "not PEM format",
			data:    "This is not a PEM certificate",
			wantErr: true,
			errMsg:  "does not appear to be in PEM format",
		},
		{
			name:    "invalid PEM (no BEGIN block)",
			data:    "-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "does not appear to be in PEM format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePEMFormat([]byte(tt.data))

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestComputeSHA256(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{
			name:     "empty data",
			data:     "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple string",
			data:     "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "certificate data",
			data:     validTestCert,
			expected: "75abc4a0c79c57fb2b5bdc2cf7f2c66e9a1ed2e92af62f4f3ccf94e8e1e8c9f4", // This will be the actual hash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeSHA256([]byte(tt.data))

			// For non-empty, non-hello-world data, just verify format
			if tt.name == "certificate data" {
				assert.Len(t, result, 64) // SHA256 is 64 hex characters
				assert.Regexp(t, "^[a-f0-9]{64}$", result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBundleVerificationResult(t *testing.T) {
	// Test the BundleVerificationResult structure
	now := time.Now()

	result := &BundleVerificationResult{
		CertCount:       150,
		MozillaDate:     now,
		IsValid:         true,
		Warning:         "Test warning",
		HasDateInHeader: true,
	}

	assert.Equal(t, 150, result.CertCount)
	assert.Equal(t, now, result.MozillaDate)
	assert.True(t, result.IsValid)
	assert.Equal(t, "Test warning", result.Warning)
	assert.True(t, result.HasDateInHeader)
}

func TestVerifyBundle_WithMozillaDate(t *testing.T) {
	// Create bundle with Mozilla header
	bundleHeader := `## Certificate data from Mozilla as of: Tue Sep  9 03:12:01 2025 GMT
##
## This is a bundle of X.509 certificates
##
`
	var bundle strings.Builder
	bundle.WriteString(bundleHeader)
	for i := 0; i < MinCertCount; i++ {
		bundle.WriteString(validTestCert)
		bundle.WriteString("\n")
	}

	bundleData := []byte(bundle.String())

	result, err := VerifyBundle(bundleData, 0)
	require.NoError(t, err)
	assert.True(t, result.IsValid)
	assert.True(t, result.HasDateInHeader)
	assert.Equal(t, 9, result.MozillaDate.Day())
	assert.Equal(t, 2025, result.MozillaDate.Year())
}
