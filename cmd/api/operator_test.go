package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/config"
	"github.com/open-proofline/server/internal/incidents"
)

func TestRunOperatorRetentionPreviewOutputsSafeJSON(t *testing.T) {
	ctx := context.Background()
	repo := &fakeOperatorRepository{
		candidates: []incidents.RetentionDeletionCandidate{
			{
				IncidentID: "inc_candidate",
				UpdatedAt:  time.Date(2026, 5, 30, 9, 0, 0, 0, time.UTC),
			},
		},
	}
	var out bytes.Buffer

	err := runOperatorRetentionPreview(ctx, []string{
		"--closed-incident-retention", "24h",
		"--limit", "5",
		"--now", "2026-06-01T10:00:00Z",
	}, &out, config.Config{
		Backends: config.BackendSelection{Metadata: config.MetadataBackendSQLite},
	}, repo)
	if err != nil {
		t.Fatalf("run retention preview: %v", err)
	}

	var decoded operatorRetentionPreviewOutput
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if decoded.Type != "retention_preview" ||
		!decoded.ReadOnly ||
		decoded.ClosedIncidentRetention != "24h0m0s" ||
		decoded.CandidateCount != 1 ||
		len(decoded.Candidates) != 1 ||
		decoded.Candidates[0].IncidentID != "inc_candidate" {
		t.Fatalf("unexpected retention preview output: %+v", decoded)
	}
	if repo.cutoff != time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC) || repo.limit != 5 {
		t.Fatalf("repo call cutoff=%s limit=%d", repo.cutoff, repo.limit)
	}
	assertOperatorOutputSafe(t, out.String())
}

func TestRunOperatorRetentionPreviewRequiresRetentionWindow(t *testing.T) {
	err := runOperatorRetentionPreview(context.Background(), nil, &bytes.Buffer{}, config.Config{}, &fakeOperatorRepository{})
	if err == nil || !strings.Contains(err.Error(), "closed-incident-retention") {
		t.Fatalf("expected retention window error, got %v", err)
	}
	var operatorErr operatorError
	if !errors.As(err, &operatorErr) {
		t.Fatalf("expected operator error wrapper, got %T", err)
	}
	if operatorErr.operation != "retention_preview" || operatorErr.category != "invalid_config_value" {
		t.Fatalf("operator error operation=%q category=%q", operatorErr.operation, operatorErr.category)
	}
}

func TestOperatorErrorLogUsesSafeOperatorFields(t *testing.T) {
	ctx := context.Background()
	repo := &fakeOperatorRepository{
		err: errors.New("metadata dependency failure with <private endpoint> and <credential>"),
	}
	var out bytes.Buffer

	err := runOperatorRetentionPreview(ctx, []string{
		"--closed-incident-retention", "24h",
		"--now", "2026-06-01T10:00:00Z",
	}, &out, config.Config{
		Backends: config.BackendSelection{Metadata: config.MetadataBackendSQLite},
	}, repo)
	if err == nil {
		t.Fatal("expected retention preview metadata error")
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	logCommandError(logger, err)

	for _, want := range []string{
		"component=operator",
		"operation=retention_preview",
		"status=failed",
		"error_category=metadata",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("operator log omitted %q: %s", want, logs.String())
		}
	}
	for _, disallowed := range []string{
		"component=startup",
		"<private endpoint>",
		"<credential>",
		"err=",
	} {
		if strings.Contains(logs.String(), disallowed) {
			t.Fatalf("operator log exposed %q: %s", disallowed, logs.String())
		}
	}
}

func TestRunOperatorDeletionStatusOutputsSafeRetryCategories(t *testing.T) {
	ctx := context.Background()
	repo := &fakeOperatorRepository{
		status: incidents.IncidentDeletionJobStatus{
			DecisionStateCounts: []incidents.IncidentDeletionStateCount{
				{State: incidents.IncidentDeletionStateFailed, Count: 1},
			},
			DecisionErrorCounts: []incidents.IncidentDeletionErrorCount{
				{State: incidents.IncidentDeletionStateFailed, ErrorCode: "blob_delete_failed", Count: 1},
			},
			ItemStateCounts: []incidents.IncidentDeletionItemStateCount{
				{State: incidents.IncidentDeletionItemStateFailed, ErrorCode: "unsafe_stored_path", Count: 1},
			},
			RunnableJobs: []incidents.IncidentDeletionStatus{
				{
					DecisionID: "del_safe",
					IncidentID: "inc_safe",
					State:      incidents.IncidentDeletionStateFailed,
					ErrorCode:  "blob_delete_failed",
				},
			},
		},
	}
	var out bytes.Buffer

	err := runOperatorDeletionStatus(ctx, []string{
		"--limit", "7",
		"--deleting-retry-after", "30m",
		"--now", "2026-06-01T10:00:00Z",
	}, &out, config.Config{
		Backends: config.BackendSelection{Metadata: config.MetadataBackendSQLite},
	}, repo)
	if err != nil {
		t.Fatalf("run deletion status: %v", err)
	}

	var decoded operatorDeletionStatusOutput
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if decoded.Type != "deletion_status" ||
		!decoded.ReadOnly ||
		decoded.DeletingRetryAfter != "30m0s" ||
		decoded.RunnableJobCount != 1 ||
		len(decoded.Status.ItemStateCounts) != 1 ||
		decoded.Status.ItemStateCounts[0].ErrorCode != "unsafe_stored_path" {
		t.Fatalf("unexpected deletion status output: %+v", decoded)
	}
	if repo.staleDeletingBefore != time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC) || repo.limit != 7 {
		t.Fatalf("repo call stale=%s limit=%d", repo.staleDeletingBefore, repo.limit)
	}
	assertOperatorOutputSafe(t, out.String())
}

type fakeOperatorRepository struct {
	candidates          []incidents.RetentionDeletionCandidate
	status              incidents.IncidentDeletionJobStatus
	err                 error
	cutoff              time.Time
	staleDeletingBefore time.Time
	limit               int
}

func (r *fakeOperatorRepository) Check(context.Context) error {
	return r.err
}

func (r *fakeOperatorRepository) ListRetentionDeletionCandidates(_ context.Context, cutoff time.Time, limit int) ([]incidents.RetentionDeletionCandidate, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.cutoff = cutoff
	r.limit = limit
	return r.candidates, nil
}

func (r *fakeOperatorRepository) GetIncidentDeletionJobStatus(_ context.Context, limit int, staleDeletingBefore time.Time) (incidents.IncidentDeletionJobStatus, error) {
	if r.err != nil {
		return incidents.IncidentDeletionJobStatus{}, r.err
	}
	r.limit = limit
	r.staleDeletingBefore = staleDeletingBefore
	return r.status, nil
}

func assertOperatorOutputSafe(t *testing.T, output string) {
	t.Helper()
	for _, unsafe := range []string{
		`"stored_path"`,
		`"object_key"`,
		`"token_hash"`,
		"Authorization",
		"plaintext",
		"raw_key",
		`"original_filename"`,
		`"latitude"`,
		`"longitude"`,
		`"notes"`,
	} {
		if strings.Contains(output, unsafe) {
			t.Fatalf("operator output exposed %q: %s", unsafe, output)
		}
	}
}
