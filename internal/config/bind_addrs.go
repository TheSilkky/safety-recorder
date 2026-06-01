package config

import (
	"fmt"
	"os"
	"strings"
)

func mainBindAddrsFromEnv() ([]string, error) {
	return bindAddrsFromEnvWithLegacy(
		"SAFE_MAIN_BIND_ADDRS",
		"SAFE_MAIN_BIND_ADDR",
		"SAFE_PRIVATE_BIND_ADDRS",
		"SAFE_PRIVATE_BIND_ADDR",
		defaultMainBindAddr,
	)
}

func adminBindAddrsFromEnv() ([]string, error) {
	for _, legacyName := range []string{"SAFE_PUBLIC_BIND_ADDRS", "SAFE_PUBLIC_BIND_ADDR"} {
		if _, ok := os.LookupEnv(legacyName); ok {
			return nil, fmt.Errorf("%s is no longer supported for listener binding; set SAFE_MAIN_BIND_ADDRS for the main API/viewer listener and SAFE_ADMIN_BIND_ADDRS for the private admin listener", legacyName)
		}
	}
	return bindAddrsFromEnv("SAFE_ADMIN_BIND_ADDRS", "SAFE_ADMIN_BIND_ADDR", defaultAdminBindAddr)
}

func bindAddrsFromEnv(pluralName, singularName, fallback string) ([]string, error) {
	if raw, ok := os.LookupEnv(pluralName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pluralName, err)
		}
		return addrs, nil
	}
	if raw, ok := os.LookupEnv(singularName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", singularName, err)
		}
		return addrs, nil
	}
	return []string{fallback}, nil
}

func bindAddrsFromEnvWithLegacy(pluralName, singularName, legacyPluralName, legacySingularName, fallback string) ([]string, error) {
	if raw, ok := os.LookupEnv(pluralName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pluralName, err)
		}
		return addrs, nil
	}
	if raw, ok := os.LookupEnv(singularName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", singularName, err)
		}
		return addrs, nil
	}
	if raw, ok := os.LookupEnv(legacyPluralName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", legacyPluralName, err)
		}
		return addrs, nil
	}
	if raw, ok := os.LookupEnv(legacySingularName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", legacySingularName, err)
		}
		return addrs, nil
	}
	return []string{fallback}, nil
}

func parseBindAddrs(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	addrs := make([]string, 0, len(parts))
	for index, part := range parts {
		addr := strings.TrimSpace(part)
		if addr == "" {
			return nil, fmt.Errorf("bind address list contains empty entry at position %d", index+1)
		}
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("bind address list must contain at least one address")
	}
	return addrs, nil
}
