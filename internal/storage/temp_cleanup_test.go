package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanupTempRemovesOnlyOldUploadFiles(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	store, err := New(dataDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	tempDir := filepath.Join(dataDir, "tmp")
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

	oldUpload := createCleanupTempUpload(t, ctx, store, now.Add(-2*time.Hour))
	activeUpload := createCleanupTempUpload(t, ctx, store, now.Add(-10*time.Minute))
	nonUpload := filepath.Join(tempDir, "operator-note")
	if err := os.WriteFile(nonUpload, []byte("not an upload temp"), 0o600); err != nil {
		t.Fatalf("write non-upload temp file: %v", err)
	}
	if err := os.Chtimes(nonUpload, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("age non-upload temp file: %v", err)
	}

	committedUpload, err := store.SaveTemp(ctx, strings.NewReader("committed"), int64(len("committed")))
	if err != nil {
		t.Fatalf("save committed upload temp: %v", err)
	}
	storedPath, err := store.CommitTemp(ctx, committedUpload, "inc_cleanup", "str_cleanup", "audio", 1)
	if err != nil {
		t.Fatalf("commit upload temp: %v", err)
	}

	summary, err := store.CleanupTemp(ctx, TempCleanupOptions{
		MinAge: time.Hour,
		Now:    now,
	})
	if err != nil {
		t.Fatalf("cleanup temp: %v", err)
	}
	if summary.Scanned != 3 || summary.Eligible != 1 || summary.Removed != 1 || summary.SkippedActive != 1 || summary.SkippedOther != 1 || summary.Errors != 0 {
		t.Fatalf("unexpected cleanup summary: %+v", summary)
	}
	assertPathMissing(t, oldUpload.Path)
	assertPathExists(t, activeUpload.Path)
	assertPathExists(t, nonUpload)
	reader, err := store.Open(ctx, storedPath)
	if err != nil {
		t.Fatalf("open committed blob after temp cleanup: %v", err)
	}
	_ = reader.Close()
}

func TestCleanupTempDryRunPreservesEligibleFiles(t *testing.T) {
	ctx := context.Background()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	oldUpload := createCleanupTempUpload(t, ctx, store, now.Add(-2*time.Hour))

	summary, err := store.CleanupTemp(ctx, TempCleanupOptions{
		MinAge: time.Hour,
		DryRun: true,
		Now:    now,
	})
	if err != nil {
		t.Fatalf("dry-run cleanup temp: %v", err)
	}
	if summary.Eligible != 1 || summary.Removed != 0 || summary.Errors != 0 {
		t.Fatalf("unexpected dry-run cleanup summary: %+v", summary)
	}
	assertPathExists(t, oldUpload.Path)
}

func TestCleanupTempRejectsMissingMinimumAge(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	_, err = store.CleanupTemp(context.Background(), TempCleanupOptions{})
	if !errors.Is(err, errInvalidTempCleanupAge) {
		t.Fatalf("cleanup temp error = %v, want invalid age", err)
	}
}

func TestCleanupTempSkipsNonRegularUploadEntries(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	store, err := New(dataDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	tempDir := filepath.Join(dataDir, "tmp")
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

	uploadDir := filepath.Join(tempDir, "upload-directory")
	if err := os.Mkdir(uploadDir, 0o700); err != nil {
		t.Fatalf("create upload-like directory: %v", err)
	}
	target := filepath.Join(dataDir, "outside-target")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	uploadLink := filepath.Join(tempDir, "upload-link")
	if err := os.Symlink(target, uploadLink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	summary, err := store.CleanupTemp(ctx, TempCleanupOptions{
		MinAge: time.Hour,
		Now:    now,
	})
	if err != nil {
		t.Fatalf("cleanup temp: %v", err)
	}
	if summary.Scanned != 2 || summary.SkippedOther != 2 || summary.Removed != 0 {
		t.Fatalf("unexpected non-regular cleanup summary: %+v", summary)
	}
	assertPathExists(t, uploadDir)
	assertPathExists(t, uploadLink)
	assertPathExists(t, target)
}

func TestS3StoreCleanupTempUsesLocalStagingDir(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	store, err := newS3Store(newFakeS3Client(), S3Options{
		Bucket:  "proofline-evidence",
		TempDir: tempDir,
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	oldUpload, err := store.SaveTemp(ctx, strings.NewReader("staged"), int64(len("staged")))
	if err != nil {
		t.Fatalf("save s3 staged temp: %v", err)
	}
	if err := os.Chtimes(oldUpload.Path, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("age s3 staged temp: %v", err)
	}

	summary, err := store.CleanupTemp(ctx, TempCleanupOptions{
		MinAge: time.Hour,
		Now:    now,
	})
	if err != nil {
		t.Fatalf("cleanup s3 temp: %v", err)
	}
	if summary.Eligible != 1 || summary.Removed != 1 || summary.Errors != 0 {
		t.Fatalf("unexpected s3 temp cleanup summary: %+v", summary)
	}
	assertPathMissing(t, oldUpload.Path)
}

func createCleanupTempUpload(t *testing.T, ctx context.Context, store *Store, modTime time.Time) *TempUpload {
	t.Helper()
	upload, err := store.SaveTemp(ctx, strings.NewReader("staged upload"), int64(len("staged upload")))
	if err != nil {
		t.Fatalf("save cleanup temp upload: %v", err)
	}
	if err := os.Chtimes(upload.Path, modTime, modTime); err != nil {
		t.Fatalf("age cleanup temp upload: %v", err)
	}
	return upload
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected path to exist: %v", err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path exists after cleanup or returned unexpected error: %v", err)
	}
}
