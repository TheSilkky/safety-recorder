package relaycap

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	Version    = "proofline.relay-capability.v1"
	RoleUpload = "upload"
	RoleFanout = "fanout"

	tokenPrefix     = "proofline-relay-capability-v1"
	minSecretLength = 32
)

var (
	ErrInvalidSecret    = errors.New("invalid relay capability secret")
	ErrInvalidToken     = errors.New("invalid relay capability token")
	ErrInvalidSignature = errors.New("invalid relay capability signature")
	ErrInvalidClaims    = errors.New("invalid relay capability claims")
	ErrExpired          = errors.New("expired relay capability")
	ErrWrongRole        = errors.New("wrong relay capability role")
	ErrWrongBinding     = errors.New("wrong relay capability binding")
)

type Capability struct {
	Version           string   `json:"version"`
	RelaySessionID    string   `json:"relay_session_id"`
	Role              string   `json:"role"`
	IncidentID        string   `json:"incident_id"`
	StreamID          string   `json:"stream_id"`
	IssuedAtUnix      int64    `json:"iat"`
	ExpiresAtUnix     int64    `json:"exp"`
	MaxChunkBytes     int64    `json:"max_chunk_bytes"`
	MaxChunks         int      `json:"max_chunks"`
	AllowedMediaTypes []string `json:"allowed_media_types"`
}

type ValidationContext struct {
	Role           string
	RelaySessionID string
	IncidentID     string
	StreamID       string
	Now            time.Time
}

func NewSessionID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("relay session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random[:]), nil
}

func SecretBytes(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if len([]byte(trimmed)) < minSecretLength {
		return nil, ErrInvalidSecret
	}
	return []byte(trimmed), nil
}

func Sign(secret []byte, capability Capability) (string, error) {
	if len(secret) < minSecretLength {
		return "", ErrInvalidSecret
	}
	if err := validateClaims(capability); err != nil {
		return "", err
	}
	payload, err := json.Marshal(capability)
	if err != nil {
		return "", fmt.Errorf("relay capability payload: %w", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := tokenPrefix + "." + encodedPayload
	signature := sign(secret, signingInput)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func Validate(secret []byte, token string, ctx ValidationContext) (Capability, error) {
	if len(secret) < minSecretLength {
		return Capability{}, ErrInvalidSecret
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != tokenPrefix {
		return Capability{}, ErrInvalidToken
	}
	signingInput := parts[0] + "." + parts[1]
	gotSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Capability{}, ErrInvalidToken
	}
	wantSignature := sign(secret, signingInput)
	if !hmac.Equal(gotSignature, wantSignature) {
		return Capability{}, ErrInvalidSignature
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Capability{}, ErrInvalidToken
	}
	var capability Capability
	if err := json.Unmarshal(payload, &capability); err != nil {
		return Capability{}, ErrInvalidToken
	}
	if err := validateClaims(capability); err != nil {
		return Capability{}, err
	}

	now := ctx.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !now.Before(time.Unix(capability.ExpiresAtUnix, 0).UTC()) {
		return Capability{}, ErrExpired
	}
	if ctx.Role != "" && capability.Role != ctx.Role {
		return Capability{}, ErrWrongRole
	}
	if ctx.RelaySessionID != "" && capability.RelaySessionID != ctx.RelaySessionID {
		return Capability{}, ErrWrongBinding
	}
	if ctx.IncidentID != "" && capability.IncidentID != ctx.IncidentID {
		return Capability{}, ErrWrongBinding
	}
	if ctx.StreamID != "" && capability.StreamID != ctx.StreamID {
		return Capability{}, ErrWrongBinding
	}
	return capability, nil
}

func sign(secret []byte, input string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(input))
	return mac.Sum(nil)
}

func validateClaims(capability Capability) error {
	if capability.Version != Version ||
		capability.RelaySessionID == "" ||
		capability.Role == "" ||
		capability.IncidentID == "" ||
		capability.StreamID == "" ||
		capability.IssuedAtUnix <= 0 ||
		capability.ExpiresAtUnix <= capability.IssuedAtUnix ||
		capability.MaxChunkBytes <= 0 ||
		capability.MaxChunks <= 0 ||
		len(capability.AllowedMediaTypes) == 0 {
		return ErrInvalidClaims
	}
	return nil
}
