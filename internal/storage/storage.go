package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	// ErrTooLarge indicates that an upload exceeded its configured byte limit.
	ErrTooLarge = errors.New("upload too large")
	// ErrTempStagingQuotaExceeded indicates that local upload staging is full.
	ErrTempStagingQuotaExceeded = errors.New("temp upload staging quota exceeded")
	// ErrAlreadyExists indicates that committing a temp upload would overwrite
	// an existing immutable chunk file.
	ErrAlreadyExists = errors.New("stored chunk already exists")
	// ErrUnsafePath indicates that a requested storage path could escape the
	// configured data directory.
	ErrUnsafePath = errors.New("unsafe storage path")
)

// Options configures local blob storage.
type Options struct {
	TempStagingQuotaBytes int64
}

// Store manages temporary and final blob files under one data directory.
type Store struct {
	dataDir   string
	tempDir   string
	tempQuota *tempStagingQuota
}

// New creates the data and temporary directories used for encrypted blob
// storage.
func New(dataDir string) (*Store, error) {
	return NewWithOptions(dataDir, Options{})
}

// NewWithOptions creates local encrypted blob storage with explicit options.
func NewWithOptions(dataDir string, opts Options) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	tempDir := filepath.Join(dataDir, "tmp")
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}
	return &Store{
		dataDir:   dataDir,
		tempDir:   tempDir,
		tempQuota: newTempStagingQuota(opts.TempStagingQuotaBytes),
	}, nil
}

// Check verifies that local blob storage and temp staging directories are
// present and writable.
func (s *Store) Check(ctx context.Context) error {
	if err := checkWritableDir(ctx, s.dataDir); err != nil {
		return fmt.Errorf("check local blob storage: %w", err)
	}
	if err := checkWritableDir(ctx, s.tempDir); err != nil {
		return fmt.Errorf("check local temp storage: %w", err)
	}
	return nil
}

func checkWritableDir(ctx context.Context, dir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}
	file, err := os.CreateTemp(dir, ".proofline-check-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Remove(name); err != nil {
		return err
	}
	return ctx.Err()
}
