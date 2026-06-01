# Main API Public Exposure Listener Split Design

This document defines the listener split that prepares Proofline's main API
routes for future public exposure without exposing admin or operator surfaces.

The initial implementation is now in place: the default `127.0.0.1:8080` main
listener serves authenticated `/v1` routes and the read-only incident viewer,
while the default `127.0.0.1:8081` private-admin listener serves only the
`/admin` dashboard route tree. Existing `/v1/admin/...` JSON routes remain
authenticated admin-only routes on the main handler and must not be routed from
a public edge. This does not make the main `/v1` API production-ready public
infrastructure.

## Goals

- Define the target main `8080` listener route set.
- Define the target private `8081` admin-dashboard listener route set.
- Keep the public incident viewer route group on the main listener.
- Keep `/admin` and `/admin/...` off the public-facing main listener.
- Keep `/v1/admin/...` blocked from public reverse-proxy routes until a future
  private admin API route group is explicitly designed.
- Define required app-level rate-limit route classes before any main API route
  is publicly exposed.
- Preserve token redaction, ciphertext-only storage, server-controlled ZIP
  paths, and public viewer read-only behavior.
- Document browser credential, CSRF, audit, logging, configuration, and test
  gates for implementation issues.

## Non-Goals

- No declaration that the main `/v1` API is production-ready public
  infrastructure.
- No declaration that Proofline is production-ready public infrastructure.
- No OAuth, JWT, external identity provider, public account portal, public
  admin dashboard, web-client, iOS-client, Android-client, or protocol
  implementation.
- No backend decryption, browser decryption, raw server-held media keys, key
  escrow, break-glass access, or key-sharing behavior.
- No emergency-services integration, trusted-contact workflow, SMS, Messenger,
  email, push notification, or cloud deployment automation.

## Current Topology

The current server starts two handler trees:

| Listener group | Default address | Current routes | Current exposure |
|---|---:|---|---|
| Main API and viewer | `127.0.0.1:8080` | Authenticated `/v1/...` routes including existing admin-only JSON APIs, `/i/{token}`, `/i/{token}/data`, viewer bundle downloads, legacy `/e/{token}` aliases, `/static/...` | Public HTTPS/reverse proxy only after deployment-specific TLS, path routing, abuse controls, browser credential review, logging review, and operations are complete. Public edges must not route `/v1/admin/...`. |
| Private admin dashboard | `127.0.0.1:8081` | `/admin`, `/admin/...`, `/admin/static/...` | Localhost, LAN, WireGuard, firewall, or strict private reverse proxy only. |

Current `/v1` routes use local username/password accounts, opaque server-side
sessions, and app-level route-class rate limits. That is not a complete public
product API security model.

## Listener Topology

The implementation keeps two route trees:

| Target listener | Target default address | Route groups | Exposure |
|---|---:|---|---|
| Main API and viewer | `127.0.0.1:8080` | Authenticated product API routes, existing admin-only JSON APIs, and read-only incident viewer routes. | Public HTTPS/reverse proxy only after authentication, authorization, path routing, rate limits, logging redaction, browser credential rules, and deployment guidance are implemented and tested. Public edges must not route `/v1/admin/...`. |
| Private admin dashboard | `127.0.0.1:8081` | Private `/admin` dashboard routes and token-neutral `/admin/static/...` assets. | Localhost, LAN, WireGuard, VPN, firewall, or strict private reverse proxy only. Still authenticated and authorized. |

The `8080` main listener is public-facing only after the deployment has added
the required controls. It must not become a public catch-all for every current
admin/operator route, and public reverse proxies must block `/v1/admin/...`.

The target `8081` private admin-dashboard listener must not serve `/v1`, public
incident viewer routes, product upload routes, public link routes, public
viewer static assets, bundle routes, or general account-owner product traffic.

## Target Main `8080` Route Set

The main listener may serve these route classes after public hardening exists:

| Route group | Target placement | Required controls before exposure |
|---|---|---|
| `/i/{token}`, `/i/{token}/data`, `/i/{token}/streams/{stream_id}/download`, `/i/{token}/incident/download` | Main listener. | Keep read-only, bearer-token scoped, fail-closed, no-store, strict browser headers, route-pattern logging, and viewer rate limits. |
| Legacy `/e/{token}` aliases | Main listener while compatibility is retained. | Same as `/i/...`; prefer canonical `/i/...` for new links. |
| `/static/...` incident viewer assets | Main listener. | Token-neutral static assets only; no incident data, tokens, keys, or private deployment details. |
| `/v1/auth/login`, `/v1/auth/logout` | Main listener only if treated as public product authentication. | Per-route login abuse limits, audit, redacted errors, TLS, browser credential decision, and tests that the returned session cannot reach absent admin routes on the main listener. |
| `/v1/account`, `/v1/account/password` | Main listener as account-owner product routes. | Authenticated account scope, password-change rate limits, session revocation behavior, and CSRF protection if browser cookies are used. |
| `/v1/incidents`, `/v1/incidents/{incident_id}`, `/v1/incidents/{incident_id}/close`, owner-scoped `/v1/incidents/{incident_id}/deletion` | Main listener as account-owner product routes. | Owner/admin authorization review for public use, action/data-class policy, incident-mode non-escalation guarantees, audit, route limits, and deletion fail-closed tests. |
| `/v1/incidents/{incident_id}/chunks`, `/v1/incidents/{incident_id}/chunks/reconcile`, chunk metadata and authenticated chunk byte reads | Main listener as capture/account-owner product routes. | Body-size limits, upload rate limits, idempotency-key redaction, reconciliation response limits, immutable chunk guards, and slow-upload timeout review. |
| `/v1/incidents/{incident_id}/streams`, stream state routes, authenticated stream/incident encrypted bundle downloads | Main listener as account-owner product routes. | State-transition authorization, download limits, no-store ZIP headers, server-controlled ZIP entry names, and encrypted-bundle wording. |
| `/v1/incidents/{incident_id}/checkins` | Main listener as capture/account-owner product route. | Check-in rate limits, body limits, actor binding, and no notification side effects from labels alone. |
| `/v1/incidents/{incident_id}/incident-tokens`, `/v1/incident-tokens/{token_id}/revoke` | Main listener as account-owner sharing routes. | Grant-creation limits, token hash storage, raw token returned once, token-label redaction guidance, revoke audit, and no admin/operator grant management on the main listener. |

These routes should remain explicit product API routes. Existing
`/v1/admin/...` JSON routes are mounted on the main handler for compatibility
with the current local account API, but they remain admin-only, rate-limited,
and not public-ready. The private `/admin` dashboard bootstrap flow replaces
the legacy JSON first-admin bootstrap route.

## Target Private `8081` Route Set

The private admin-dashboard listener should serve only the private admin
dashboard route tree:

| Route group | Target placement | Notes |
|---|---|---|
| `/admin`, `/admin/login`, `/admin/bootstrap`, `/admin/logout`, `/admin/password`, `/admin/accounts/{account_id}/password` | Private `8081` only. | Keep HttpOnly SameSite admin cookie scoped to `/admin`, session-bound CSRF tokens for state-changing forms, no-store, and conservative browser headers. |
| `/admin/static/...` | Private `8081` only. | Token-neutral admin CSS only; no incident evidence, tokens, keys, or deployment details. |
| `/v1/admin/accounts`, `/v1/admin/accounts/{account_id}/password`, `/v1/admin/accounts/{account_id}/sessions/revoke` | Main handler, admin-only compatibility route. | Public reverse proxies must block these paths until a future private admin API route group is designed. |
| `GET` and `POST /v1/admin/incidents/{incident_id}/deletion` | Main handler, admin-only compatibility route. | Admin-global incident deletion remains an admin-only action and must not be routed from public entry points. |
| `/v1/bootstrap/admin` | Not mounted. | Use private `/admin/bootstrap`; remove the bootstrap secret after first-admin creation. |
| `/v1/health/live`, `/v1/health/ready` | Not mounted. | Do not publish operator readiness details on the main public origin or the dashboard-only listener. |

If future implementation renames admin API routes from `/v1/admin/...` to a
new private prefix or a separate private admin API listener, it should keep
compatibility or migration guidance explicit and keep that route tree separate
from the private `/admin` dashboard listener.

## Explicit Exclusions From The Main Listener

Public reverse proxies that forward to the main listener must not route:

- `/admin` or `/admin/...`
- `/v1/admin/...`
- `/v1/bootstrap/admin`
- `/v1/health/live` or `/v1/health/ready`
- operator maintenance commands or status pages
- migration, backup, restore, deletion-worker, or support routes
- escrow, break-glass, backend decryption, browser decryption, or raw-key routes
- public dashboard routes for admin/operator workflows

Public incident viewer routes must remain read-only. They must not create,
revoke, extend, or manage viewer tokens; change incident state; expose
deletion controls; expose grant controls; expose admin/operator state; release
wrapped keys; decrypt evidence; or return raw media keys.

## Rate-Limit Route Classes

Before any main API route is exposed beyond a private boundary, implementation
must add app-level route-class limits in addition to deployment-edge controls.
The current backend implements main API route-class limits on the existing
`/v1` handler tree without changing listener defaults. Those limits are a
prerequisite for future exposure, not a complete public security model. At
minimum:

| Class | Example routes | Purpose |
|---|---|---|
| Viewer page lookup | `GET /i/{token}`, `GET /e/{token}` | Slow bearer-token guessing and token enumeration. |
| Viewer data polling | `GET /i/{token}/data`, `GET /e/{token}/data` | Bound polling and refresh traffic. |
| Viewer download | Viewer stream and incident ZIP downloads | Protect bundle generation and storage reads. |
| Static asset | `/static/...` | Keep asset floods from bypassing route accounting. |
| Login/auth | `/v1/auth/login`, `/v1/auth/logout` | Slow password guessing and session churn. |
| Account/password | `/v1/account`, `/v1/account/password` | Bound password change and account self-service traffic. |
| Incident metadata write | Incident create, close, deletion, token creation/revocation | Bound state changes and grant creation. |
| Incident metadata read | Incident, stream, chunk, check-in metadata reads | Bound authenticated metadata scraping. |
| Upload body | Chunk uploads and future resumable upload routes | Protect request body handling, temp storage, hashing, and metadata writes. |
| Upload reconciliation/idempotency | Duplicate reconciliation and idempotent retry paths | Prevent metadata comparison and replay endpoints from becoming enumeration tools. |
| Private/API download | Private chunk bytes and authenticated bundle downloads | Protect storage reads and ZIP generation for authenticated callers. |
| Admin dashboard actions | Private `/admin` | Keep private admin abuse controls separate from public product controls. |
| Admin JSON API actions | `/v1/admin/...` | Admin-only compatibility routes on the main handler; block at public reverse proxies. |

Limiter keys must be server-controlled and must not include raw viewer tokens,
raw session tokens, Authorization headers, request bodies, uploaded bytes,
idempotency keys, plaintext, raw keys, stored paths, object keys, private
deployment details, or user safety narrative. When Valkey/Redis-compatible
coordination is configured, rate-limit keys should remain short-lived
coordination state and not durable evidence metadata.

## Browser Credentials And CSRF

Implementation must decide credential semantics per route group:

- Bearer Authorization sessions avoid browser CSRF by not relying on automatic
  cookie attachment, but they still require XSS, storage, referrer, and log
  redaction review.
- Browser cookie sessions on public product routes require `HttpOnly`,
  `Secure` on HTTPS, `SameSite` policy, CSRF tokens for every authenticated
  state-changing request, no-store responses, and tests for missing or invalid
  CSRF tokens.
- The existing admin web cookie remains scoped to `/admin` and private to the
  admin-dashboard listener.
- Public incident viewer bearer-token GET routes must stay read-only and
  should continue using `Referrer-Policy: no-referrer`, `Cache-Control:
  no-store`, `X-Content-Type-Options: nosniff`, restrictive
  `Permissions-Policy`, and strict CSP with `frame-ancestors 'none'`.

Do not add a broad browser account portal or public admin dashboard as part of
the listener split.

## Logging, Audit, And Redaction

The main listener will receive public traffic, so implementation must preserve
and extend the current redaction posture:

- Log route patterns instead of token-bearing paths.
- Redact canonical `/i/{token}` and legacy `/e/{token}` paths before token
  lookup.
- Do not log request bodies, uploaded bytes, Authorization headers, raw session
  tokens, raw viewer tokens, raw incident tokens, raw idempotency keys,
  plaintext, raw keys, stored paths, object keys, private deployment details,
  original filenames, location values, notes, or user safety narrative.
- Public errors should remain small and should not reveal whether a token is
  invalid, expired, revoked, deleting, deleted, or blocked by policy.
- Audit records for public product API actions should use non-sensitive actor,
  action, route class, incident, grant, and outcome identifiers.
- Admin/operator audit should stay private and must not expose plaintext, raw
  keys, raw tokens, or evidence bytes casually.

## Configuration Migration

Current configuration names describe the implemented topology:

- `SAFE_MAIN_BIND_ADDRS` serves the main API and read-only incident viewer
- `SAFE_ADMIN_BIND_ADDRS` serves the private `/admin` dashboard route tree

`SAFE_PRIVATE_BIND_ADDRS` and `SAFE_PRIVATE_BIND_ADDR` remain accepted as
legacy aliases for the main listener. `SAFE_PUBLIC_BIND_ADDRS` and
`SAFE_PUBLIC_BIND_ADDR` fail startup so a previously public viewer bind cannot
silently become the private-admin listener.

The target default ports are:

| Target role | Target address |
|---|---:|
| Main API and viewer | `127.0.0.1:8080` |
| Private admin dashboard | `127.0.0.1:8081` |

Container examples should continue binding to container addresses and relying
on host port publishing, firewall rules, WireGuard, or reverse proxy routing to
control real exposure. The docs must not imply that binding to separate ports
is a complete security model.

## Implementation Test Requirements

Implementation and follow-up issues should include tests that prove:

- main listener serves the intended public-ready product API routes
- main listener serves `/i/...`, legacy `/e/...`, and `/static/...`
- main listener returns not-found for `/admin`, `/admin/...`,
  `/v1/bootstrap/admin`, `/v1/health/live`, and `/v1/health/ready`
- main listener keeps `/v1/admin/...` authenticated, admin-only, rate-limited,
  and documented as blocked from public reverse-proxy routes
- private admin-dashboard listener serves `/admin`, `/admin/...`, and
  `/admin/static/...`
- private admin-dashboard listener does not serve `/v1`, public viewer routes,
  public viewer static assets, bundle routes, or account-owner upload/product
  routes
- public incident viewer routes remain read-only
- route-class rate limits cover viewer, auth, upload, metadata, sharing, and
  download routes before exposure
- browser-facing main and admin responses keep MDN-aligned security headers
  and no-store behavior where appropriate
- ZIP downloads keep `Content-Type: application/zip`, attachment disposition,
  no-store behavior, safe server-controlled entry names, and no filesystem or
  object-key exposure
- request logging uses redacted route patterns and does not include sensitive
  values
- simulator smoke tests use the updated main/viewer base URL after listener
  defaults change

The current deployment rule remains: `/admin` stays on the private dashboard
listener, and `/v1/admin/...` must be blocked from public reverse-proxy routes.
The main `/v1` API must not be exposed as production public infrastructure
until public deployment controls are explicitly reviewed for the deployment.
