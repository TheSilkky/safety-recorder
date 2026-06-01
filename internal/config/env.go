package config

import (
	"os"
	"strings"
)

type configSource struct {
	values map[string]string
}

var secretFileEnvPairs = map[string]string{
	"SAFE_AUTH_BOOTSTRAP_SECRET_FILE": "SAFE_AUTH_BOOTSTRAP_SECRET",
	"SAFE_POSTGRES_DSN_FILE":          "SAFE_POSTGRES_DSN",
	"SAFE_S3_ACCESS_KEY_ID_FILE":      "SAFE_S3_ACCESS_KEY_ID",
	"SAFE_S3_SECRET_ACCESS_KEY_FILE":  "SAFE_S3_SECRET_ACCESS_KEY",
	"SAFE_S3_SESSION_TOKEN_FILE":      "SAFE_S3_SESSION_TOKEN",
	"SAFE_VALKEY_PASSWORD_FILE":       "SAFE_VALKEY_PASSWORD",
	"SAFE_SMTP_PASSWORD_FILE":         "SAFE_SMTP_PASSWORD",
}

var secretDirectEnvPairs = reverseSecretPairs(secretFileEnvPairs)

func newConfigSource(fileValues map[string]string) configSource {
	values := make(map[string]string, len(fileValues)+len(os.Environ()))
	for name, value := range fileValues {
		values[name] = value
	}

	envValues := make(map[string]string)
	for _, entry := range os.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		envValues[name] = value
	}

	for name, value := range envValues {
		if _, ok := secretFileEnvPairs[name]; ok {
			continue
		}
		values[name] = value
		if fileName, ok := secretDirectEnvPairs[name]; ok {
			delete(values, fileName)
		}
	}
	for fileName, directName := range secretFileEnvPairs {
		if value, ok := envValues[fileName]; ok {
			values[fileName] = value
			delete(values, directName)
		}
	}

	return configSource{values: values}
}

func reverseSecretPairs(pairs map[string]string) map[string]string {
	reversed := make(map[string]string, len(pairs))
	for fileName, directName := range pairs {
		reversed[directName] = fileName
	}
	return reversed
}

func (s configSource) Lookup(name string) (string, bool) {
	value, ok := s.values[name]
	return value, ok
}

func (s configSource) Get(name string) string {
	return s.values[name]
}

func envOrDefault(source configSource, name, fallback string) string {
	if value := source.Get(name); value != "" {
		return value
	}
	return fallback
}

func secretFromSource(source configSource, directName, fileName string) (string, error) {
	fileValue, fileSet := source.Lookup(fileName)
	if fileSet {
		fileValue = strings.TrimSpace(fileValue)
		if fileValue == "" {
			return "", newConfigParseError(fileName, "secret file path is required")
		}
		return ResolveSecret(directName, SecretValue{File: fileValue})
	}
	return strings.TrimSpace(source.Get(directName)), nil
}
