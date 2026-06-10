package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/open-proofline/server/internal/config"
	"github.com/open-proofline/server/internal/coordination"
	"github.com/open-proofline/server/internal/db"
	"github.com/open-proofline/server/internal/email"
	"github.com/open-proofline/server/internal/httpapi"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/postgresdb"
	"github.com/open-proofline/server/internal/retention"
	"github.com/open-proofline/server/internal/storage"
)

func main() {
	logOutput := os.Stdout
	if commandIsOperator(os.Args[1:]) {
		logOutput = os.Stderr
	}
	logger := slog.New(slog.NewJSONHandler(logOutput, nil))
	if err := runCommand(os.Args[1:], os.Stdout, logger); err != nil {
		logCommandError(logger, err)
		os.Exit(1)
	}
}

func commandIsOperator(args []string) bool {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--config":
			i++
			continue
		case strings.HasPrefix(args[i], "--config="):
			continue
		default:
			return args[i] == "operator"
		}
	}
	return false
}

func run(logger *slog.Logger, configFilePath string) error {
	cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigFilePath: configFilePath})
	if err != nil {
		return withStartupStage(startupStageConfigLoad, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	coord, err := newCoordinator(cfg)
	if err != nil {
		return withStartupStage(startupStageCoordinationInit, err)
	}
	defer func() { _ = coord.Close() }()
	if err := coord.Check(ctx); err != nil {
		return withStartupStage(startupStageCoordinationCheck, err)
	}

	repo, closeRepo, err := newMetadataRepository(ctx, cfg)
	if err != nil {
		return withStartupStage(startupStageMetadataOpen, err)
	}
	defer closeRepo()
	if err := checkAuthBootstrap(ctx, repo, cfg); err != nil {
		return withStartupStage(startupStageAuthBootstrapCheck, err)
	}

	blobStore, err := newBlobStore(cfg)
	if err != nil {
		return withStartupStage(startupStageBlobStoreOpen, err)
	}
	if err := runTempUploadCleanup(ctx, logger, blobStore, cfg); err != nil {
		return withStartupStage(startupStageTempUploadCleanup, err)
	}

	apiOptions := httpapi.Options{
		MaxUploadBytes:             cfg.MaxUploadBytes,
		AccountBlobQuotaBytes:      cfg.AccountDefaultBlobQuotaBytes,
		DefaultIncidentTokenTTL:    &cfg.DefaultIncidentTokenTTL,
		SessionTTL:                 cfg.SessionTTL,
		BootstrapSecret:            cfg.AuthBootstrapSecret,
		WebAuth:                    webAuthConfig(cfg.WebAuth),
		WebAuthn:                   webAuthnConfig(cfg.WebAuthn),
		AccountRegistration:        accountRegistrationConfig(cfg.AccountRegistration),
		SecondFactorEmailTTL:       cfg.SecondFactorEmailChallengeTTL,
		RelayCapability:            relayCapabilityConfig(cfg.RelayCapability),
		EmailSender:                newEmailSender(cfg.Email),
		MainRateLimit:              mainRateLimitConfig(cfg.MainAPIRateLimit),
		MainRateLimiter:            newMainRateLimiter(cfg, coord),
		PublicRateLimit:            publicRateLimitConfig(cfg.PublicViewerRateLimit),
		PublicRateLimiter:          newPublicRateLimiter(cfg, coord),
		UploadCoordinator:          coord,
		UploadCoordinationLeaseTTL: cfg.UploadCoordinationLeaseTTL,
		Logger:                     logger,
	}
	mainHandler := httpapi.NewMain(repo, blobStore, apiOptions)
	adminHandler := httpapi.NewAdmin(repo, blobStore, apiOptions)
	deletionWorker := retention.NewWorker(repo, blobStore, retention.Options{
		Interval:                cfg.DeletionWorkerInterval,
		ClosedIncidentRetention: cfg.ClosedIncidentRetention,
		TokenMetadataRetention:  cfg.TokenMetadataRetention,
		TombstoneRetention:      cfg.TombstoneRetention,
		Logger:                  logger,
	})
	deletionWorker.Start(ctx)
	servers := newHTTPServers(cfg, mainHandler, adminHandler)

	errCh := make(chan error, len(servers))
	for _, server := range servers {
		startServer(errCh, logger, server)
	}

	select {
	case <-ctx.Done():
		return withStartupStage(startupStageShutdown, shutdownServers(servers))
	case err := <-errCh:
		_ = shutdownServers(servers)
		return err
	}
}

func mainRateLimitConfig(cfg config.MainAPIRateLimitConfig) httpapi.MainRateLimitConfig {
	return httpapi.MainRateLimitConfig{
		Enabled:            cfg.Enabled,
		Window:             cfg.Window,
		AuthLimit:          cfg.AuthLimit,
		AuthRegisterLimit:  cfg.AuthRegisterLimit,
		AuthEmailVerify:    cfg.AuthEmailVerify,
		BootstrapLimit:     cfg.BootstrapLimit,
		AccountLimit:       cfg.AccountLimit,
		IncidentReadLimit:  cfg.IncidentReadLimit,
		IncidentWriteLimit: cfg.IncidentWriteLimit,
		UploadLimit:        cfg.UploadLimit,
		ReconcileLimit:     cfg.ReconcileLimit,
		StreamLimit:        cfg.StreamLimit,
		TokenLimit:         cfg.TokenLimit,
		DownloadLimit:      cfg.DownloadLimit,
		AdminLimit:         cfg.AdminLimit,
	}
}

func publicRateLimitConfig(cfg config.PublicViewerRateLimitConfig) httpapi.PublicRateLimitConfig {
	return httpapi.PublicRateLimitConfig{
		Enabled:       cfg.Enabled,
		Window:        cfg.Window,
		PageLimit:     cfg.PageLimit,
		DataLimit:     cfg.DataLimit,
		DownloadLimit: cfg.DownloadLimit,
		StaticLimit:   cfg.StaticLimit,
	}
}

func webAuthConfig(cfg config.WebAuthConfig) httpapi.WebAuthConfig {
	sameSite := http.SameSiteLaxMode
	if cfg.SessionCookieSameSite == "strict" {
		sameSite = http.SameSiteStrictMode
	}
	return httpapi.WebAuthConfig{
		Enabled:               cfg.Enabled,
		AllowedOrigins:        cfg.AllowedOrigins,
		SessionCookieName:     cfg.SessionCookieName,
		SessionCookieSecure:   cfg.SessionCookieSecure,
		SessionCookieSameSite: sameSite,
		CSRFHeaderName:        cfg.CSRFHeaderName,
	}
}

func webAuthnConfig(cfg config.WebAuthnConfig) httpapi.WebAuthnConfig {
	return httpapi.WebAuthnConfig{
		Enabled:          cfg.Enabled,
		RPID:             cfg.RPID,
		RPDisplayName:    cfg.RPDisplayName,
		AllowedOrigins:   cfg.AllowedOrigins,
		UserVerification: cfg.UserVerification,
		ChallengeTTL:     cfg.ChallengeTTL,
	}
}

func accountRegistrationConfig(cfg config.AccountRegistrationConfig) httpapi.AccountRegistrationConfig {
	return httpapi.AccountRegistrationConfig{
		Mode:                 cfg.Mode,
		EmailVerificationTTL: cfg.EmailVerificationTTL,
		PublicWebOrigin:      cfg.PublicWebOrigin,
	}
}

func relayCapabilityConfig(cfg config.RelayCapabilityConfig) httpapi.RelayCapabilityConfig {
	return httpapi.RelayCapabilityConfig{
		Secret:    cfg.Secret,
		TTL:       cfg.TTL,
		MaxChunks: cfg.MaxChunks,
	}
}

func newEmailSender(cfg config.EmailConfig) email.Sender {
	if cfg.Backend != config.EmailBackendSMTP {
		return email.NoneSender{}
	}
	return email.NewSMTPSender(email.SMTPOptions{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
		StartTLS: email.SMTPStartTLSMode(cfg.SMTP.StartTLS),
		Timeout:  cfg.SMTP.Timeout,
	})
}

func newMainRateLimiter(cfg config.Config, coord coordination.Coordinator) httpapi.RateLimiter {
	if !cfg.MainAPIRateLimit.Enabled {
		return nil
	}
	switch cfg.Backends.Coordination {
	case config.CoordinationBackendValkey, config.CoordinationBackendRedis:
		if limiter, ok := coord.(httpapi.RateLimiter); ok {
			return limiter
		}
	}
	return httpapi.NewMemoryRateLimiter()
}

func newPublicRateLimiter(cfg config.Config, coord coordination.Coordinator) httpapi.PublicRateLimiter {
	if !cfg.PublicViewerRateLimit.Enabled {
		return nil
	}
	switch cfg.Backends.Coordination {
	case config.CoordinationBackendValkey, config.CoordinationBackendRedis:
		if limiter, ok := coord.(httpapi.PublicRateLimiter); ok {
			return limiter
		}
	}
	return httpapi.NewMemoryRateLimiter()
}

func newCoordinator(cfg config.Config) (coordination.Coordinator, error) {
	switch cfg.Backends.Coordination {
	case config.CoordinationBackendNone:
		return coordination.NewNone(), nil
	case config.CoordinationBackendValkey, config.CoordinationBackendRedis:
		return coordination.NewValkeyClient(coordination.ValkeyOptions{
			Addr:         cfg.Valkey.Addr,
			Username:     cfg.Valkey.Username,
			Password:     cfg.Valkey.Password,
			DB:           cfg.Valkey.DB,
			UseTLS:       cfg.Valkey.UseTLS,
			DialTimeout:  cfg.Valkey.DialTimeout,
			ReadTimeout:  cfg.Valkey.ReadTimeout,
			WriteTimeout: cfg.Valkey.WriteTimeout,
		})
	default:
		return nil, config.UnsupportedBackendError{
			EnvName: "SAFE_COORDINATION_BACKEND",
			Supported: []string{
				config.CoordinationBackendNone,
				config.CoordinationBackendValkey,
				config.CoordinationBackendRedis,
			},
		}
	}
}

func newMetadataRepository(ctx context.Context, cfg config.Config) (httpapi.MetadataRepository, func(), error) {
	switch cfg.Backends.Metadata {
	case config.MetadataBackendSQLite:
		conn, err := db.Open(ctx, cfg.DBPath)
		if err != nil {
			return nil, nil, err
		}
		return incidents.NewRepository(conn), func() { _ = conn.Close() }, nil
	case config.MetadataBackendPostgres:
		conn, err := postgresdb.Open(ctx, cfg.Postgres)
		if err != nil {
			return nil, nil, err
		}
		return postgresdb.NewRepository(conn), func() { _ = conn.Close() }, nil
	default:
		return nil, nil, config.UnsupportedBackendError{
			EnvName:   "SAFE_METADATA_BACKEND",
			Supported: []string{config.MetadataBackendSQLite, config.MetadataBackendPostgres},
		}
	}
}

func newBlobStore(cfg config.Config) (storage.BlobStore, error) {
	switch cfg.Backends.Blob {
	case config.BlobBackendLocal:
		return storage.NewWithOptions(cfg.DataDir, storage.Options{
			TempStagingQuotaBytes: cfg.TempUploadStagingQuotaBytes,
		})
	case config.BlobBackendS3:
		return storage.NewS3(storage.S3Options{
			Endpoint:              cfg.S3Blob.Endpoint,
			Region:                cfg.S3Blob.Region,
			Bucket:                cfg.S3Blob.Bucket,
			Prefix:                cfg.S3Blob.Prefix,
			AccessKeyID:           cfg.S3Blob.AccessKeyID,
			SecretAccessKey:       cfg.S3Blob.SecretAccessKey,
			SessionToken:          cfg.S3Blob.SessionToken,
			ForcePathStyle:        cfg.S3Blob.ForcePathStyle,
			TempDir:               filepath.Join(cfg.DataDir, "tmp"),
			TempStagingQuotaBytes: cfg.TempUploadStagingQuotaBytes,
		})
	default:
		return nil, config.UnsupportedBackendError{
			EnvName:   "SAFE_BLOB_BACKEND",
			Supported: []string{config.BlobBackendLocal, config.BlobBackendS3},
		}
	}
}

func runTempUploadCleanup(ctx context.Context, logger *slog.Logger, store storage.BlobStore, cfg config.Config) error {
	if cfg.TempUploadCleanupAge <= 0 {
		return nil
	}
	cleaner, ok := store.(storage.TempCleaner)
	if !ok {
		return nil
	}
	summary, err := cleaner.CleanupTemp(ctx, storage.TempCleanupOptions{
		MinAge: cfg.TempUploadCleanupAge,
		DryRun: cfg.TempUploadCleanupDryRun,
	})
	if err != nil {
		return fmt.Errorf("temp upload cleanup: %w", err)
	}
	logger.Info("temp upload cleanup completed",
		"component", "startup",
		"startup_stage", startupStageTempUploadCleanup,
		"status", "completed",
		"dry_run", cfg.TempUploadCleanupDryRun,
		"scanned", summary.Scanned,
		"eligible", summary.Eligible,
		"removed", summary.Removed,
		"skipped_active", summary.SkippedActive,
		"skipped_other", summary.SkippedOther,
		"errors", summary.Errors,
	)
	return nil
}
