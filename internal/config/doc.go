// Package config reads the server's small TOML, environment, and secret-file
// configuration.
//
// The backend keeps configuration limited to separate main and private-admin
// bind address lists, backend selectors, backend-specific settings, local data
// paths, upload size limits, account/auth controls, rate limits, and HTTP
// timeouts.
package config
