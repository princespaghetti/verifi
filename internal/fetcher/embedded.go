// Package fetcher handles fetching and managing CA certificate bundles.
package fetcher

import (
	_ "embed"
)

// embeddedMozillaBundle contains the Mozilla CA bundle embedded in the binary.
// This is downloaded once during development from https://curl.se/ca/cacert.pem
// and embedded at build time using go:embed.
//
//go:embed assets/mozilla-ca-bundle.pem
var embeddedMozillaBundle []byte

// GetEmbeddedBundle returns the embedded Mozilla CA bundle.
// This allows verifi to work completely offline without network access.
func GetEmbeddedBundle() []byte {
	return embeddedMozillaBundle
}
