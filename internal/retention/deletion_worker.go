package retention

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/storage"
)

const (
	defaultBatchSize          = 25
	defaultDeletingRetryAfter = 15 * time.Minute
)

// Repository is the metadata boundary required by the deletion worker.
type Repository interface {
	QueueRetentionIncidentDeletions(ctx context.Context, cutoff time.Time, limit int) (int, error)
	PruneIncidentTokenMetadata(ctx context.Context, cutoff time.Time, limit int) (int, error)
	PruneIncidentDeletionTombstones(ctx context.Context, cutoff time.Time, limit int) (int, error)
	ListRunnableIncidentDeletions(ctx context.Context, limit int, staleDeletingBefore time.Time) ([]incidents.IncidentDeletionStatus, error)
	MarkIncidentDeletionDeleting(ctx context.Context, decisionID string, staleDeletingBefore time.Time) (incidents.IncidentDeletionStatus, error)
	ListIncidentDeletionItems(ctx context.Context, decisionID string) ([]incidents.IncidentDeletionItem, error)
	MarkIncidentDeletionItemDeleted(ctx context.Context, itemID string) error
	MarkIncidentDeletionItemFailed(ctx context.Context, itemID, errorCode string) error
	CompleteIncidentDeletion(ctx context.Context, decisionID string) (incidents.IncidentDeletionStatus, error)
	FailIncidentDeletion(ctx context.Context, decisionID, errorCode string) (incidents.IncidentDeletionStatus, error)
}

// Options configures automatic deletion and retention maintenance.
type Options struct {
	Interval                time.Duration
	ClosedIncidentRetention time.Duration
	TokenMetadataRetention  time.Duration
	TombstoneRetention      time.Duration
	DeletingRetryAfter      time.Duration
	BatchSize               int
	Logger                  *slog.Logger
}

// Summary reports non-sensitive worker counts for tests and logs.
type Summary struct {
	RetentionQueued     int
	TokenMetadataPruned int
	TombstonesPruned    int
	Processed           int
	Completed           int
	Failed              int
}

// Worker processes durable incident deletion decisions and optional closed
// incident retention. It never accepts client-provided stored paths.
type Worker struct {
	repo   Repository
	store  storage.BlobStore
	opts   Options
	logger *slog.Logger
}

// NewWorker builds a deletion worker.
func NewWorker(repo Repository, store storage.BlobStore, opts Options) *Worker {
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.DeletingRetryAfter <= 0 {
		opts.DeletingRetryAfter = defaultDeletingRetryAfter
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		repo:   repo,
		store:  store,
		opts:   opts,
		logger: logger,
	}
}

// Start launches the automatic scheduler. A non-positive interval disables the
// loop while leaving explicit deletion decisions durable for a later run.
func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.opts.Interval <= 0 {
		return
	}
	go w.loop(ctx)
}

func (w *Worker) loop(ctx context.Context) {
	w.runAndLog(ctx)
	ticker := time.NewTicker(w.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runAndLog(ctx)
		}
	}
}

func (w *Worker) runAndLog(ctx context.Context) {
	summary, err := w.RunOnce(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Warn("incident deletion maintenance failed", "error_category", deletionMaintenanceErrorCategory(err))
		return
	}
	if summary.RetentionQueued > 0 ||
		summary.TokenMetadataPruned > 0 ||
		summary.TombstonesPruned > 0 ||
		summary.Completed > 0 ||
		summary.Failed > 0 {
		w.logger.Info("incident deletion maintenance completed",
			"retention_queued", summary.RetentionQueued,
			"token_metadata_pruned", summary.TokenMetadataPruned,
			"tombstones_pruned", summary.TombstonesPruned,
			"processed", summary.Processed,
			"completed", summary.Completed,
			"failed", summary.Failed,
		)
	}
}

// RunOnce performs one maintenance pass. Retention only queues closed incidents
// when ClosedIncidentRetention is positive; open incidents are never selected by
// retention.
func (w *Worker) RunOnce(ctx context.Context) (Summary, error) {
	summary := Summary{}
	if w.opts.ClosedIncidentRetention > 0 {
		cutoff := time.Now().UTC().Add(-w.opts.ClosedIncidentRetention)
		queued, err := w.repo.QueueRetentionIncidentDeletions(ctx, cutoff, w.opts.BatchSize)
		if err != nil {
			return summary, err
		}
		summary.RetentionQueued = queued
	}
	if w.opts.TokenMetadataRetention > 0 {
		cutoff := time.Now().UTC().Add(-w.opts.TokenMetadataRetention)
		pruned, err := w.repo.PruneIncidentTokenMetadata(ctx, cutoff, w.opts.BatchSize)
		if err != nil {
			return summary, err
		}
		summary.TokenMetadataPruned = pruned
	}
	if w.opts.TombstoneRetention > 0 {
		cutoff := time.Now().UTC().Add(-w.opts.TombstoneRetention)
		pruned, err := w.repo.PruneIncidentDeletionTombstones(ctx, cutoff, w.opts.BatchSize)
		if err != nil {
			return summary, err
		}
		summary.TombstonesPruned = pruned
	}

	staleDeletingBefore := time.Now().UTC().Add(-w.opts.DeletingRetryAfter)
	jobs, err := w.repo.ListRunnableIncidentDeletions(ctx, w.opts.BatchSize, staleDeletingBefore)
	if err != nil {
		return summary, err
	}
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		completed, err := w.processDeletion(ctx, job.DecisionID, staleDeletingBefore)
		if err != nil {
			return summary, err
		}
		summary.Processed++
		if completed {
			summary.Completed++
		} else {
			summary.Failed++
		}
	}
	return summary, nil
}

func (w *Worker) processDeletion(ctx context.Context, decisionID string, staleDeletingBefore time.Time) (bool, error) {
	status, err := w.repo.MarkIncidentDeletionDeleting(ctx, decisionID, staleDeletingBefore)
	if errors.Is(err, incidents.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	items, err := w.repo.ListIncidentDeletionItems(ctx, status.DecisionID)
	if err != nil {
		return false, err
	}

	failed := false
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if err := w.store.Remove(ctx, item.StoredPath); err != nil {
			if errors.Is(err, context.Canceled) {
				return false, err
			}
			if errors.Is(err, os.ErrNotExist) {
				if err := w.repo.MarkIncidentDeletionItemDeleted(ctx, item.ID); err != nil {
					return false, err
				}
				continue
			}
			failed = true
			if err := w.repo.MarkIncidentDeletionItemFailed(ctx, item.ID, deletionErrorCode(err)); err != nil {
				return false, err
			}
			continue
		}
		if err := w.repo.MarkIncidentDeletionItemDeleted(ctx, item.ID); err != nil {
			return false, err
		}
	}

	if failed {
		_, err := w.repo.FailIncidentDeletion(ctx, status.DecisionID, "blob_delete_failed")
		return false, err
	}
	if _, err := w.repo.CompleteIncidentDeletion(ctx, status.DecisionID); err != nil {
		return false, err
	}
	return true, nil
}

func deletionErrorCode(err error) string {
	switch {
	case errors.Is(err, storage.ErrUnsafePath):
		return "unsafe_stored_path"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "blob_delete_failed"
	}
}

func deletionMaintenanceErrorCategory(err error) string {
	switch {
	case err == nil:
		return "unknown"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, storage.ErrUnsafePath):
		return "unsafe_path"
	case errors.Is(err, storage.ErrAlreadyExists):
		return "already_exists"
	case errors.Is(err, incidents.ErrNotFound):
		return "not_found"
	case errors.Is(err, incidents.ErrInvalidState):
		return "invalid_state"
	case errors.Is(err, os.ErrNotExist):
		return "not_found"
	case errors.Is(err, os.ErrPermission):
		return "permission"
	default:
		return "maintenance_error"
	}
}
