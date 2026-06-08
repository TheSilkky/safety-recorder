# Codex Prompt: Security Review

Review the backend and documentation for security issues.

Default mode is review-only: inspect current code, docs, and diffs and report
findings without editing files. Enter fix mode only when the maintainer requests
fixes or the prompt explicitly says to fix the findings.

Do **not** add new features.
Do **not** add React, Node, npm, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboards.
Do **not** include sensitive vulnerability details in public-facing docs or issue drafts.

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
- Do not expose `/v1` as an unreviewed public catch-all; keep `/v1/admin/...` and `/admin` off public edges.
- Do not log raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, wrapped-key ciphertext, private deployment details, stored paths, object keys, user safety data, or future token-like values; check logging changes against `docs/logging-requirements.md`.
- Do not add React, Node, npm, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features unless explicitly requested.
- Prefer Go standard library where practical.
- Preserve private/public listener separation.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour as an incidental implementation detail.
- Any key custody/decryption change must be an explicit security-sensitive task that updates the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.

## Review focus

Check:

- public route surface
- main `/v1` write/admin route exposure
- main/private-admin mux separation and public edge routing boundaries
- incident token generation
- token hashing
- token expiry and revocation
- raw token logging
- token leakage through URLs/referrers/logs
- request body / uploaded bytes / Authorization header logging
- structured logging field consistency with `docs/logging-requirements.md`
- no raw `err.Error()` logging in startup, request, upload, storage, token, key, auth, object-store, email, or user-safety paths
- wrapped-key ciphertext, stored path, object key, private deployment detail, and user safety data log leakage
- upload size limits
- SHA-256 verification
- temp file cleanup
- chunk overwrite prevention
- SQLite constraints
- migration safety
- media stream completion validation
- uploads to completed/failed streams
- ZIP bundle download authorization
- ZIP path traversal
- ZIP entry name sanitization
- filesystem path exposure
- download response headers
- `Cache-Control: no-store`
- `Referrer-Policy: no-referrer`
- `X-Content-Type-Options: nosniff`
- CSP / frame protection
- HTML template escaping
- panic/log leakage
- Docker bind exposure risks
- reverse-proxy/TLS assumptions
- simulator key file handling
- key file `.gitignore` coverage
- no raw keys in logs, docs examples, ZIP bundles, or SQLite unless explicitly designed
- no accidental backend decryption, key escrow, browser decryption, key sharing, or undocumented key custody changes
- AEAD associated-data consistency, if encryption code changed
- nonce/key misuse risks in docs/tests
- stale documentation that overpromises production readiness

Rate-limit coverage checks:

- route registration and rate-limit classifier coverage stay aligned
- every new main `/v1` route is assigned a rate-limit class or explicitly documented as intentionally unclassified
- authenticated write routes for accounts, contacts, sharing grants, wrapped keys, tokens, uploads, streams, deletion, and auth/session flows have appropriate rate-limit coverage
- browser cookie-auth routes such as web login, web logout, and CSRF are checked for rate-limit coverage or explicitly justified
- tests cover rate-limit classification for every registered main API route class where practical
- stale, unused, or misleading rate-limit config fields are removed, renamed, wired, or documented

## Sensitive finding handling

If you find a likely vulnerability that should not be public:

- do not write exploit details into a public issue draft
- do not include raw tokens, secrets, private deployment details, or user safety data
- recommend following `SECURITY.md`
- if this task creates notes, put sensitive notes in a clearly marked private draft location

## Output format

Return findings with severity:

- Critical
- High
- Medium
- Low
- Informational

For each finding, include:

- what is wrong
- why it matters
- minimal recommended fix
- affected files/routes if known
- whether it is safe for public issue tracking
- any registered routes that appear missing from the rate-limit classifier

If fixes are needed, make only minimal fixes and run:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```
