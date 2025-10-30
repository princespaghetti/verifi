package certstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileLock_LockUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	lock := NewFileLock(lockPath)

	// Test basic lock
	ctx := context.Background()
	if err := lock.Lock(ctx); err != nil {
		t.Fatalf("Lock() failed: %v", err)
	}

	// Verify lock file was created
	lockFile := lockPath + ".lock"
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		t.Error("Lock file was not created")
	}

	// Test unlock
	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock() failed: %v", err)
	}
}

func TestFileLock_ContextTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	lock1 := NewFileLock(lockPath)
	lock2 := NewFileLock(lockPath)

	// Acquire lock with first instance
	ctx1 := context.Background()
	if err := lock1.Lock(ctx1); err != nil {
		t.Fatalf("First Lock() failed: %v", err)
	}
	defer lock1.Unlock()

	// Try to acquire with second instance with short timeout
	ctx2, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := lock2.Lock(ctx2)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Second Lock() should have failed due to timeout")
		lock2.Unlock()
	}

	// Should have timed out around 300ms
	if elapsed < 200*time.Millisecond {
		t.Errorf("Lock timeout was too quick: %v", elapsed)
	}
	if elapsed > 600*time.Millisecond {
		t.Errorf("Lock timeout was too slow: %v", elapsed)
	}
}

func TestFileLock_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	const numGoroutines = 10
	const incrementsPerGoroutine = 100

	var counter int32
	var wg sync.WaitGroup
	var errCount int32

	wg.Add(numGoroutines)

	// Launch multiple goroutines that increment a counter
	// Only the lock protects the counter
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < incrementsPerGoroutine; j++ {
				lock := NewFileLock(lockPath)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

				if err := lock.Lock(ctx); err != nil {
					atomic.AddInt32(&errCount, 1)
					cancel()
					return
				}

				// Critical section - increment counter
				temp := atomic.LoadInt32(&counter)
				time.Sleep(1 * time.Millisecond) // Simulate work
				atomic.StoreInt32(&counter, temp+1)

				lock.Unlock()
				cancel()
			}
		}()
	}

	wg.Wait()

	// Check for errors
	if errCount > 0 {
		t.Errorf("Lock() failed %d times in goroutines", errCount)
	}

	expected := int32(numGoroutines * incrementsPerGoroutine)
	finalCounter := atomic.LoadInt32(&counter)
	if finalCounter != expected {
		t.Errorf("Counter = %d, want %d (race condition detected)", finalCounter, expected)
	}
}

func TestFileLock_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	lock1 := NewFileLock(lockPath)
	lock2 := NewFileLock(lockPath)

	// Acquire lock with first instance
	ctx1 := context.Background()
	if err := lock1.Lock(ctx1); err != nil {
		t.Fatalf("First Lock() failed: %v", err)
	}
	defer lock1.Unlock()

	// Create cancellable context for second lock attempt
	ctx2, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := lock2.Lock(ctx2)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Second Lock() should have failed due to cancellation")
		lock2.Unlock()
	}

	// Should have been cancelled around 200ms
	if elapsed < 100*time.Millisecond {
		t.Errorf("Lock cancellation was too quick: %v", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Lock cancellation was too slow: %v", elapsed)
	}

	// Verify the error is context.Canceled (or wraps it)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestFileLock_AlreadyLocked(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	lock1 := NewFileLock(lockPath)

	// Acquire lock
	ctx := context.Background()
	if err := lock1.Lock(ctx); err != nil {
		t.Fatalf("First Lock() failed: %v", err)
	}
	defer lock1.Unlock()

	// Try to acquire the same lock again from different instance
	lock2 := NewFileLock(lockPath)
	ctx2, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := lock2.Lock(ctx2)
	if err == nil {
		t.Error("Second Lock() should have failed because file is already locked")
		lock2.Unlock()
	}
}

func TestFileLock_SequentialAccess(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	// Test that locks can be acquired and released sequentially
	for i := 0; i < 5; i++ {
		lock := NewFileLock(lockPath)
		ctx := context.Background()

		if err := lock.Lock(ctx); err != nil {
			t.Fatalf("Lock() iteration %d failed: %v", i, err)
		}

		// Do some work
		time.Sleep(10 * time.Millisecond)

		if err := lock.Unlock(); err != nil {
			t.Fatalf("Unlock() iteration %d failed: %v", i, err)
		}
	}
}

func TestFileLock_MultipleFiles(t *testing.T) {
	// Test that different lock files don't interfere with each other
	tmpDir := t.TempDir()

	lock1 := NewFileLock(filepath.Join(tmpDir, "lock1"))
	lock2 := NewFileLock(filepath.Join(tmpDir, "lock2"))

	ctx := context.Background()

	// Both locks should succeed simultaneously
	if err := lock1.Lock(ctx); err != nil {
		t.Fatalf("Lock1() failed: %v", err)
	}
	defer lock1.Unlock()

	if err := lock2.Lock(ctx); err != nil {
		t.Fatalf("Lock2() failed: %v", err)
	}
	defer lock2.Unlock()

	// Both should be held at the same time
	t.Log("Both locks acquired successfully")
}
