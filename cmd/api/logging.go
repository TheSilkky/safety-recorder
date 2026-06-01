package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"

	"github.com/open-proofline/server/internal/config"
	"github.com/open-proofline/server/internal/coordination"
	"github.com/open-proofline/server/internal/storage"
)

func logStartupError(logger *slog.Logger, err error) {
	attrs := []any{"error_category", safeStartupErrorCategory(err)}
	if detail := safeStartupErrorDetail(err); detail != "" {
		attrs = append(attrs, "error_detail", detail)
	}
	logger.Error("server stopped", attrs...)
}

func safeStartupErrorCategory(err error) string {
	if err == nil {
		return "unknown"
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, storage.ErrUnsafePath):
		return "unsafe_path"
	case errors.Is(err, storage.ErrTooLarge):
		return "too_large"
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
		return "config"
	}
	var configParseErr config.ParseError
	if errors.As(err, &configParseErr) {
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
		return "system"
	}
	return "startup"
}

func safeStartupErrorDetail(err error) string {
	if errors.Is(err, errAuthBootstrapRequired) {
		return "create an admin account with SAFE_AUTH_BOOTSTRAP_SECRET before serving authenticated /v1 routes"
	}
	var unsupportedBackendErr config.UnsupportedBackendError
	if errors.As(err, &unsupportedBackendErr) {
		return unsupportedBackendErr.Error()
	}
	return ""
}
