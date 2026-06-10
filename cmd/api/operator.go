package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
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
	ListModeAwareRetentionPreviewIncidents(ctx context.Context, limit int) ([]incidents.ModeAwareRetentionPreviewIncident, error)
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

type operatorModeAwareRetentionPreviewOutput struct {
	Type             string                                             `json:"type"`
	MetadataBackend  string                                             `json:"metadata_backend"`
	ReadOnly         bool                                               `json:"read_only"`
	LiveDeletion     bool                                               `json:"live_deletion"`
	PolicyWindows    map[string]string                                  `json:"policy_windows"`
	Limit            int                                                `json:"limit"`
	CandidateCount   int                                                `json:"candidate_count"`
	IneligibleCount  int                                                `json:"ineligible_count"`
	CandidatesByMode map[string][]incidents.ModeAwareRetentionCandidate `json:"candidates_by_policy_class"`
	Ineligible       []incidents.ModeAwareRetentionIneligible           `json:"ineligible"`
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

type operatorError struct {
	operation string
	category  string
	err       error
}

func (e operatorError) Error() string {
	return e.err.Error()
}

func (e operatorError) Unwrap() error {
	return e.err
}

func withOperatorError(operation, category string, err error) error {
	if err == nil {
		return nil
	}
	return operatorError{operation: operation, category: category, err: err}
}

func safeOperatorOperation(err error) string {
	var operatorErr operatorError
	if errors.As(err, &operatorErr) && operatorErr.operation != "" {
		return operatorErr.operation
	}
	return "unknown"
}

func safeOperatorErrorCategory(err error) string {
	var operatorErr operatorError
	if errors.As(err, &operatorErr) && operatorErr.category != "" {
		return operatorErr.category
	}
	return "unknown"
}

func runCommand(args []string, stdout io.Writer, logger *slog.Logger) error {
	configFilePath, args, err := extractConfigFlag(args)
	if err != nil {
		return withStartupStage(startupStageArgsParse, err)
	}
	if len(args) > 0 && args[0] == "operator" {
		return runOperatorCommand(context.Background(), args[1:], stdout, configFilePath)
	}
	if len(args) != 0 {
		return withStartupStage(startupStageArgsParse, fmt.Errorf("unknown command or flag %q", args[0]))
	}
	return run(logger, configFilePath)
}

func runOperatorCommand(ctx context.Context, args []string, stdout io.Writer, configFilePath string) error {
	var err error
	configFilePath, args, err = extractConfigFlagWithExisting(args, configFilePath)
	if err != nil {
		return withStartupStage(startupStageArgsParse, err)
	}
	if len(args) == 0 {
		return withStartupStage(startupStageArgsParse, fmt.Errorf("operator command required: retention-preview, mode-retention-preview, or deletion-status"))
	}

	cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigFilePath: configFilePath})
	if err != nil {
		return withStartupStage(startupStageConfigLoad, err)
	}
	repo, closeRepo, err := newOperatorRepository(ctx, cfg)
	if err != nil {
		return withStartupStage(startupStageMetadataOpen, err)
	}
	defer closeRepo()

	switch args[0] {
	case "retention-preview":
		return runOperatorRetentionPreview(ctx, args[1:], stdout, cfg, repo)
	case "mode-retention-preview":
		return runOperatorModeAwareRetentionPreview(ctx, args[1:], stdout, cfg, repo)
	case "deletion-status":
		return runOperatorDeletionStatus(ctx, args[1:], stdout, cfg, repo)
	default:
		return withStartupStage(startupStageArgsParse, fmt.Errorf("unknown operator command %q", args[0]))
	}
}

func extractConfigFlag(args []string) (string, []string, error) {
	return extractConfigFlagWithExisting(args, "")
}

func extractConfigFlagWithExisting(args []string, existing string) (string, []string, error) {
	configFilePath := existing
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--config requires a path")
			}
			if configFilePath != "" {
				return "", nil, fmt.Errorf("--config may be set only once")
			}
			configFilePath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--config="):
			if configFilePath != "" {
				return "", nil, fmt.Errorf("--config may be set only once")
			}
			configFilePath = strings.TrimPrefix(arg, "--config=")
			if configFilePath == "" {
				return "", nil, fmt.Errorf("--config requires a path")
			}
		default:
			rest = append(rest, arg)
		}
	}
	return configFilePath, rest, nil
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
		return withOperatorError("retention_preview", "invalid_config_value", err)
	}
	if flags.NArg() != 0 {
		return withOperatorError("retention_preview", "invalid_config_value", fmt.Errorf("operator retention-preview does not accept positional arguments"))
	}
	if closedIncidentRetention <= 0 {
		return withOperatorError("retention_preview", "invalid_config_value", fmt.Errorf("operator retention-preview requires --closed-incident-retention or SAFE_CLOSED_INCIDENT_RETENTION"))
	}
	if limit <= 0 {
		return withOperatorError("retention_preview", "invalid_config_value", fmt.Errorf("operator retention-preview requires a positive --limit"))
	}

	now, err := operatorNow(nowText)
	if err != nil {
		return withOperatorError("retention_preview", "invalid_config_value", err)
	}
	cutoff := now.Add(-closedIncidentRetention)
	candidates, err := repo.ListRetentionDeletionCandidates(ctx, cutoff, limit)
	if err != nil {
		return withOperatorError("retention_preview", "metadata", err)
	}

	return withOperatorError("retention_preview", "unknown", writeOperatorJSON(stdout, operatorRetentionPreviewOutput{
		Type:                    "retention_preview",
		MetadataBackend:         cfg.Backends.Metadata,
		ReadOnly:                true,
		ClosedIncidentRetention: closedIncidentRetention.String(),
		Cutoff:                  cutoff,
		Limit:                   limit,
		CandidateCount:          len(candidates),
		Candidates:              candidates,
	}))
}

func runOperatorModeAwareRetentionPreview(ctx context.Context, args []string, stdout io.Writer, cfg config.Config, repo operatorRepository) error {
	limit := operatorDefaultLimit
	nowText := ""
	var emergencyRetention time.Duration
	var interactionRecordRetention time.Duration
	var safetyCheckRetention time.Duration
	var evidenceNoteRetention time.Duration
	flags := flag.NewFlagSet("operator mode-retention-preview", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.DurationVar(&emergencyRetention, "emergency-retention", 0, "dry-run retention window for emergency incidents")
	flags.DurationVar(&interactionRecordRetention, "interaction-record-retention", 0, "dry-run retention window for interaction records")
	flags.DurationVar(&safetyCheckRetention, "safety-check-retention", 0, "dry-run retention window for safety checks")
	flags.DurationVar(&evidenceNoteRetention, "evidence-note-retention", 0, "dry-run retention window for evidence notes")
	flags.IntVar(&limit, "limit", limit, "maximum closed active incidents to inspect")
	flags.StringVar(&nowText, "now", nowText, "RFC3339 time override for deterministic previews")
	if err := flags.Parse(args); err != nil {
		return withOperatorError("mode_retention_preview", "invalid_config_value", err)
	}
	if flags.NArg() != 0 {
		return withOperatorError("mode_retention_preview", "invalid_config_value", fmt.Errorf("operator mode-retention-preview does not accept positional arguments"))
	}
	if limit <= 0 {
		return withOperatorError("mode_retention_preview", "invalid_config_value", fmt.Errorf("operator mode-retention-preview requires a positive --limit"))
	}
	now, err := operatorNow(nowText)
	if err != nil {
		return withOperatorError("mode_retention_preview", "invalid_config_value", err)
	}
	policyWindows := map[string]time.Duration{
		incidents.IncidentModeEmergency:         emergencyRetention,
		incidents.IncidentModeInteractionRecord: interactionRecordRetention,
		incidents.IncidentModeSafetyCheck:       safetyCheckRetention,
		incidents.IncidentModeEvidenceNote:      evidenceNoteRetention,
	}

	items, err := repo.ListModeAwareRetentionPreviewIncidents(ctx, limit)
	if err != nil {
		return withOperatorError("mode_retention_preview", "metadata", err)
	}
	candidates, ineligible := classifyModeAwareRetentionPreview(items, policyWindows, now)
	return withOperatorError("mode_retention_preview", "unknown", writeOperatorJSON(stdout, operatorModeAwareRetentionPreviewOutput{
		Type:             "mode_retention_preview",
		MetadataBackend:  cfg.Backends.Metadata,
		ReadOnly:         true,
		LiveDeletion:     false,
		PolicyWindows:    safePolicyWindowStrings(policyWindows),
		Limit:            limit,
		CandidateCount:   countModeAwareRetentionCandidates(candidates),
		IneligibleCount:  len(ineligible),
		CandidatesByMode: candidates,
		Ineligible:       ineligible,
	}))
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
		return withOperatorError("deletion_status", "invalid_config_value", err)
	}
	if flags.NArg() != 0 {
		return withOperatorError("deletion_status", "invalid_config_value", fmt.Errorf("operator deletion-status does not accept positional arguments"))
	}
	if limit <= 0 {
		return withOperatorError("deletion_status", "invalid_config_value", fmt.Errorf("operator deletion-status requires a positive --limit"))
	}
	if deletingRetryAfter <= 0 {
		return withOperatorError("deletion_status", "invalid_config_value", fmt.Errorf("operator deletion-status requires a positive --deleting-retry-after"))
	}

	now, err := operatorNow(nowText)
	if err != nil {
		return withOperatorError("deletion_status", "invalid_config_value", err)
	}
	staleDeletingBefore := now.Add(-deletingRetryAfter)
	status, err := repo.GetIncidentDeletionJobStatus(ctx, limit, staleDeletingBefore)
	if err != nil {
		return withOperatorError("deletion_status", "metadata", err)
	}

	return withOperatorError("deletion_status", "unknown", writeOperatorJSON(stdout, operatorDeletionStatusOutput{
		Type:                "deletion_status",
		MetadataBackend:     cfg.Backends.Metadata,
		ReadOnly:            true,
		DeletingRetryAfter:  deletingRetryAfter.String(),
		StaleDeletingBefore: staleDeletingBefore,
		Limit:               limit,
		RunnableJobCount:    len(status.RunnableJobs),
		Status:              status,
	}))
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

func classifyModeAwareRetentionPreview(items []incidents.ModeAwareRetentionPreviewIncident, policyWindows map[string]time.Duration, now time.Time) (map[string][]incidents.ModeAwareRetentionCandidate, []incidents.ModeAwareRetentionIneligible) {
	candidates := map[string][]incidents.ModeAwareRetentionCandidate{}
	ineligible := []incidents.ModeAwareRetentionIneligible{}
	for _, item := range items {
		reason := modeAwareRetentionIneligibleReason(item, policyWindows)
		if reason != "" {
			ineligible = append(ineligible, modeAwareRetentionIneligible(item, reason))
			continue
		}
		window := policyWindows[item.IncidentMode]
		cutoff := now.Add(-window)
		if item.UpdatedAt.After(cutoff) {
			ineligible = append(ineligible, modeAwareRetentionIneligible(item, "not_past_policy_cutoff"))
			continue
		}
		candidate := incidents.ModeAwareRetentionCandidate{
			IncidentID:       item.IncidentID,
			PolicyClass:      item.IncidentMode,
			IncidentMode:     item.IncidentMode,
			CaptureProfile:   item.CaptureProfile,
			EscalationPolicy: item.EscalationPolicy,
			SharingState:     item.SharingState,
			UpdatedAt:        item.UpdatedAt,
			Cutoff:           cutoff,
		}
		candidates[item.IncidentMode] = append(candidates[item.IncidentMode], candidate)
	}
	return candidates, ineligible
}

func modeAwareRetentionIneligibleReason(item incidents.ModeAwareRetentionPreviewIncident, policyWindows map[string]time.Duration) string {
	if item.IncidentMode == "" || item.CaptureProfile == "" || item.EscalationPolicy == "" || item.SharingState == "" {
		return "missing_policy_input"
	}
	if !incidents.ValidIncidentMode(item.IncidentMode) ||
		!incidents.ValidCaptureProfile(item.CaptureProfile) ||
		!incidents.ValidEscalationPolicy(item.EscalationPolicy) ||
		!incidents.ValidSharingState(item.SharingState) {
		return "invalid_policy_input"
	}
	if policyWindows[item.IncidentMode] <= 0 {
		return "policy_class_disabled"
	}
	return ""
}

func modeAwareRetentionIneligible(item incidents.ModeAwareRetentionPreviewIncident, reason string) incidents.ModeAwareRetentionIneligible {
	return incidents.ModeAwareRetentionIneligible{
		IncidentID:       item.IncidentID,
		Reason:           reason,
		IncidentMode:     item.IncidentMode,
		CaptureProfile:   item.CaptureProfile,
		EscalationPolicy: item.EscalationPolicy,
		SharingState:     item.SharingState,
		UpdatedAt:        item.UpdatedAt,
	}
}

func countModeAwareRetentionCandidates(candidates map[string][]incidents.ModeAwareRetentionCandidate) int {
	count := 0
	for _, group := range candidates {
		count += len(group)
	}
	return count
}

func safePolicyWindowStrings(policyWindows map[string]time.Duration) map[string]string {
	values := make(map[string]string, len(policyWindows))
	for _, mode := range []string{
		incidents.IncidentModeEmergency,
		incidents.IncidentModeInteractionRecord,
		incidents.IncidentModeSafetyCheck,
		incidents.IncidentModeEvidenceNote,
	} {
		values[mode] = policyWindows[mode].String()
	}
	return values
}

func writeOperatorJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
