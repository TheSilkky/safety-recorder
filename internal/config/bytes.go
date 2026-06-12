package config

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func maxUploadBytesFromSource(source configSource) (int64, error) {
	maxUploadBytes := defaultMaxUploadBytes
	if raw := source.Get("SAFE_MAX_UPLOAD_BYTES"); raw != "" {
		parsed, err := parseBytes(raw)
		if err != nil {
			return 0, fmt.Errorf("parse SAFE_MAX_UPLOAD_BYTES: %w", err)
		}
		maxUploadBytes = parsed
	}
	return maxUploadBytes, nil
}

func accountDefaultBlobQuotaBytesFromSource(source configSource) (int64, error) {
	quotaBytes := defaultAccountDefaultBlobQuotaBytes
	if raw := source.Get("SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES"); raw != "" {
		parsed, err := parseBytes(raw)
		if err != nil {
			return 0, fmt.Errorf("parse SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES: %w", err)
		}
		quotaBytes = parsed
	}
	return quotaBytes, nil
}

func tempUploadStagingQuotaBytesFromSource(source configSource) (int64, error) {
	quotaBytes := defaultTempUploadStagingQuotaBytes
	if raw := source.Get("SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES"); raw != "" {
		parsed, err := parseBytes(raw)
		if err != nil {
			return 0, fmt.Errorf("parse SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES: %w", err)
		}
		quotaBytes = parsed
	}
	return quotaBytes, nil
}

func parseBytes(raw string) (int64, error) {
	value := strings.TrimSpace(strings.ToUpper(raw))
	if value == "" {
		return 0, fmt.Errorf("empty byte value")
	}

	units := []struct {
		suffix string
		size   int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"G", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"M", 1024 * 1024},
		{"KB", 1024},
		{"K", 1024},
		{"B", 1},
	}

	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			return parseUnitBytes(number, unit.size)
		}
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("byte value must be positive")
	}
	if parsed > maxConfiguredUploadBytes {
		return 0, fmt.Errorf("byte value is too large")
	}
	return parsed, nil
}

func parseUnitBytes(rawNumber string, multiplier int64) (int64, error) {
	number := strings.TrimSpace(rawNumber)
	parsed, ok := new(big.Rat).SetString(number)
	if !ok {
		return 0, fmt.Errorf("invalid byte value")
	}
	if parsed.Sign() <= 0 {
		return 0, fmt.Errorf("byte value must be positive")
	}

	bytes := new(big.Rat).Mul(parsed, big.NewRat(multiplier, 1))
	if bytes.Cmp(big.NewRat(1, 1)) < 0 {
		return 0, fmt.Errorf("byte value must be at least one byte")
	}
	if bytes.Cmp(big.NewRat(maxConfiguredUploadBytes, 1)) > 0 {
		return 0, fmt.Errorf("byte value is too large")
	}

	truncated := new(big.Int).Quo(bytes.Num(), bytes.Denom())
	return truncated.Int64(), nil
}
