package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultBindAddr = "127.0.0.1:8090"

	startupStageConfigLoad = "config_load"
	startupStageHTTPListen = "http_listen"
	startupStageShutdown   = "shutdown"
)

type streamIngressConfig struct {
	BindAddr string
	RelayID  string
	Region   string
	Ready    bool
}

type configParseError struct {
	Name    string
	Message string
}

func (e configParseError) Error() string {
	return fmt.Sprintf("parse %s: %s", e.Name, e.Message)
}

type startupError struct {
	Stage string
	Err   error
}

func (e startupError) Error() string {
	return e.Err.Error()
}

func (e startupError) Unwrap() error {
	return e.Err
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logStartupError(logger, err)
		os.Exit(1)
	}
}

func run(args []string, logger *slog.Logger) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return withStartupStage(startupStageConfigLoad, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:              cfg.BindAddr,
		Handler:           newHandler(cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting http server",
			"component", "startup",
			"startup_stage", startupStageHTTPListen,
			"listener", "stream_ingress",
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- withStartupStage(startupStageHTTPListen, fmt.Errorf("stream ingress server listen failed: %w", err))
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return withStartupStage(startupStageShutdown, server.Shutdown(shutdownCtx))
	case err := <-errCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return err
	}
}

func loadConfig(args []string) (streamIngressConfig, error) {
	readyDefault, err := boolEnv("SAFE_STREAM_INGRESS_READY", false)
	if err != nil {
		return streamIngressConfig{}, err
	}

	cfg := streamIngressConfig{
		BindAddr: envOrDefault("SAFE_STREAM_INGRESS_BIND_ADDR", defaultBindAddr),
		RelayID:  strings.TrimSpace(os.Getenv("SAFE_STREAM_INGRESS_RELAY_ID")),
		Region:   strings.TrimSpace(os.Getenv("SAFE_STREAM_INGRESS_REGION")),
		Ready:    readyDefault,
	}

	fs := flag.NewFlagSet("stream-ingress", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.BindAddr, "bind", cfg.BindAddr, "private bind address")
	fs.StringVar(&cfg.RelayID, "relay-id", cfg.RelayID, "relay identity label")
	fs.StringVar(&cfg.Region, "region", cfg.Region, "relay region label")
	fs.BoolVar(&cfg.Ready, "ready", cfg.Ready, "return ready from /health/ready")
	if err := fs.Parse(args); err != nil {
		return streamIngressConfig{}, err
	}

	cfg.BindAddr = strings.TrimSpace(cfg.BindAddr)
	cfg.RelayID = strings.TrimSpace(cfg.RelayID)
	cfg.Region = strings.TrimSpace(cfg.Region)
	if cfg.BindAddr == "" {
		return streamIngressConfig{}, configParseError{Name: "SAFE_STREAM_INGRESS_BIND_ADDR", Message: "bind address is required"}
	}
	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func boolEnv(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, configParseError{Name: name, Message: "boolean value is required"}
	}
	return parsed, nil
}

func newHandler(cfg streamIngressConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"service": "stream-ingress",
			"status":  "ok",
		})
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		status := http.StatusServiceUnavailable
		state := "not_ready"
		if cfg.Ready {
			status = http.StatusOK
			state = "ready"
		}
		writeJSON(w, status, map[string]any{
			"relay_identity_configured": cfg.RelayID != "",
			"region_configured":         cfg.Region != "",
			"service":                   "stream-ingress",
			"status":                    state,
			"uploads":                   "unimplemented",
		})
	})
	return mux
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", http.MethodGet)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
		"error": "method_not_allowed",
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func withStartupStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	return startupError{Stage: stage, Err: err}
}

func logStartupError(logger *slog.Logger, err error) {
	var staged startupError
	if !errors.As(err, &staged) {
		staged = startupError{Stage: "unknown", Err: err}
	}

	args := []any{
		"component", "startup",
		"startup_stage", staged.Stage,
	}
	var parseErr configParseError
	if errors.As(staged.Err, &parseErr) {
		args = append(args,
			"error_category", "invalid_config",
			"config_key", parseErr.Name,
			"safe_error_detail", parseErr.Message,
		)
	} else {
		args = append(args, "error_category", safeErrorCategory(staged.Err))
	}
	logger.Error("stream ingress startup failed", args...)
}

func safeErrorCategory(err error) string {
	switch {
	case errors.Is(err, http.ErrServerClosed):
		return "server_closed"
	case errors.Is(err, flag.ErrHelp):
		return "usage"
	default:
		return "startup_failed"
	}
}
