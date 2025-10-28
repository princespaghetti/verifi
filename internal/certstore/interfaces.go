// Package certstore provides certificate store management functionality.
package certstore

import (
	"context"
	"io/fs"
	"os"
)

// Locker provides file locking for concurrent access safety.
type Locker interface {
	Lock(ctx context.Context) error
	Unlock() error
}

// FileSystem abstracts file system operations for testing.
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	Rename(oldpath, newpath string) error
	Stat(path string) (fs.FileInfo, error)
	ReadDir(path string) ([]fs.DirEntry, error)
}

// OSFileSystem is the production implementation of FileSystem.
type OSFileSystem struct{}

// ReadFile reads the file at the given path.
func (fs *OSFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes data to the file at the given path.
func (fs *OSFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// MkdirAll creates a directory and all parent directories.
func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Remove removes the file or directory at the given path.
func (fs *OSFileSystem) Remove(path string) error {
	return os.Remove(path)
}

// Rename renames (moves) oldpath to newpath.
// This operation is atomic on POSIX systems.
func (fs *OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// Stat returns file info for the given path.
func (fs *OSFileSystem) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// ReadDir reads the directory at the given path.
func (fs *OSFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}
