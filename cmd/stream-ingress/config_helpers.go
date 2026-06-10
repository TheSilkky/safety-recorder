package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func secretEnv(name, fileName string) (string, error) {
	filePath := strings.TrimSpace(os.Getenv(fileName))
	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", configParseError{Name: fileName, Message: "secret file could not be read"}
		}
		secret := strings.TrimSuffix(strings.TrimSuffix(string(content), "\n"), "\r")
		if secret == "" {
			return "", configParseError{Name: fileName, Message: "secret file is empty"}
		}
		return secret, nil
	}
	return strings.TrimSpace(os.Getenv(name)), nil
}

func normalizeCoreBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", configParseError{Name: "SAFE_STREAM_INGRESS_CORE_BASE_URL", Message: "valid http or https URL is required"}
	}
	if parsed.User != nil {
		return "", configParseError{Name: "SAFE_STREAM_INGRESS_CORE_BASE_URL", Message: "URL userinfo is not allowed"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", configParseError{Name: "SAFE_STREAM_INGRESS_CORE_BASE_URL", Message: "http or https URL is required"}
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func positiveBytes(name, raw string) (int64, error) {
	value, err := parseByteCount(raw)
	if err != nil || value <= 0 {
		return 0, configParseError{Name: name, Message: "positive byte count is required"}
	}
	if value > int64(1<<63-1-relayUploadMultipartOverhead) {
		return 0, configParseError{Name: name, Message: "byte count is too large"}
	}
	return value, nil
}

func parseByteCount(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("empty byte count")
	}
	upper := strings.ToUpper(value)
	multiplier := int64(1)
	for _, suffix := range []struct {
		text       string
		multiplier int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"G", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"M", 1024 * 1024},
		{"KB", 1024},
		{"K", 1024},
		{"B", 1},
	} {
		if strings.HasSuffix(upper, suffix.text) {
			multiplier = suffix.multiplier
			value = strings.TrimSpace(value[:len(value)-len(suffix.text)])
			break
		}
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid byte count")
	}
	result := parsed * float64(multiplier)
	if result < 1 || result > float64(int64(1<<63-1)) {
		return 0, fmt.Errorf("byte count out of range")
	}
	return int64(result), nil
}

func positiveDuration(name, raw string) (time.Duration, error) {
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, configParseError{Name: name, Message: "positive duration is required"}
	}
	return value, nil
}

func positiveInt(name, raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, configParseError{Name: name, Message: "positive integer is required"}
	}
	return value, nil
}
