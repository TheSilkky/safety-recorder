package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/open-proofline/server/internal/config"
	"github.com/open-proofline/server/internal/coordination"
	"github.com/open-proofline/server/internal/storage"
)

const (
	startupStageArgsParse          = "args_parse"
	startupStageConfigLoad         = "config_load"
	startupStageCoordinationInit   = "coordination_init"
	startupStageCoordinationCheck  = "coordination_check"
	startupStageMetadataOpen       = "metadata_open"
	startupStageAuthBootstrapCheck = "auth_bootstrap_check"
	startupStageBlobStoreOpen      = "blob_store_open"
	startupStageTempUploadCleanup  = "temp_upload_cleanup"
	startupStageHTTPListen         = "http_listen"
	startupStageShutdown           = "shutdown"
)

type startupError struct {
	stage    string
	listener string
	err      error
}

func (e startupError) Error() string {
	return e.err.Error()
}

func (e startupError) Unwrap() error {
	return e.err
}

func withStartupStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	var existing startupError
	if errors.As(err, &existing) {
		return err
	}
	return startupError{stage: stage, err: err}
}

func withStartupListenerStage(stage, listener string, err error) error {
	if err == nil {
		return nil
	}
	return startupError{stage: stage, listener: listener, err: err}
}

func logCommandError(logger *slog.Logger, err error) {
	var operatorErr operatorError
	if errors.As(err, &operatorErr) {
		logOperatorError(logger, err)
		return
	}
	logStartupError(logger, err)
}

func logStartupError(logger *slog.Logger, err error) {
	attrs := []any{"component", "startup"}
	if stage := safeStartupStage(err); stage != "" {
		attrs = append(attrs, "startup_stage", stage)
	}
	if listener := safeStartupListener(err); listener != "" {
		attrs = append(attrs, "listener", listener)
	}
	attrs = append(attrs, "error_category", safeStartupErrorCategory(err))
	if configKey := safeStartupConfigKey(err); configKey != "" {
		attrs = append(attrs, "config_key", configKey)
	}
	if configKeyClass := safeStartupConfigKeyClass(err); configKeyClass != "" {
		attrs = append(attrs, "config_key_class", configKeyClass)
	}
	if detail := safeStartupErrorDetail(err); detail != "" {
		attrs = append(attrs, "safe_error_detail", detail)
	}
	logger.Error("server stopped", attrs...)
}

func safeStartupStage(err error) string {
	var startupErr startupError
	if errors.As(err, &startupErr) {
		return startupErr.stage
	}
	return ""
}

func logOperatorError(logger *slog.Logger, err error) {
	logger.Error("operator command failed",
		"component", "operator",
		"operation", safeOperatorOperation(err),
		"status", "failed",
		"error_category", safeOperatorErrorCategory(err),
	)
}

func safeStartupListener(err error) string {
	var startupErr startupError
	if errors.As(err, &startupErr) {
		return startupErr.listener
	}
	return ""
}

func safeStartupErrorCategory(err error) string {
	if err == nil {
		return "unknown"
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, storage.ErrUnsafePath):
		return "unsafe_path"
	case errors.Is(err, storage.ErrTooLarge):
		return "too_large"
	case errors.Is(err, storage.ErrTempStagingQuotaExceeded):
		return "temp_staging_quota_exceeded"
	case errors.Is(err, storage.ErrAlreadyExists):
		return "already_exists"
	case errors.Is(err, coordination.ErrUnavailable):
		return "coordination_unavailable"
	case errors.Is(err, errAuthBootstrapRequired):
		return "auth_bootstrap_required"
	case errors.Is(err, os.ErrNotExist):
		return "not_found"
	case errors.Is(err, os.ErrExist):
		return "already_exists"
	case errors.Is(err, os.ErrPermission):
		return "permission"
	}

	var unsupportedBackendErr config.UnsupportedBackendError
	if errors.As(err, &unsupportedBackendErr) {
		return "unsupported_backend"
	}
	var configParseErr config.ParseError
	if errors.As(err, &configParseErr) {
		if class := safeStartupConfigKeyClass(err); class != "" {
			return class
		}
		return "config"
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return "filesystem"
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return "filesystem"
	}
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		return "filesystem"
	}

	switch safeStartupStage(err) {
	case startupStageConfigLoad:
		return "config"
	case startupStageCoordinationInit, startupStageCoordinationCheck:
		return "coordination_unavailable"
	case startupStageMetadataOpen:
		return "metadata"
	case startupStageBlobStoreOpen, startupStageTempUploadCleanup:
		return "storage"
	case startupStageHTTPListen:
		return "network"
	case startupStageShutdown:
		return "shutdown"
	}
	return "unknown"
}

func safeStartupConfigKey(err error) string {
	var unsupportedBackendErr config.UnsupportedBackendError
	if errors.As(err, &unsupportedBackendErr) && isSafeStartupConfigKey(unsupportedBackendErr.EnvName) {
		return unsupportedBackendErr.EnvName
	}
	return ""
}

func isSafeStartupConfigKey(name string) bool {
	switch name {
	case "SAFE_METADATA_BACKEND",
		"SAFE_BLOB_BACKEND",
		"SAFE_COORDINATION_BACKEND",
		"SAFE_EMAIL_BACKEND",
		"SAFE_ACCOUNT_REGISTRATION_MODE":
		return true
	default:
		return false
	}
}

func safeStartupConfigKeyClass(err error) string {
	var configParseErr config.ParseError
	if !errors.As(err, &configParseErr) {
		return ""
	}
	name := configParseErr.Name
	message := configParseErr.Message
	if strings.Contains(name, "_FILE") || strings.Contains(message, "secret file") {
		return "secret_file_config"
	}
	if strings.Contains(name, "SECRET") ||
		strings.Contains(name, "PASSWORD") ||
		strings.Contains(name, "TOKEN") ||
		strings.Contains(name, "DSN") ||
		strings.Contains(name, "ACCESS_KEY") {
		return "secret_config"
	}
	return ""
}

func safeStartupErrorDetail(err error) string {
	if errors.Is(err, errAuthBootstrapRequired) {
		return "admin account required before serving authenticated routes"
	}
	var unsupportedBackendErr config.UnsupportedBackendError
	if errors.As(err, &unsupportedBackendErr) {
		return "unsupported backend; supported values: " + strings.Join(unsupportedBackendErr.Supported, ", ")
	}
	var configParseErr config.ParseError
	if errors.As(err, &configParseErr) && safeStartupConfigKeyClass(err) != "" {
		switch configParseErr.Message {
		case "direct secret and secret file are both configured",
			"secret file cannot be read",
			"secret file is empty":
			return configParseErr.Message
		}
	}
	return ""
}
