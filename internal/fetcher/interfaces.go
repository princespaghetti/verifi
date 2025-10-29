package fetcher

import "net/http"

// HTTPClient is an interface for making HTTP requests.
// This interface allows for easy mocking in tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
