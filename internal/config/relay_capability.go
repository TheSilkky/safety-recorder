package config

import (
	"fmt"
	"strings"
)

const minRelayCapabilitySecretBytes = 32

func relayCapabilityConfigFromSource(source configSource) (RelayCapabilityConfig, error) {
	secret, err := secretFromSource(source, "SAFE_RELAY_CAPABILITY_SECRET", "SAFE_RELAY_CAPABILITY_SECRET_FILE")
	if err != nil {
		return RelayCapabilityConfig{}, err
	}
	ttl, err := durationFromSource(source, "SAFE_RELAY_CAPABILITY_TTL", defaultRelayCapabilityTTL)
	if err != nil {
		return RelayCapabilityConfig{}, err
	}
	if ttl <= 0 {
		return RelayCapabilityConfig{}, fmt.Errorf("parse SAFE_RELAY_CAPABILITY_TTL: duration must be positive")
	}
	maxChunks, err := positiveIntFromSource(source, "SAFE_RELAY_CAPABILITY_MAX_CHUNKS", defaultRelayCapabilityMaxChunks)
	if err != nil {
		return RelayCapabilityConfig{}, err
	}
	secret = strings.TrimSpace(secret)
	if secret != "" && len([]byte(secret)) < minRelayCapabilitySecretBytes {
		return RelayCapabilityConfig{}, fmt.Errorf("parse SAFE_RELAY_CAPABILITY_SECRET: secret must be at least 32 bytes")
	}
	return RelayCapabilityConfig{
		Secret:    secret,
		TTL:       ttl,
		MaxChunks: maxChunks,
	}, nil
}
