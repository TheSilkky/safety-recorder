package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errInvalidTempCleanupAge = errors.New("temp upload cleanup minimum age must be positive")

// TempCleanupOptions configures conservative cleanup for orphaned local upload
// staging files. Cleanup is limited to regular files created by SaveTemp.
type TempCleanupOptions struct {
	MinAge time.Duration
	DryRun bool
	Now    time.Time
}

// TempCleanupSummary contains only safe counts for operator logs.
type TempCleanupSummary struct {
	Scanned       int
	Eligible      int
	Removed       int
	SkippedActive int
	SkippedOther  int
	Errors        int
}

// TempCleaner is implemented by blob stores that use local upload staging.
type TempCleaner interface {
	CleanupTemp(ctx context.Context, opts TempCleanupOptions) (TempCleanupSummary, error)
}

// CleanupTemp removes old orphaned upload temp files from local storage.
func (s *Store) CleanupTemp(ctx context.Context, opts TempCleanupOptions) (TempCleanupSummary, error) {
	return cleanupTempDir(ctx, s.tempDir, opts)
}

// CleanupTemp removes old orphaned upload temp files from S3 local staging.
func (s *S3Store) CleanupTemp(ctx context.Context, opts TempCleanupOptions) (TempCleanupSummary, error) {
	return cleanupTempDir(ctx, s.tempDir, opts)
}

func cleanupTempDir(ctx context.Context, tempDir string, opts TempCleanupOptions) (TempCleanupSummary, error) {
	if opts.MinAge <= 0 {
		return TempCleanupSummary{}, errInvalidTempCleanupAge
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := ctx.Err(); err != nil {
		return TempCleanupSummary{}, err
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return TempCleanupSummary{}, fmt.Errorf("read temp upload directory: %w", err)
	}

	summary := TempCleanupSummary{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		summary.Scanned++
		if !strings.HasPrefix(entry.Name(), "upload-") {
			summary.SkippedOther++
			continue
		}
		info, err := entry.Info()
		if err != nil {
			summary.Errors++
			continue
		}
		if !info.Mode().IsRegular() {
			summary.SkippedOther++
			continue
		}
		if now.Sub(info.ModTime()) < opts.MinAge {
			summary.SkippedActive++
			continue
		}

		summary.Eligible++
		if opts.DryRun {
			continue
		}
		if err := os.Remove(filepath.Join(tempDir, entry.Name())); err != nil {
			summary.Errors++
			continue
		}
		summary.Removed++
	}
	return summary, nil
}
