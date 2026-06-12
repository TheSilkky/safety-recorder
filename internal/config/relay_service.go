package config

import (
	"fmt"
	"strings"
)

func relayServiceConfigFromSource(source configSource) (RelayServiceConfig, error) {
	token, err := secretFromSource(source, "SAFE_RELAY_SERVICE_AUTH_TOKEN", "SAFE_RELAY_SERVICE_AUTH_TOKEN_FILE")
	if err != nil {
		return RelayServiceConfig{}, err
	}
	token = strings.TrimSpace(token)
	if token != "" && len([]byte(token)) < minRelayServiceAuthTokenBytes {
		return RelayServiceConfig{}, fmt.Errorf("parse SAFE_RELAY_SERVICE_AUTH_TOKEN: token must be at least 32 bytes")
	}
	return RelayServiceConfig{AuthToken: token}, nil
}
