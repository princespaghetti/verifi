package certstore

import (
	"context"
	"fmt"
	"time"

	"github.com/gofrs/flock"
)

// FileLock provides cross-platform file locking using flock.
type FileLock struct {
	lock *flock.Flock
}

// NewFileLock creates a new file lock for the given path.
// The lock file will be created at path + ".lock".
func NewFileLock(path string) *FileLock {
	return &FileLock{
		lock: flock.New(path + ".lock"),
	}
}

// Lock acquires the file lock with context support.
// It will retry with a 100ms interval until the context is cancelled or the lock is acquired.
func (l *FileLock) Lock(ctx context.Context) error {
	locked, err := l.lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("failed to acquire lock: timeout")
	}
	return nil
}

// Unlock releases the file lock.
func (l *FileLock) Unlock() error {
	return l.lock.Unlock()
}
