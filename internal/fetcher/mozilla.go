package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const (
	// DefaultMozillaBundleURL is the default URL for the Mozilla CA bundle.
	DefaultMozillaBundleURL = "https://curl.se/ca/cacert.pem"
)

// Fetcher handles downloading Mozilla CA bundles.
type Fetcher struct {
	client HTTPClient
}

// NewFetcher creates a new Fetcher with the given HTTP client.
// If client is nil, uses http.DefaultClient.
func NewFetcher(client HTTPClient) *Fetcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &Fetcher{
		client: client,
	}
}

// FetchMozillaBundle downloads the Mozilla CA bundle from the specified URL.
// The context can be used to cancel the download or set a timeout.
func (f *Fetcher) FetchMozillaBundle(ctx context.Context, url string) ([]byte, error) {
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set User-Agent to identify ourselves
	req.Header.Set("User-Agent", "verifi/1.0 (certificate management tool)")

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download bundle: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // Ignore close error - standard practice

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("downloaded bundle is empty")
	}

	return data, nil
}
