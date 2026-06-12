package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/config"
	"github.com/open-proofline/server/internal/coordination"
	"github.com/open-proofline/server/internal/db"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/storage"
)

func TestNewHTTPServersCreatesOneServerPerBindAddress(t *testing.T) {
	mainHandler := http.NewServeMux()
	adminHandler := http.NewServeMux()
	cfg := config.Config{
		MainBindAddrs:  []string{"127.0.0.1:8080", "10.66.0.1:8080"},
		AdminBindAddrs: []string{"127.0.0.1:8081", "192.168.1.20:8081"},
	}

	servers := newHTTPServers(cfg, mainHandler, adminHandler)

	if len(servers) != 4 {
		t.Fatalf("got %d servers, want 4", len(servers))
	}
	assertServer(t, servers[0], "main api and viewer", "main_api_viewer", "127.0.0.1:8080", mainHandler)
	assertServer(t, servers[1], "main api and viewer", "main_api_viewer", "10.66.0.1:8080", mainHandler)
	assertServer(t, servers[2], "private admin", "private_admin", "127.0.0.1:8081", adminHandler)
	assertServer(t, servers[3], "private admin", "private_admin", "192.168.1.20:8081", adminHandler)
}

func TestNewHTTPServersAppliesMainAndAdminTimeouts(t *testing.T) {
	mainHandler := http.NewServeMux()
	adminHandler := http.NewServeMux()
	cfg := config.Config{
		MainBindAddrs:  []string{"127.0.0.1:8080"},
		AdminBindAddrs: []string{"127.0.0.1:8081"},
		MainTimeouts: config.HTTPTimeouts{
			ReadHeaderTimeout: 11 * time.Second,
			ReadTimeout:       0,
			WriteTimeout:      0,
			IdleTimeout:       121 * time.Second,
		},
		AdminTimeouts: config.HTTPTimeouts{
			ReadHeaderTimeout: 12 * time.Second,
			ReadTimeout:       31 * time.Second,
			WriteTimeout:      301 * time.Second,
			IdleTimeout:       122 * time.Second,
		},
	}

	servers := newHTTPServers(cfg, mainHandler, adminHandler)

	assertServerTimeouts(t, servers[0].server, cfg.MainTimeouts)
	assertServerTimeouts(t, servers[1].server, cfg.AdminTimeouts)
}

func TestStartupErrorLogDoesNotExposeFilesystemPath(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	err := withStartupStage(startupStageBlobStoreOpen, &os.PathError{
		Op:   "mkdir",
		Path: "<private filesystem path>",
		Err:  os.ErrPermission,
	})

	logStartupError(logger, err)

	if bytes.Contains(logs.Bytes(), []byte("<private filesystem path>")) {
		t.Fatalf("startup log exposed filesystem path: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("component=startup")) {
		t.Fatalf("startup log omitted component: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("startup_stage=blob_store_open")) {
		t.Fatalf("startup log omitted safe stage: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("error_category=permission")) {
		t.Fatalf("startup log omitted safe error category: %s", logs.String())
	}
}

func TestStartupErrorLogRedactsSecretConfigNameAndUsesSafeDetail(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	err := withStartupStage(startupStageConfigLoad, config.ParseError{
		Name:    "SAFE_AUTH_BOOTSTRAP_SECRET_FILE",
		Message: "secret file cannot be read",
	})

	logStartupError(logger, err)

	if !bytes.Contains(logs.Bytes(), []byte("startup_stage=config_load")) {
		t.Fatalf("startup log omitted config stage: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("error_category=secret_file_config")) {
		t.Fatalf("startup log omitted secret config category: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("config_key_class=secret_file_config")) {
		t.Fatalf("startup log omitted secret config key class: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("safe_error_detail=\"secret file cannot be read\"")) {
		t.Fatalf("startup log omitted safe secret-file detail: %s", logs.String())
	}
	if bytes.Contains(logs.Bytes(), []byte("SAFE_AUTH_BOOTSTRAP_SECRET_FILE")) {
		t.Fatalf("startup log exposed secret config key name: %s", logs.String())
	}
}

func TestExtractConfigFlag(t *testing.T) {
	path, rest, err := extractConfigFlag([]string{"--config", "/tmp/proofline.toml", "operator", "deletion-status"})
	if err != nil {
		t.Fatalf("extractConfigFlag: %v", err)
	}
	if path != "/tmp/proofline.toml" || !reflect.DeepEqual(rest, []string{"operator", "deletion-status"}) {
		t.Fatalf("path=%q rest=%v", path, rest)
	}

	path, rest, err = extractConfigFlag([]string{"operator", "--config=/tmp/proofline.toml", "retention-preview"})
	if err != nil {
		t.Fatalf("extractConfigFlag with equals: %v", err)
	}
	if path != "/tmp/proofline.toml" || !reflect.DeepEqual(rest, []string{"operator", "retention-preview"}) {
		t.Fatalf("path=%q rest=%v", path, rest)
	}
}

func TestCheckAuthBootstrapFailsWithoutAdminOrBootstrapSecret(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "proofline.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	repo := incidents.NewRepository(conn)

	err = checkAuthBootstrap(ctx, repo, config.Config{})
	if !errors.Is(err, errAuthBootstrapRequired) {
		t.Fatalf("checkAuthBootstrap error = %v, want auth bootstrap required", err)
	}
}

func TestCheckAuthBootstrapAllowsBootstrapSecretOrExistingAdmin(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "proofline.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	repo := incidents.NewRepository(conn)

	if err := checkAuthBootstrap(ctx, repo, config.Config{AuthBootstrapSecret: "bootstrap-secret"}); err != nil {
		t.Fatalf("expected bootstrap secret to allow startup, got %v", err)
	}
	if _, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "admin",
		PasswordHash: "stored-hash",
		Role:         auth.RoleAdmin,
	}); err != nil {
		t.Fatalf("create admin account: %v", err)
	}
	if err := checkAuthBootstrap(ctx, repo, config.Config{}); err != nil {
		t.Fatalf("expected existing admin to allow startup, got %v", err)
	}
}

func TestRunTempUploadCleanupLogsSafeCounts(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	store, err := storage.New(dataDir)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	upload, err := store.SaveTemp(ctx, strings.NewReader("staged upload"), int64(len("staged upload")))
	if err != nil {
		t.Fatalf("save temp upload: %v", err)
	}
	if err := os.Chtimes(upload.Path, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatalf("age temp upload: %v", err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	err = runTempUploadCleanup(ctx, logger, store, config.Config{
		TempUploadCleanupAge: time.Hour,
	})
	if err != nil {
		t.Fatalf("run temp upload cleanup: %v", err)
	}
	if _, err := os.Stat(upload.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp upload exists after cleanup or returned unexpected error: %v", err)
	}
	for _, want := range [][]byte{
		[]byte("msg=\"temp upload cleanup completed\""),
		[]byte("component=startup"),
		[]byte("startup_stage=temp_upload_cleanup"),
		[]byte("status=completed"),
		[]byte("eligible=1"),
		[]byte("removed=1"),
	} {
		if !bytes.Contains(logs.Bytes(), want) {
			t.Fatalf("cleanup log omitted %q: %s", want, logs.String())
		}
	}
	if bytes.Contains(logs.Bytes(), []byte(dataDir)) || bytes.Contains(logs.Bytes(), []byte(upload.Path)) {
		t.Fatalf("cleanup log exposed a filesystem path: %s", logs.String())
	}
}

func TestStartupErrorLogIncludesSafeBackendConfigDetail(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	err := withStartupStage(startupStageConfigLoad, config.UnsupportedBackendError{
		EnvName:   "SAFE_METADATA_BACKEND",
		Supported: []string{config.MetadataBackendSQLite, config.MetadataBackendPostgres},
	})

	logStartupError(logger, err)

	if !bytes.Contains(logs.Bytes(), []byte("startup_stage=config_load")) {
		t.Fatalf("startup log omitted config stage: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("error_category=unsupported_backend")) {
		t.Fatalf("startup log omitted unsupported backend category: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("config_key=SAFE_METADATA_BACKEND")) {
		t.Fatalf("startup log omitted backend env name: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("safe_error_detail=\"unsupported backend; supported values: sqlite, postgresql\"")) {
		t.Fatalf("startup log omitted supported backend values: %s", logs.String())
	}
}

func TestStartupErrorLogDoesNotExposeCoordinationConnectionDetail(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	logStartupError(logger, withStartupStage(startupStageCoordinationCheck, coordination.ErrUnavailable))

	if !bytes.Contains(logs.Bytes(), []byte("startup_stage=coordination_check")) {
		t.Fatalf("startup log omitted coordination stage: %s", logs.String())
	}
	if !bytes.Contains(logs.Bytes(), []byte("error_category=coordination_unavailable")) {
		t.Fatalf("startup log omitted coordination category: %s", logs.String())
	}
	if bytes.Contains(logs.Bytes(), []byte("safe_error_detail")) {
		t.Fatalf("startup log exposed coordination detail: %s", logs.String())
	}
}

func TestStartupErrorLogIncludesAuthBootstrapStageAndSafeDetail(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	logStartupError(logger, withStartupStage(startupStageAuthBootstrapCheck, errAuthBootstrapRequired))

	for _, want := range [][]byte{
		[]byte("startup_stage=auth_bootstrap_check"),
		[]byte("error_category=auth_bootstrap_required"),
		[]byte("safe_error_detail=\"admin account required before serving authenticated routes\""),
	} {
		if !bytes.Contains(logs.Bytes(), want) {
			t.Fatalf("startup log omitted %q: %s", want, logs.String())
		}
	}
	if bytes.Contains(logs.Bytes(), []byte("SAFE_AUTH_BOOTSTRAP_SECRET")) {
		t.Fatalf("startup log exposed bootstrap secret config key: %s", logs.String())
	}
}

func TestStartupErrorLogIncludesSafeListenerClass(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	logStartupError(logger, withStartupListenerStage(startupStageHTTPListen, "private_admin", os.ErrPermission))

	for _, want := range [][]byte{
		[]byte("startup_stage=http_listen"),
		[]byte("listener=private_admin"),
		[]byte("error_category=permission"),
	} {
		if !bytes.Contains(logs.Bytes(), want) {
			t.Fatalf("startup log omitted %q: %s", want, logs.String())
		}
	}
}

func assertServer(t *testing.T, got namedServer, name, listener, addr string, handler http.Handler) {
	t.Helper()
	if got.name != name {
		t.Fatalf("server name = %q, want %q", got.name, name)
	}
	if got.listener != listener {
		t.Fatalf("server listener = %q, want %q", got.listener, listener)
	}
	if got.server.Addr != addr {
		t.Fatalf("server addr = %q, want %q", got.server.Addr, addr)
	}
	if got.server.Handler != handler {
		t.Fatal("server handler did not match expected shared handler")
	}
}

func assertServerTimeouts(t *testing.T, server *http.Server, want config.HTTPTimeouts) {
	t.Helper()
	if server.ReadHeaderTimeout != want.ReadHeaderTimeout ||
		server.ReadTimeout != want.ReadTimeout ||
		server.WriteTimeout != want.WriteTimeout ||
		server.IdleTimeout != want.IdleTimeout {
		t.Fatalf("server timeouts = read_header %s read %s write %s idle %s, want %+v",
			server.ReadHeaderTimeout,
			server.ReadTimeout,
			server.WriteTimeout,
			server.IdleTimeout,
			want,
		)
	}
}
