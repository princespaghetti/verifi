package fetcher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPClient implements HTTPClient interface for testing
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestNewFetcher(t *testing.T) {
	t.Run("with nil client", func(t *testing.T) {
		fetcher := NewFetcher(nil)
		assert.NotNil(t, fetcher)
		assert.NotNil(t, fetcher.client)
	})

	t.Run("with custom client", func(t *testing.T) {
		customClient := &mockHTTPClient{}
		fetcher := NewFetcher(customClient)
		assert.NotNil(t, fetcher)
		assert.Equal(t, customClient, fetcher.client)
	})
}

func TestFetchMozillaBundle_Success(t *testing.T) {
	bundleData := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----")

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			// Verify User-Agent header
			assert.Equal(t, "verifi/1.0 (certificate management tool)", req.Header.Get("User-Agent"))
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, DefaultMozillaBundleURL, req.URL.String())

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(bundleData)),
			}, nil
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx := context.Background()

	result, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
	require.NoError(t, err)
	assert.Equal(t, bundleData, result)
}

func TestFetchMozillaBundle_CustomURL(t *testing.T) {
	customURL := "https://custom.example.com/cacert.pem"
	bundleData := []byte("test bundle")

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, customURL, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(bundleData)),
			}, nil
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx := context.Background()

	result, err := fetcher.FetchMozillaBundle(ctx, customURL)
	require.NoError(t, err)
	assert.Equal(t, bundleData, result)
}

func TestFetchMozillaBundle_HTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		statusText string
	}{
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			statusText: "404 Not Found",
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			statusText: "500 Internal Server Error",
		},
		{
			name:       "403 Forbidden",
			statusCode: http.StatusForbidden,
			statusText: "403 Forbidden",
		},
		{
			name:       "503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			statusText: "503 Service Unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockHTTPClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tt.statusCode,
						Status:     tt.statusText,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				},
			}

			fetcher := NewFetcher(mockClient)
			ctx := context.Background()

			result, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "download failed with status")
			assert.Contains(t, err.Error(), tt.statusText)
		})
	}
}

func TestFetchMozillaBundle_NetworkError(t *testing.T) {
	networkErr := errors.New("network connection failed")

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, networkErr
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx := context.Background()

	result, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "download bundle")
	assert.Contains(t, err.Error(), "network connection failed")
}

func TestFetchMozillaBundle_EmptyResponse(t *testing.T) {
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
			}, nil
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx := context.Background()

	result, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "downloaded bundle is empty")
}

func TestFetchMozillaBundle_ContextCancellation(t *testing.T) {
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			// Verify context is passed to request
			assert.NotNil(t, req.Context())
			return nil, context.Canceled
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestFetchMozillaBundle_ContextTimeout(t *testing.T) {
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			// Check if context is cancelled
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(100 * time.Millisecond):
				// This shouldn't be reached due to timeout
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("test")),
				}, nil
			}
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestFetchMozillaBundle_InvalidURL(t *testing.T) {
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx := context.Background()

	result, err := fetcher.FetchMozillaBundle(ctx, "://invalid-url")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "create request")
}

func TestFetchMozillaBundle_ReadError(t *testing.T) {
	// Create a reader that fails
	errorReader := &errorReader{err: errors.New("read error")}

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(errorReader),
			}, nil
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx := context.Background()

	result, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "read response")
	assert.Contains(t, err.Error(), "read error")
}

func TestFetchMozillaBundle_LargeBundle(t *testing.T) {
	// Create a large bundle (simulate real Mozilla bundle size ~200KB)
	largeBundle := bytes.Repeat([]byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"), 1000)

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(largeBundle)),
			}, nil
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx := context.Background()

	result, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
	require.NoError(t, err)
	assert.Equal(t, largeBundle, result)
	assert.Greater(t, len(result), 50000) // Should be > 50KB
}

func TestFetchMozillaBundle_UserAgent(t *testing.T) {
	var capturedUserAgent string

	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			capturedUserAgent = req.Header.Get("User-Agent")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("test")),
			}, nil
		},
	}

	fetcher := NewFetcher(mockClient)
	ctx := context.Background()

	_, err := fetcher.FetchMozillaBundle(ctx, DefaultMozillaBundleURL)
	require.NoError(t, err)
	assert.Equal(t, "verifi/1.0 (certificate management tool)", capturedUserAgent)
}

func TestDefaultMozillaBundleURL(t *testing.T) {
	// Verify the default URL is correct
	assert.Equal(t, "https://curl.se/ca/cacert.pem", DefaultMozillaBundleURL)
}

// errorReader is a helper type that always returns an error on Read
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}
