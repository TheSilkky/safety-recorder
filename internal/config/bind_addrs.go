package config

import (
	"fmt"
	"strings"
)

func mainBindAddrsFromSource(source configSource) ([]string, error) {
	return bindAddrsFromSourceWithLegacy(
		source,
		"SAFE_MAIN_BIND_ADDRS",
		"SAFE_MAIN_BIND_ADDR",
		"SAFE_PRIVATE_BIND_ADDRS",
		"SAFE_PRIVATE_BIND_ADDR",
		defaultMainBindAddr,
	)
}

func adminBindAddrsFromSource(source configSource) ([]string, error) {
	for _, legacyName := range []string{"SAFE_PUBLIC_BIND_ADDRS", "SAFE_PUBLIC_BIND_ADDR"} {
		if _, ok := source.Lookup(legacyName); ok {
			return nil, fmt.Errorf("%s is no longer supported for listener binding; set SAFE_MAIN_BIND_ADDRS for the main API/viewer listener and SAFE_ADMIN_BIND_ADDRS for the private admin listener", legacyName)
		}
	}
	return bindAddrsFromSource(source, "SAFE_ADMIN_BIND_ADDRS", "SAFE_ADMIN_BIND_ADDR", defaultAdminBindAddr)
}

func bindAddrsFromSource(source configSource, pluralName, singularName, fallback string) ([]string, error) {
	if raw, ok := source.Lookup(pluralName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pluralName, err)
		}
		return addrs, nil
	}
	if raw, ok := source.Lookup(singularName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", singularName, err)
		}
		return addrs, nil
	}
	return []string{fallback}, nil
}

func bindAddrsFromSourceWithLegacy(source configSource, pluralName, singularName, legacyPluralName, legacySingularName, fallback string) ([]string, error) {
	if raw, ok := source.Lookup(pluralName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pluralName, err)
		}
		return addrs, nil
	}
	if raw, ok := source.Lookup(singularName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", singularName, err)
		}
		return addrs, nil
	}
	if raw, ok := source.Lookup(legacyPluralName); ok {
		addrs, err := parseBindAddrs(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", legacyPluralName, err)
		}
		return addrs, nil
	}
	if raw, ok := source.Lookup(legacySingularName); ok {
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
