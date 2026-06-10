package httpapi

import (
	"context"
	"errors"
	"net"
	"os"
	"reflect"

	"github.com/open-proofline/server/internal/email"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/storage"
)

var errRateLimitUnavailable = errors.New("rate limiter unavailable")

func (a *API) logInternalError(operation string, err error, attrs ...any) {
	logAttrs := []any{"component", "httpapi", "operation", operation, "error_category", safeErrorCategory(err)}
	logAttrs = append(logAttrs, attrs...)
	a.logger.Error("internal error", logAttrs...)
}

func (a *API) logRecoveredPanic(recovered any) {
	a.logger.Error("panic recovered",
		"component", "httpapi",
		"operation", "panic_recovery",
		"error_category", "unknown",
		"panic_type", safePanicType(recovered),
	)
}

func safePanicType(recovered any) string {
	if recovered == nil {
		return "unknown"
	}
	return reflect.TypeOf(recovered).String()
}

func safeErrorCategory(err error) string {
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
	case errors.Is(err, incidents.ErrDuplicate):
		return "duplicate"
	case errors.Is(err, incidents.ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, incidents.ErrIncidentClosed):
		return "incident_closed"
	case errors.Is(err, incidents.ErrInvalidState):
		return "invalid_state"
	case errors.Is(err, incidents.ErrNotFound):
		return "not_found"
	case errors.Is(err, email.ErrDisabled):
		return "email"
	case errors.Is(err, errRateLimitUnavailable):
		return "rate_limit_unavailable"
	case errors.Is(err, os.ErrNotExist):
		return "not_found"
	case errors.Is(err, os.ErrExist):
		return "already_exists"
	case errors.Is(err, os.ErrPermission):
		return "permission"
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
	return "unknown"
}
