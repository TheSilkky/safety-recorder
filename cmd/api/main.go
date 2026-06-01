package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/open-proofline/server/internal/config"
	"github.com/open-proofline/server/internal/coordination"
	"github.com/open-proofline/server/internal/db"
	"github.com/open-proofline/server/internal/httpapi"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/postgresdb"
	"github.com/open-proofline/server/internal/retention"
	"github.com/open-proofline/server/internal/storage"
)

func main() {
	logOutput := os.Stdout
	if len(os.Args) > 1 && os.Args[1] == "operator" {
		logOutput = os.Stderr
	}
	logger := slog.New(slog.NewJSONHandler(logOutput, nil))
	if err := runCommand(os.Args[1:], os.Stdout, logger); err != nil {
		logStartupError(logger, err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	coord, err := newCoordinator(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = coord.Close() }()
	if err := coord.Check(ctx); err != nil {
		return err
	}

	repo, closeRepo, err := newMetadataRepository(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeRepo()
	if err := checkAuthBootstrap(ctx, repo, cfg); err != nil {
		return err
	}

	blobStore, err := newBlobStore(cfg)
	if err != nil {
		return err
	}
	if err := runTempUploadCleanup(ctx, logger, blobStore, cfg); err != nil {
		return err
	}

	apiOptions := httpapi.Options{
		MaxUploadBytes:             cfg.MaxUploadBytes,
		DefaultIncidentTokenTTL:    &cfg.DefaultIncidentTokenTTL,
		SessionTTL:                 cfg.SessionTTL,
		BootstrapSecret:            cfg.AuthBootstrapSecret,
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
		return shutdownServers(servers)
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
		return nil, fmt.Errorf("unsupported coordination backend %q", cfg.Backends.Coordination)
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
		return nil, nil, fmt.Errorf("unsupported metadata backend %q", cfg.Backends.Metadata)
	}
}

func newBlobStore(cfg config.Config) (storage.BlobStore, error) {
	switch cfg.Backends.Blob {
	case config.BlobBackendLocal:
		return storage.New(cfg.DataDir)
	case config.BlobBackendS3:
		return storage.NewS3(storage.S3Options{
			Endpoint:        cfg.S3Blob.Endpoint,
			Region:          cfg.S3Blob.Region,
			Bucket:          cfg.S3Blob.Bucket,
			Prefix:          cfg.S3Blob.Prefix,
			AccessKeyID:     cfg.S3Blob.AccessKeyID,
			SecretAccessKey: cfg.S3Blob.SecretAccessKey,
			SessionToken:    cfg.S3Blob.SessionToken,
			ForcePathStyle:  cfg.S3Blob.ForcePathStyle,
			TempDir:         filepath.Join(cfg.DataDir, "tmp"),
		})
	default:
		return nil, fmt.Errorf("unsupported blob backend %q", cfg.Backends.Blob)
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
