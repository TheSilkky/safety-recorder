package retention

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/storage"
)

func TestRunAndLogDoesNotExposeRawMaintenanceErrors(t *testing.T) {
	var logs bytes.Buffer
	worker := NewWorker(errorRepository{
		err: errors.New("dial private-db.internal:5432 with secret token"),
	}, noopBlobStore{}, Options{
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	})

	worker.runAndLog(context.Background())

	output := logs.String()
	if strings.Contains(output, "private-db.internal") ||
		strings.Contains(output, "secret token") {
		t.Fatalf("maintenance log exposed raw error details: %s", output)
	}
	if !strings.Contains(output, `"error_category":"maintenance_error"`) {
		t.Fatalf("maintenance log omitted safe error category: %s", output)
	}
}

type errorRepository struct {
	err error
}

func (r errorRepository) QueueRetentionIncidentDeletions(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func (r errorRepository) PruneIncidentTokenMetadata(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func (r errorRepository) PruneIncidentDeletionTombstones(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func (r errorRepository) ListRunnableIncidentDeletions(context.Context, int, time.Time) ([]incidents.IncidentDeletionStatus, error) {
	return nil, r.err
}

func (r errorRepository) MarkIncidentDeletionDeleting(context.Context, string, time.Time) (incidents.IncidentDeletionStatus, error) {
	return incidents.IncidentDeletionStatus{}, incidents.ErrNotFound
}

func (r errorRepository) ListIncidentDeletionItems(context.Context, string) ([]incidents.IncidentDeletionItem, error) {
	return nil, nil
}

func (r errorRepository) MarkIncidentDeletionItemDeleted(context.Context, string) error {
	return nil
}

func (r errorRepository) MarkIncidentDeletionItemFailed(context.Context, string, string) error {
	return nil
}

func (r errorRepository) CompleteIncidentDeletion(context.Context, string) (incidents.IncidentDeletionStatus, error) {
	return incidents.IncidentDeletionStatus{}, nil
}

func (r errorRepository) FailIncidentDeletion(context.Context, string, string) (incidents.IncidentDeletionStatus, error) {
	return incidents.IncidentDeletionStatus{}, nil
}

type noopBlobStore struct{}

func (noopBlobStore) Check(context.Context) error {
	return nil
}

func (noopBlobStore) SaveTemp(context.Context, io.Reader, int64) (*storage.TempUpload, error) {
	return nil, nil
}

func (noopBlobStore) CommitTemp(context.Context, *storage.TempUpload, string, string, string, int) (string, error) {
	return "", nil
}

func (noopBlobStore) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (noopBlobStore) Remove(context.Context, string) error {
	return nil
}
