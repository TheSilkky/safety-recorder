package config

import (
	"fmt"
	"os"
	"strings"
)

// SecretValue describes one secret-bearing setting at a single precedence
// layer. Use either Value or File, not both.
type SecretValue struct {
	Value string
	File  string
}

// ResolveSecret resolves a direct secret value or a file-backed secret without
// including the secret or file path in returned errors.
func ResolveSecret(name string, secret SecretValue) (string, error) {
	value := strings.TrimSpace(secret.Value)
	file := strings.TrimSpace(secret.File)
	if value != "" && file != "" {
		return "", newConfigParseError(name, "direct secret and secret file are both configured")
	}
	if file == "" {
		return value, nil
	}

	contents, err := os.ReadFile(file)
	if err != nil {
		return "", newConfigParseError(name, "secret file cannot be read")
	}
	resolved := trimOneTrailingLineEnding(string(contents))
	if resolved == "" {
		return "", newConfigParseError(name, "secret file is empty")
	}
	return resolved, nil
}

func trimOneTrailingLineEnding(value string) string {
	if strings.HasSuffix(value, "\r\n") {
		return strings.TrimSuffix(value, "\r\n")
	}
	if strings.HasSuffix(value, "\n") {
		return strings.TrimSuffix(value, "\n")
	}
	return value
}

type ParseError struct {
	Name    string
	Message string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parse %s: %s", e.Name, e.Message)
}

func newConfigParseError(name, message string) error {
	return ParseError{Name: name, Message: message}
}
