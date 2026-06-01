package coordination

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNoopCoordinator(t *testing.T) {
	coord := NewNone()

	if err := coord.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	lease, err := coord.AcquireUploadLease(context.Background(), "proofline:upload-operation:v1:hash", time.Minute)
	if err != nil {
		t.Fatalf("AcquireUploadLease: %v", err)
	}
	if !lease.Acquired {
		t.Fatalf("noop lease was not acquired: %+v", lease)
	}
	if err := coord.ReleaseUploadLease(context.Background(), lease); err != nil {
		t.Fatalf("ReleaseUploadLease: %v", err)
	}
	if err := coord.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestValkeyCoordinatorCheck(t *testing.T) {
	client := &fakePinger{}
	coord, err := NewValkey(client)
	if err != nil {
		t.Fatalf("NewValkey: %v", err)
	}

	if err := coord.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if client.pings != 1 {
		t.Fatalf("pings = %d, want 1", client.pings)
	}
}

func TestValkeyCoordinatorCheckFailureIsSafe(t *testing.T) {
	client := &fakePinger{
		pingErr: errors.New("dial 10.0.0.5:6379 with password secret failed"),
	}
	coord, err := NewValkey(client)
	if err != nil {
		t.Fatalf("NewValkey: %v", err)
	}

	err = coord.Check(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Check error = %v, want ErrUnavailable", err)
	}
	if strings.Contains(err.Error(), "10.0.0.5") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("coordination error exposed backend detail: %v", err)
	}
}

func TestValkeyCoordinatorClose(t *testing.T) {
	client := &fakePinger{closeErr: errors.New("close failed")}
	coord, err := NewValkey(client)
	if err != nil {
		t.Fatalf("NewValkey: %v", err)
	}

	err = coord.Close()
	if !errors.Is(err, client.closeErr) {
		t.Fatalf("Close error = %v, want %v", err, client.closeErr)
	}
	if client.closes != 1 {
		t.Fatalf("closes = %d, want 1", client.closes)
	}
}

func TestValkeyCoordinatorRateLimit(t *testing.T) {
	client := &fakePinger{counts: []int64{1, 2, 3}}
	coord, err := NewValkey(client)
	if err != nil {
		t.Fatalf("NewValkey: %v", err)
	}

	for index, wantAllowed := range []bool{true, true, false} {
		allowed, err := coord.Allow(context.Background(), "proofline:public-viewer-rate:v1:page:hash", 2, time.Minute)
		if err != nil {
			t.Fatalf("Allow %d: %v", index, err)
		}
		if allowed != wantAllowed {
			t.Fatalf("Allow %d = %t, want %t", index, allowed, wantAllowed)
		}
	}
	if client.increments != 3 {
		t.Fatalf("increments = %d, want 3", client.increments)
	}
	if client.keys[0] != "proofline:public-viewer-rate:v1:page:hash" {
		t.Fatalf("unexpected key %q", client.keys[0])
	}
	if client.ttls[0] != time.Minute {
		t.Fatalf("unexpected ttl %s", client.ttls[0])
	}
}

func TestValkeyCoordinatorRateLimitFailureIsSafe(t *testing.T) {
	client := &fakePinger{
		incrementErr: errors.New("dial 10.0.0.5:6379 with password secret failed"),
	}
	coord, err := NewValkey(client)
	if err != nil {
		t.Fatalf("NewValkey: %v", err)
	}

	allowed, err := coord.Allow(context.Background(), "proofline:public-viewer-rate:v1:data:hash", 1, time.Minute)
	if allowed {
		t.Fatal("expected Allow to deny on backend failure")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Allow error = %v, want ErrUnavailable", err)
	}
	if strings.Contains(err.Error(), "10.0.0.5") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("rate limit error exposed backend detail: %v", err)
	}
}

func TestValkeyCoordinatorUploadLease(t *testing.T) {
	client := &fakePinger{leaseResults: []bool{true, false}}
	coord, err := NewValkey(client)
	if err != nil {
		t.Fatalf("NewValkey: %v", err)
	}

	lease, err := coord.AcquireUploadLease(context.Background(), "proofline:upload-operation:v1:hash", 30*time.Second)
	if err != nil {
		t.Fatalf("AcquireUploadLease: %v", err)
	}
	if !lease.Acquired {
		t.Fatalf("lease was not acquired: %+v", lease)
	}
	if lease.Token == "" {
		t.Fatal("lease token was empty")
	}
	if client.leaseKeys[0] != "proofline:upload-operation:v1:hash" {
		t.Fatalf("unexpected lease key %q", client.leaseKeys[0])
	}
	if client.leaseTTLs[0] != 30*time.Second {
		t.Fatalf("lease ttl = %s, want 30s", client.leaseTTLs[0])
	}

	if err := coord.ReleaseUploadLease(context.Background(), lease); err != nil {
		t.Fatalf("ReleaseUploadLease: %v", err)
	}
	if client.releaseKeys[0] != lease.Key || client.releaseValues[0] != lease.Token {
		t.Fatalf("release did not use acquired lease token")
	}

	busy, err := coord.AcquireUploadLease(context.Background(), "proofline:upload-operation:v1:hash", time.Minute)
	if err != nil {
		t.Fatalf("AcquireUploadLease busy: %v", err)
	}
	if busy.Acquired {
		t.Fatalf("busy lease was acquired: %+v", busy)
	}
	if busy.RetryAfter != time.Minute {
		t.Fatalf("busy retry after = %s, want 1m", busy.RetryAfter)
	}
}

func TestValkeyCoordinatorUploadLeaseFailureIsSafe(t *testing.T) {
	client := &fakePinger{
		setNXErr: errors.New("dial 10.0.0.5:6379 with password secret failed"),
	}
	coord, err := NewValkey(client)
	if err != nil {
		t.Fatalf("NewValkey: %v", err)
	}

	_, err = coord.AcquireUploadLease(context.Background(), "proofline:upload-operation:v1:hash", time.Minute)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("AcquireUploadLease error = %v, want ErrUnavailable", err)
	}
	if strings.Contains(err.Error(), "10.0.0.5") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("upload lease error exposed backend detail: %v", err)
	}

	err = coord.ReleaseUploadLease(context.Background(), UploadLease{
		Key:   "proofline:upload-operation:v1:hash",
		Token: "server-generated-token",
	})
	if err != nil {
		t.Fatalf("ReleaseUploadLease without release error: %v", err)
	}

	client.deleteErr = errors.New("dial 10.0.0.5:6379 with password secret failed")
	err = coord.ReleaseUploadLease(context.Background(), UploadLease{
		Key:   "proofline:upload-operation:v1:hash",
		Token: "server-generated-token",
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ReleaseUploadLease error = %v, want ErrUnavailable", err)
	}
	if strings.Contains(err.Error(), "10.0.0.5") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("upload lease release error exposed backend detail: %v", err)
	}
}

func TestNewValkeyRequiresClient(t *testing.T) {
	_, err := NewValkey(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

type fakePinger struct {
	pingErr       error
	incrementErr  error
	setNXErr      error
	deleteErr     error
	closeErr      error
	counts        []int64
	leaseResults  []bool
	keys          []string
	ttls          []time.Duration
	leaseKeys     []string
	leaseValues   []string
	leaseTTLs     []time.Duration
	releaseKeys   []string
	releaseValues []string
	pings         int
	increments    int
	leases        int
	releases      int
	closes        int
}

func (f *fakePinger) Ping(context.Context) error {
	f.pings++
	return f.pingErr
}

func (f *fakePinger) IncrementWithExpiry(_ context.Context, key string, ttl time.Duration) (int64, error) {
	f.increments++
	f.keys = append(f.keys, key)
	f.ttls = append(f.ttls, ttl)
	if f.incrementErr != nil {
		return 0, f.incrementErr
	}
	if len(f.counts) == 0 {
		return int64(f.increments), nil
	}
	count := f.counts[0]
	f.counts = f.counts[1:]
	return count, nil
}

func (f *fakePinger) SetNXWithExpiry(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	f.leases++
	f.leaseKeys = append(f.leaseKeys, key)
	f.leaseValues = append(f.leaseValues, value)
	f.leaseTTLs = append(f.leaseTTLs, ttl)
	if f.setNXErr != nil {
		return false, f.setNXErr
	}
	if len(f.leaseResults) == 0 {
		return true, nil
	}
	result := f.leaseResults[0]
	f.leaseResults = f.leaseResults[1:]
	return result, nil
}

func (f *fakePinger) DeleteIfValue(_ context.Context, key, value string) (bool, error) {
	f.releases++
	f.releaseKeys = append(f.releaseKeys, key)
	f.releaseValues = append(f.releaseValues, value)
	if f.deleteErr != nil {
		return false, f.deleteErr
	}
	return true, nil
}

func (f *fakePinger) Close() error {
	f.closes++
	return f.closeErr
}
