# Codex Prompt: Code Review

Review current changes for correctness, maintainability, security, and scope control.

Do **not** add features unless needed to fix a bug.
Do **not** make broad refactors during review unless required to fix a blocking issue.

## Source of truth

Before making changes, read current source-of-truth files as relevant:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
- relevant files in `docs/`
- relevant source files
- relevant tests
- relevant issue or PR, if this is issue/PR work

Do not rely on stale assumptions from this prompt if the repository has changed.
## Global constraints

- Keep changes scoped to the task.
- Do not add unrelated features.
- Do not weaken security warnings.
- Do not claim production readiness.
- Do not expose `/v1` publicly.
- Do not log raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, or future token-like values.
- Do not add React, Node, npm, OAuth, JWT, user accounts, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features unless explicitly requested.
- Prefer Go standard library where practical.
- Preserve private/public listener separation.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour as an incidental implementation detail.
- Any key custody/decryption change must be an explicit security-sensitive task that updates the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the iPhone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.

## Review focus

Check for:

- upload overwrite risks
- temp file cleanup
- SHA-256 verification correctness
- duplicate chunk rejection
- upload size limits
- chunk-index semantics and stream completion assumptions
- SQLite constraints and migrations
- schema migration tracking, if touched
- HTTP server timeout settings, if touched
- stream completion/failure logic
- legacy chunks without `stream_id`
- evidence bundle ZIP path traversal
- ZIP manifest correctness
- server-controlled ZIP entry names
- download routes exposing filesystem paths
- private write/admin routes routed from public incident viewer edges
- public incident viewer routes mutating data
- raw token logging
- request body logging
- uploaded file byte logging
- Authorization header logging
- plaintext/key logging
- backend decryption, key escrow, browser decryption, or key sharing accidentally introduced or introduced without explicit design scope
- encryption envelope validation, if touched
- simulator key file handling, if touched
- template escaping
- security headers on incident viewer and downloads
- bind address parsing
- tests that do not assert important behaviour
- docs drift caused by code changes

## Output format

Return:

1. Critical issues
2. Important issues
3. Minor issues
4. Suggested minimal fixes
5. Tests/validation recommended

If you make changes:

- keep them small
- do not add unrelated features
- do not change public JSON field names unless required for a bug
- run:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
```

Then summarize what changed.
