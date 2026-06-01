package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/open-proofline/server/internal/config"
	"github.com/open-proofline/server/internal/incidents"
)

const (
	operatorDefaultLimit              = 25
	operatorDefaultDeletingRetryAfter = 15 * time.Minute
)

type operatorRepository interface {
	Check(ctx context.Context) error
	ListRetentionDeletionCandidates(ctx context.Context, cutoff time.Time, limit int) ([]incidents.RetentionDeletionCandidate, error)
	GetIncidentDeletionJobStatus(ctx context.Context, limit int, staleDeletingBefore time.Time) (incidents.IncidentDeletionJobStatus, error)
}

type operatorRetentionPreviewOutput struct {
	Type                    string                                 `json:"type"`
	MetadataBackend         string                                 `json:"metadata_backend"`
	ReadOnly                bool                                   `json:"read_only"`
	ClosedIncidentRetention string                                 `json:"closed_incident_retention"`
	Cutoff                  time.Time                              `json:"cutoff"`
	Limit                   int                                    `json:"limit"`
	CandidateCount          int                                    `json:"candidate_count"`
	Candidates              []incidents.RetentionDeletionCandidate `json:"candidates"`
}

type operatorDeletionStatusOutput struct {
	Type                string                              `json:"type"`
	MetadataBackend     string                              `json:"metadata_backend"`
	ReadOnly            bool                                `json:"read_only"`
	DeletingRetryAfter  string                              `json:"deleting_retry_after"`
	StaleDeletingBefore time.Time                           `json:"stale_deleting_before"`
	Limit               int                                 `json:"limit"`
	RunnableJobCount    int                                 `json:"runnable_job_count"`
	Status              incidents.IncidentDeletionJobStatus `json:"status"`
}

func runCommand(args []string, stdout io.Writer, logger *slog.Logger) error {
	if len(args) > 0 && args[0] == "operator" {
		return runOperatorCommand(context.Background(), args[1:], stdout)
	}
	return run(logger)
}

func runOperatorCommand(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("operator command required: retention-preview or deletion-status")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	repo, closeRepo, err := newOperatorRepository(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeRepo()

	switch args[0] {
	case "retention-preview":
		return runOperatorRetentionPreview(ctx, args[1:], stdout, cfg, repo)
	case "deletion-status":
		return runOperatorDeletionStatus(ctx, args[1:], stdout, cfg, repo)
	default:
		return fmt.Errorf("unknown operator command %q", args[0])
	}
}

func newOperatorRepository(ctx context.Context, cfg config.Config) (operatorRepository, func(), error) {
	repo, closeRepo, err := newMetadataRepository(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	operatorRepo, ok := repo.(operatorRepository)
	if !ok {
		closeRepo()
		return nil, nil, fmt.Errorf("metadata backend %q does not support operator status", cfg.Backends.Metadata)
	}
	return operatorRepo, closeRepo, nil
}

func runOperatorRetentionPreview(ctx context.Context, args []string, stdout io.Writer, cfg config.Config, repo operatorRepository) error {
	closedIncidentRetention := cfg.ClosedIncidentRetention
	limit := operatorDefaultLimit
	nowText := ""
	flags := flag.NewFlagSet("operator retention-preview", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.DurationVar(&closedIncidentRetention, "closed-incident-retention", closedIncidentRetention, "closed incident retention window to preview")
	flags.IntVar(&limit, "limit", limit, "maximum candidates to include")
	flags.StringVar(&nowText, "now", nowText, "RFC3339 time override for deterministic previews")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("operator retention-preview does not accept positional arguments")
	}
	if closedIncidentRetention <= 0 {
		return fmt.Errorf("operator retention-preview requires --closed-incident-retention or SAFE_CLOSED_INCIDENT_RETENTION")
	}
	if limit <= 0 {
		return fmt.Errorf("operator retention-preview requires a positive --limit")
	}

	now, err := operatorNow(nowText)
	if err != nil {
		return err
	}
	cutoff := now.Add(-closedIncidentRetention)
	candidates, err := repo.ListRetentionDeletionCandidates(ctx, cutoff, limit)
	if err != nil {
		return err
	}

	return writeOperatorJSON(stdout, operatorRetentionPreviewOutput{
		Type:                    "retention_preview",
		MetadataBackend:         cfg.Backends.Metadata,
		ReadOnly:                true,
		ClosedIncidentRetention: closedIncidentRetention.String(),
		Cutoff:                  cutoff,
		Limit:                   limit,
		CandidateCount:          len(candidates),
		Candidates:              candidates,
	})
}

func runOperatorDeletionStatus(ctx context.Context, args []string, stdout io.Writer, cfg config.Config, repo operatorRepository) error {
	limit := operatorDefaultLimit
	deletingRetryAfter := operatorDefaultDeletingRetryAfter
	nowText := ""
	flags := flag.NewFlagSet("operator deletion-status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.IntVar(&limit, "limit", limit, "maximum runnable deletion jobs to include")
	flags.DurationVar(&deletingRetryAfter, "deleting-retry-after", deletingRetryAfter, "age after which deleting jobs are considered retryable")
	flags.StringVar(&nowText, "now", nowText, "RFC3339 time override for deterministic status output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("operator deletion-status does not accept positional arguments")
	}
	if limit <= 0 {
		return fmt.Errorf("operator deletion-status requires a positive --limit")
	}
	if deletingRetryAfter <= 0 {
		return fmt.Errorf("operator deletion-status requires a positive --deleting-retry-after")
	}

	now, err := operatorNow(nowText)
	if err != nil {
		return err
	}
	staleDeletingBefore := now.Add(-deletingRetryAfter)
	status, err := repo.GetIncidentDeletionJobStatus(ctx, limit, staleDeletingBefore)
	if err != nil {
		return err
	}

	return writeOperatorJSON(stdout, operatorDeletionStatusOutput{
		Type:                "deletion_status",
		MetadataBackend:     cfg.Backends.Metadata,
		ReadOnly:            true,
		DeletingRetryAfter:  deletingRetryAfter.String(),
		StaleDeletingBefore: staleDeletingBefore,
		Limit:               limit,
		RunnableJobCount:    len(status.RunnableJobs),
		Status:              status,
	})
}

func operatorNow(value string) (time.Time, error) {
	if value == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse --now: %w", err)
	}
	return parsed.UTC(), nil
}

func writeOperatorJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
