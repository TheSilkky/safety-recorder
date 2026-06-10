package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// TempUpload describes a staged upload and the hash computed while it was read.
type TempUpload struct {
	Path      string
	ByteSize  int64
	SHA256Hex string
}

type tempStagingQuota struct {
	maxBytes int64
	mu       sync.Mutex
}

func newTempStagingQuota(maxBytes int64) *tempStagingQuota {
	if maxBytes <= 0 {
		return nil
	}
	return &tempStagingQuota{maxBytes: maxBytes}
}

// SaveTemp streams reader into a temporary file, enforcing maxBytes and
// computing SHA-256 without buffering the upload in memory.
func (s *Store) SaveTemp(ctx context.Context, reader io.Reader, maxBytes int64) (*TempUpload, error) {
	return saveTempToDir(ctx, s.tempDir, reader, maxBytes, s.tempQuota)
}

// TempStagingUsage returns aggregate temporary upload staging usage when a
// quota is configured. It never returns file names or paths.
func (s *Store) TempStagingUsage(ctx context.Context) (usedBytes, quotaBytes int64, quotaConfigured bool, err error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, false, err
	}
	if s.tempQuota == nil {
		return 0, 0, false, nil
	}
	usedBytes, err = tempStagingUsedBytes(s.tempDir)
	if err != nil {
		return 0, 0, true, err
	}
	return usedBytes, s.tempQuota.maxBytes, true, ctx.Err()
}

func saveTempToDir(ctx context.Context, tempDir string, reader io.Reader, maxBytes int64, quota *tempStagingQuota) (*TempUpload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if quota != nil {
		quota.mu.Lock()
		defer quota.mu.Unlock()
	}
	effectiveMaxBytes := maxBytes
	if quota != nil {
		remaining, err := tempStagingRemainingBytes(tempDir, quota.maxBytes)
		if err != nil {
			return nil, fmt.Errorf("read temp upload staging usage: %w", err)
		}
		if remaining <= 0 {
			return nil, ErrTempStagingQuotaExceeded
		}
		if remaining < effectiveMaxBytes {
			effectiveMaxBytes = remaining
		}
	}

	file, err := os.CreateTemp(tempDir, "upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temp upload: %w", err)
	}
	tempPath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}

	hash := sha256.New()
	// Read one byte past the limit so an oversized upload is detected without
	// accepting a truncated file.
	limited := &io.LimitedReader{R: reader, N: readLimitPlusOne(effectiveMaxBytes)}
	byteSize, copyErr := io.Copy(io.MultiWriter(file, hash), limited)
	if copyErr != nil {
		cleanup()
		return nil, fmt.Errorf("write temp upload: %w", copyErr)
	}
	if byteSize > effectiveMaxBytes {
		cleanup()
		if quota != nil && effectiveMaxBytes < maxBytes {
			return nil, ErrTempStagingQuotaExceeded
		}
		return nil, ErrTooLarge
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return nil, fmt.Errorf("sync temp upload: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("close temp upload: %w", err)
	}

	return &TempUpload{
		Path:      tempPath,
		ByteSize:  byteSize,
		SHA256Hex: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func tempStagingRemainingBytes(tempDir string, quotaBytes int64) (int64, error) {
	usedBytes, err := tempStagingUsedBytes(tempDir)
	if err != nil {
		return 0, err
	}
	if usedBytes >= quotaBytes {
		return 0, nil
	}
	return quotaBytes - usedBytes, nil
}

func tempStagingUsedBytes(tempDir string) (int64, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return 0, err
	}
	var usedBytes int64
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "upload-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		size := info.Size()
		if size <= 0 {
			continue
		}
		if usedBytes > quotaBytesSaturation-size {
			return quotaBytesSaturation, nil
		}
		usedBytes += size
	}
	return usedBytes, nil
}

const quotaBytesSaturation = int64(1<<63 - 1)

func readLimitPlusOne(maxBytes int64) int64 {
	if maxBytes >= quotaBytesSaturation {
		return quotaBytesSaturation
	}
	return maxBytes + 1
}

// Cleanup removes the staged file if it has not already been committed.
func (u *TempUpload) Cleanup() {
	if u == nil || u.Path == "" {
		return
	}
	_ = os.Remove(u.Path)
	u.Path = ""
}
