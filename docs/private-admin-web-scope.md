# Private Admin Web Scope

Status: current scope and validation guidance. This document does not add
routes, change runtime behavior, approve public deployment, add a frontend app,
or make Proofline production-ready public infrastructure.

The private admin web interface is the server-rendered operator surface for
local and private-network maintenance. It exists to make already-implemented
private admin workflows usable without asking operators to hand-craft JSON
requests. It is not a public admin dashboard and it is not a user-facing
account portal.

## Route Boundary

The admin web route tree remains private-admin only:

- `/admin`
- `/admin/...`
- `/admin/static/...`
- `/admin/api/...`

These routes are mounted on the private-admin listener, outside the `/v1` API
namespace and outside the public incident viewer route tree. Public reverse
proxies and public web-client origins must not route them. Keep the listener
behind localhost, LAN, WireGuard, firewall rules, VPN, or a strict private
reverse proxy. Separate bind addresses reduce accidental exposure but do not
replace authentication, authorization, CSRF checks, security headers, safe
logging, and deployment review.

The token-neutral files under `/admin/static/...` can be served without an
admin session because they contain no incident data, tokens, keys, credentials,
or deployment details. They still belong to the private admin route tree and
must not be forwarded from public entry points.

## Implementation Shape

The admin web surface stays in the Go server backend:

- Go handlers and session helpers live in `internal/httpapi/admin_web*.go`.
- The page renders `internal/httpapi/web/templates/admin.html` through
  `html/template`.
- Static assets are embedded with Go `embed` from
  `internal/httpapi/web/admin/static`.
- Generated CSS is checked in at
  `internal/httpapi/web/admin/static/styles.css`.
- Source CSS lives at `internal/httpapi/web/admin/tailwind.css`; the
  maintainer-only build command is documented in
  [Private admin Tailwind CSS](admin-web-tailwind.md).

Do not add a repo-local React app, Vite app, package manager runtime, public
asset pipeline, or separate admin frontend to this server repository for this
surface. Proofline brand icons and favicons may be embedded when they are
token-neutral static assets and remain under the private admin static prefix.

## Current Capabilities

The current `/admin` surface supports:

- first-admin bootstrap when no admin exists and the bootstrap secret is
  configured
- admin login and logout using the same account/session store as the JSON API
- completed admin second-factor setup before operator actions
- active-factor session verification when email challenge, TOTP, or WebAuthn
  is active
- configured WebAuthn/FIDO2 passkey or security-key setup as the preferred
  admin second-factor path, with email challenge fallback only when mail
  delivery is configured and TOTP setup through the existing authenticated
  second-factor API
- local account listing
- local account creation
- current-admin password change with current-password verification
- another account's password reset
- another account's session revocation
- another account's second-factor recovery reset with controlled reason codes
- count-oriented legacy unowned incident review
- legacy unowned incident assignment or keep-unowned decisions with controlled
  reason codes
- admin-global incident deletion requests with existing reason-code behavior
- non-sensitive deletion status lookup for an operator-provided incident ID

State-changing authenticated forms use a session-bound CSRF token. The web
surface blocks unsafe current-admin self-reset actions from per-account forms.

## Visual Direction

The private admin web interface should feel like a quiet operator console that
matches Proofline's current web-client and website visual language without
turning into a marketing page or a standalone product shell.

Use dense, scannable sections for repeated operator work. Prefer clear form
labels, predictable tables, concise status panels, and compact error/notice
states. Avoid placeholder layouts, public landing-page structure, evidence
browser affordances, or decorative pages that obscure the operator task. Keep
the first screen focused on private admin state, account controls, incident
operations, and the private-boundary status.

## Sensitive Data Display Rules

The admin web interface may show only safe operator metadata needed for the
implemented workflows, such as account IDs, usernames, roles, account creation
times, controlled reason codes, safe route-boundary status, count-oriented
legacy unowned incident metadata, and non-sensitive deletion status fields.

It must not display or log:

- evidence bytes or decrypted evidence
- incident notes, transcripts, free-form user safety narratives, or full
  location context
- request bodies, uploaded bytes, original filenames, or Authorization headers
- raw viewer, incident, registration, email challenge, session, CSRF, or
  idempotency tokens
- password hashes, raw passwords, TOTP seeds, `otpauth_url` values, WebAuthn
  challenge values, WebAuthn client data JSON, or credential bytes
- plaintext, raw keys, raw media keys, contact private keys, wrapped-key
  ciphertext, or browser fragment secrets
- stored paths, staging paths, object keys, private filesystem paths, private
  endpoints, DSNs, SMTP credentials, secret file paths, or other private
  deployment details

If a future operator workflow needs broader support context, add a narrow
issue and review the display, logging, API, retention, and public-artifact
impact before implementing it.

## Non-Goals

The private admin web surface does not provide:

- public admin tooling
- a public account portal
- broad public `/v1` exposure approval
- browser recording, browser decryption, backend decryption, key escrow, or
  break-glass raw-key access
- an incident evidence browser, media player, decrypted export, or playable
  bundle export
- OAuth, JWT, billing, notification providers, emergency-services integration,
  or hosted deployment automation
- public production-readiness claims

## Validation Expectations

For docs-only admin web scope changes, run:

```bash
scripts/check-markdown-links.py
git diff --check
```

If a change touches Go handlers, templates, embedded assets, generated CSS, or
route behavior, also run the relevant implementation validation:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
scripts/check-markdown-links.py
git diff --check
```

Route or template changes should also be reviewed for private/public listener
separation, admin second-factor enforcement, CSRF coverage for unsafe forms,
no-store and browser security headers, token-neutral static assets, safe
errors, safe logging, and the sensitive-data display rules above.
