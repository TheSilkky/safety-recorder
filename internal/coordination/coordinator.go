package coordination

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrUnavailable reports that an explicitly configured coordination backend
// cannot currently be used. It intentionally carries no backend address,
// credential, key, token, or request detail.
var ErrUnavailable = errors.New("coordination backend unavailable")

// Coordinator is the narrow server boundary for optional short-lived
// coordination. Durable metadata and blob storage remain the source of truth.
type Coordinator interface {
	Check(context.Context) error
	AcquireUploadLease(context.Context, string, time.Duration) (UploadLease, error)
	ReleaseUploadLease(context.Context, UploadLease) error
	Close() error
}

// UploadLease records a short-lived, non-durable upload coordination lease.
// Keys and tokens are server-generated and must not contain raw request values.
type UploadLease struct {
	Key        string
	Token      string
	Acquired   bool
	RetryAfter time.Duration
}

// Noop is the default coordination backend.
type Noop struct{}

// NewNone returns the disabled/default coordination backend.
func NewNone() Noop {
	return Noop{}
}

// Check is always successful for the disabled/default backend.
func (Noop) Check(context.Context) error {
	return nil
}

// AcquireUploadLease is always successful for the disabled/default backend.
func (Noop) AcquireUploadLease(context.Context, string, time.Duration) (UploadLease, error) {
	return UploadLease{Acquired: true}, nil
}

// ReleaseUploadLease is a no-op for the disabled/default backend.
func (Noop) ReleaseUploadLease(context.Context, UploadLease) error {
	return nil
}

// Close is a no-op for the disabled/default backend.
func (Noop) Close() error {
	return nil
}

// Client is the small client surface needed for configured coordination.
type Client interface {
	Ping(context.Context) error
	IncrementWithExpiry(context.Context, string, time.Duration) (int64, error)
	SetNXWithExpiry(context.Context, string, string, time.Duration) (bool, error)
	DeleteIfValue(context.Context, string, string) (bool, error)
	Close() error
}

// Valkey uses a Valkey/Redis-compatible service for optional coordination.
type Valkey struct {
	client Client
}

// NewValkey builds a Valkey coordinator around a client. Tests can provide a
// fake client without requiring a running Valkey service.
func NewValkey(client Client) (*Valkey, error) {
	if client == nil {
		return nil, fmt.Errorf("missing valkey coordination client")
	}
	return &Valkey{client: client}, nil
}

// Check verifies that the configured service is reachable. The returned error
// is deliberately generic so startup logs do not expose private endpoints.
func (v *Valkey) Check(ctx context.Context) error {
	if err := v.client.Ping(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return ErrUnavailable
	}
	return nil
}

// Allow records a fixed-window rate-limit hit against a server-controlled key.
func (v *Valkey) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 || window <= 0 {
		return true, nil
	}
	count, err := v.client.IncrementWithExpiry(ctx, key, window)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, err
		}
		return false, ErrUnavailable
	}
	return count <= int64(limit), nil
}

// AcquireUploadLease tries to reserve a short-lived upload lease. A busy lease
// is a retryable in-progress hint, not durable evidence state.
func (v *Valkey) AcquireUploadLease(ctx context.Context, key string, ttl time.Duration) (UploadLease, error) {
	if key == "" || ttl <= 0 {
		return UploadLease{Acquired: true}, nil
	}
	token, err := newLeaseToken()
	if err != nil {
		return UploadLease{}, ErrUnavailable
	}
	acquired, err := v.client.SetNXWithExpiry(ctx, key, token, ttl)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return UploadLease{}, err
		}
		return UploadLease{}, ErrUnavailable
	}
	if !acquired {
		return UploadLease{Key: key, Acquired: false, RetryAfter: ttl}, nil
	}
	return UploadLease{Key: key, Token: token, Acquired: true, RetryAfter: ttl}, nil
}

// ReleaseUploadLease removes only the lease token acquired by this process.
func (v *Valkey) ReleaseUploadLease(ctx context.Context, lease UploadLease) error {
	if lease.Key == "" || lease.Token == "" {
		return nil
	}
	if _, err := v.client.DeleteIfValue(ctx, lease.Key, lease.Token); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return ErrUnavailable
	}
	return nil
}

// Close releases the underlying Valkey client.
func (v *Valkey) Close() error {
	return v.client.Close()
}

func newLeaseToken() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
