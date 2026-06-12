# Codex Prompt: MDN-Aligned Web Security Header Review

Review browser-facing response headers and web security posture.

Do **not** add features.
Do **not** add React, Node, npm, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, Docker Compose, Kubernetes, or cloud integrations.
Do **not** change endpoint behaviour unless required to fix a security bug.
Do **not** add browser decryption or unrelated incident viewer features.

## Source of truth

Before making changes, read current source-of-truth files as relevant:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
- `docs/v1-preview-direction.md`
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
- Do not expose `/v1` as an unreviewed public catch-all; keep `/admin/api/...` and `/admin` off public edges.
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

## Goal

Ensure browser-facing behaviour is MDN-aligned for a small server-rendered incident viewer and private JSON API.

Use MDN guidance conceptually for:

- Content-Security-Policy
- X-Content-Type-Options
- Referrer-Policy
- Permissions-Policy
- Strict-Transport-Security
- X-Frame-Options or CSP `frame-ancestors`
- Cache-Control for token-protected incident viewer pages/downloads

## Review requirements

Check:

- public incident viewer HTML responses
- public incident viewer JSON responses
- static CSS/JS responses
- stream ZIP download responses
- incident ZIP download responses
- private API JSON responses
- 404/error responses

Look for:

- missing or incorrect `Content-Type`
- missing `X-Content-Type-Options: nosniff`
- missing or weak `Referrer-Policy`
- missing or weak `Content-Security-Policy`
- missing `frame-ancestors` or equivalent clickjacking protection
- unsafe inline scripts/styles
- token leakage through referrers
- token leakage through logs
- cacheable token-protected incident viewer pages
- cacheable incident viewer downloads
- incorrect headers on ZIP downloads
- headers that should be set by reverse proxy rather than app
- HSTS accidentally enabled for localhost/dev HTTP

## Implementation guidance

For incident viewer HTML, prefer strict CSP such as:

```http
Content-Security-Policy: default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'
```

If inline CSS/JS is introduced, either move it to static assets or explicitly justify the CSP change.

For token-protected incident viewer pages, JSON, errors, and downloads, ensure:

```http
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
Permissions-Policy: geolocation=(), microphone=(), camera=()
Cache-Control: no-store
```

For ZIP downloads, ensure:

```http
Content-Type: application/zip
Content-Disposition: attachment
X-Content-Type-Options: nosniff
Cache-Control: no-store
Referrer-Policy: no-referrer
```

For HSTS:

- Do not enable HSTS by default for localhost/dev HTTP.
- Document that HSTS should be enabled only when served over production HTTPS.
- Prefer setting HSTS at the production HTTPS reverse proxy.

## Tests

Add or update tests to verify security headers for:

- incident viewer HTML
- incident viewer JSON data endpoint
- static assets
- stream ZIP download responses
- incident ZIP download responses
- private API JSON responses where appropriate
- 404/error responses

## Documentation

Update `docs/security-model.md`, `docs/deployment.md`, or README only if header behaviour or deployment guidance changes.

## Validation

Run:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

## Output

Summarize:

1. headers added or changed
2. tests added/updated
3. documentation updated
4. security items intentionally deferred to reverse proxy/deployment
