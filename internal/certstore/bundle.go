package certstore

import (
	"context"
	"path/filepath"
	"strings"

	verifierrors "github.com/princespaghetti/verifi/internal/errors"
)

// readUserCerts reads all PEM certificate files from the certs/user/ directory.
// Returns a slice of certificate data (one entry per file).
func (s *Store) readUserCerts(ctx context.Context) ([][]byte, error) {
	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	userCertsDir := filepath.Join(s.basePath, "certs", "user")

	// Read directory contents
	entries, err := s.fs.ReadDir(userCertsDir)
	if err != nil {
		return nil, &verifierrors.VerifiError{
			Op:   "read user certs directory",
			Path: userCertsDir,
			Err:  err,
		}
	}

	var certData [][]byte

	for _, entry := range entries {
		// Check context periodically
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Skip directories and non-.pem files
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".pem") {
			continue
		}

		// Read certificate file
		certPath := filepath.Join(userCertsDir, entry.Name())
		data, err := s.fs.ReadFile(certPath)
		if err != nil {
			return nil, &verifierrors.VerifiError{
				Op:   "read user certificate",
				Path: certPath,
				Err:  err,
			}
		}

		certData = append(certData, data)
	}

	return certData, nil
}

// userCertPath returns the full path for a user certificate by name
func (s *Store) userCertPath(name string) string {
	return filepath.Join(s.basePath, "certs", "user", name+".pem")
}
