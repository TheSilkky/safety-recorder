package relaycap

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSignAndValidateCapability(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	capability := testCapability(now)

	token, err := Sign(secret, capability)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	got, err := Validate(secret, token, ValidationContext{
		Role:           RoleUpload,
		RelaySessionID: capability.RelaySessionID,
		IncidentID:     capability.IncidentID,
		StreamID:       capability.StreamID,
		Now:            now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.RelaySessionID != capability.RelaySessionID {
		t.Fatalf("RelaySessionID = %q, want %q", got.RelaySessionID, capability.RelaySessionID)
	}
}

func TestValidateRejectsTamperingExpiryRoleAndBinding(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	capability := testCapability(now)
	token, err := Sign(secret, capability)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	parts := strings.Split(token, ".")
	tamperedSignature := []byte(parts[2])
	if tamperedSignature[0] == 'A' {
		tamperedSignature[0] = 'B'
	} else {
		tamperedSignature[0] = 'A'
	}
	tampered := parts[0] + "." + parts[1] + "." + string(tamperedSignature)
	if _, err := Validate(secret, tampered, ValidationContext{Role: RoleUpload, IncidentID: capability.IncidentID, StreamID: capability.StreamID, Now: now}); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered Validate error = %v, want ErrInvalidSignature", err)
	}
	if _, err := Validate(secret, token, ValidationContext{Role: RoleUpload, IncidentID: capability.IncidentID, StreamID: capability.StreamID, Now: now.Add(10 * time.Minute)}); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired Validate error = %v, want ErrExpired", err)
	}
	if _, err := Validate(secret, token, ValidationContext{Role: "fanout", IncidentID: capability.IncidentID, StreamID: capability.StreamID, Now: now}); !errors.Is(err, ErrWrongRole) {
		t.Fatalf("wrong role Validate error = %v, want ErrWrongRole", err)
	}
	if _, err := Validate(secret, token, ValidationContext{Role: RoleUpload, RelaySessionID: "other-session", IncidentID: capability.IncidentID, StreamID: capability.StreamID, Now: now}); !errors.Is(err, ErrWrongBinding) {
		t.Fatalf("wrong session Validate error = %v, want ErrWrongBinding", err)
	}
	if _, err := Validate(secret, token, ValidationContext{Role: RoleUpload, IncidentID: "other-incident", StreamID: capability.StreamID, Now: now}); !errors.Is(err, ErrWrongBinding) {
		t.Fatalf("wrong incident Validate error = %v, want ErrWrongBinding", err)
	}
	if _, err := Validate(secret, token, ValidationContext{Role: RoleUpload, IncidentID: capability.IncidentID, StreamID: "other-stream", Now: now}); !errors.Is(err, ErrWrongBinding) {
		t.Fatalf("wrong stream Validate error = %v, want ErrWrongBinding", err)
	}
}

func TestCapabilityPayloadOmitsRawSessionAndKeyMaterial(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	capability := testCapability(time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC))
	token, err := Sign(secret, capability)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d, want 3", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	for _, disallowed := range []string{
		"raw-session-token-secret",
		"Authorization",
		"viewer_token",
		"incident_token",
		"object_key",
		"stored_path",
		"wrapped_key",
		"plaintext",
		"raw_key",
		"gps",
		"latitude",
		"longitude",
	} {
		if strings.Contains(string(payload), disallowed) {
			t.Fatalf("capability payload exposed %q: %s", disallowed, string(payload))
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode payload JSON: %v", err)
	}
	if _, ok := decoded["max_chunk_bytes"]; !ok {
		t.Fatalf("payload omitted bounded max_chunk_bytes: %s", string(payload))
	}
	if _, ok := decoded["max_chunks"]; !ok {
		t.Fatalf("payload omitted bounded max_chunks: %s", string(payload))
	}
}

func TestSecretBytesRequiresConfiguredSecret(t *testing.T) {
	if _, err := SecretBytes("short"); !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("short secret error = %v, want ErrInvalidSecret", err)
	}
	if _, err := SecretBytes("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("SecretBytes valid secret: %v", err)
	}
}

func testCapability(now time.Time) Capability {
	return Capability{
		Version:           Version,
		RelaySessionID:    "relay_session_1",
		Role:              RoleUpload,
		IncidentID:        "incident_1",
		StreamID:          "stream_1",
		IssuedAtUnix:      now.Unix(),
		ExpiresAtUnix:     now.Add(5 * time.Minute).Unix(),
		MaxChunkBytes:     1024,
		MaxChunks:         64,
		AllowedMediaTypes: []string{"audio"},
	}
}
