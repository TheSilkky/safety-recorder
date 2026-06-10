# Public Web-Client Deployment Boundary

Status: design context only. This document does not deploy the web client, add
routes, change CORS or CSRF behavior, change cookie settings, implement browser
decryption, replace the built-in viewer, or approve broad public `/v1`
exposure.

This document defines the server, route, browser credential, cache, CORS, CSRF,
logging, and edge assumptions required before a public Proofline web-client
origin can be treated as a reviewed v1 preview deployment path.

## Summary

The future `open-proofline/web-client` is v1-critical, but a public static web
client is not safe merely because it is a static site. A public web client
creates a browser credential boundary, a cross-origin API boundary, and a route
exposure boundary for the server.

The deployment model should be:

```text
browser
  -> public web-client origin
  -> exact-origin credentialed CORS to selected main /v1 route groups
  -> Proofline main API listener

operator/admin browser
  -> private admin origin or private network only
  -> private-admin listener
```

The public web-client origin may call only route groups that have been reviewed
for that deployment. Public reverse proxies must not forward wildcard traffic
to the main API and must never route `/admin`, `/admin/...`, or
`/v1/admin/...` from public entry points.

## Current Status

Implemented server primitives relevant to this boundary:

- local username/password accounts
- opaque server-side sessions
- optional browser-cookie sessions for main `/v1` routes
- exact-origin credentialed CORS for configured web origins
- CSRF tokens for unsafe cookie-authenticated main `/v1` requests
- owner-only public-safe incident list/detail reads
- account/device recipient-key metadata routes
- trusted-contact relationship metadata routes
- trusted-contact public-key metadata routes
- owner-scoped sharing-grant and wrapped-key metadata routes
- signed-in trusted-contact wrapped-key metadata reads
- app-level route-class limits for main API and public viewer routes
- private `/v1/admin/...` JSON routes on the private-admin listener only
- private `/admin` dashboard routes on the private-admin listener only

Not implemented:

- public production web-client deployment
- browser decryption
- browser recording
- trusted-contact incident reads beyond wrapped-key metadata
- account/device wrapped-key delivery
- viewer-route replacement
- public account portal production hardening
- broad public `/v1` deployment approval
- backend decryption, server escrow, break-glass, or raw-key access

## Deployment Classes

Use deployment classes so a preview does not accidentally widen route exposure.

| Deployment class | Allowed purpose | Route posture |
|---|---|---|
| Metadata account portal | Login, account read/change, owner incident metadata reads, contact/key/grant metadata review. | Smallest public web-client surface. No upload, chunk read, encrypted bundle download, viewer replacement, or decryption. |
| Evidence capture/review preview | Browser client may create incidents, upload encrypted chunks, manage streams, check in, and download encrypted bundles for client-side review. | Requires explicit upload/download/body-size/quota/abuse/logging review before exposure. |
| No-account viewer replacement | Public token viewer moves to the web client. | Use the fragment-token route decision in [web-client viewer routing](web-client-viewer-routing.md). Viewer tokens remain bearer secrets. |
| Browser decrypting viewer | Browser decrypts evidence locally. | Depends on the trust gate in [browser-decryption.md](browser-decryption.md). A dynamic same-origin decrypting viewer is not enough for production trusted-contact access. |

Do not mix these classes in release notes or deployment claims. A metadata
account portal deployment is not evidence-upload readiness. An evidence
capture/review preview is not browser decryption readiness. A no-account viewer
replacement is not trusted-contact decryption.

## Route Groups Allowed From A Public Web-Client Origin

All allowed route groups still require the current route authentication,
authorization, body-size limits, route-class limits, safe errors, no-store
responses where applicable, and deployment logging review.

### Browser Auth And Account Session

Allowed when `SAFE_WEB_AUTH_ENABLED=true` and the origin is explicitly listed in
`SAFE_WEB_ALLOWED_ORIGINS`:

- `POST /v1/auth/web/login`
- `GET /v1/auth/web/csrf`
- `POST /v1/auth/web/logout`
- `GET /v1/account`
- `POST /v1/account/second-factor/email/challenge`
- `POST /v1/account/second-factor/email/verify`
- `POST /v1/account/second-factor/totp/enroll`
- `POST /v1/account/second-factor/totp/confirm`
- `POST /v1/account/second-factor/totp/verify`
- `POST /v1/account/second-factor/webauthn/register/start`
- `POST /v1/account/second-factor/webauthn/register/finish`
- `POST /v1/account/second-factor/webauthn/verify/start`
- `POST /v1/account/second-factor/webauthn/verify/finish`
- `POST /v1/account/password`

The web client must call credentialed requests with:

```javascript
credentials: "include"
```

Unsafe cookie-authenticated requests must include the CSRF header returned by
`GET /v1/auth/web/csrf`. Bearer-authenticated API clients keep bearer behavior
and do not require CSRF. Requests that send both bearer and browser-cookie
credentials are rejected by the server and should be treated as a client bug.
Email, TOTP, and WebAuthn second-factor setup routes are available to
setup-incomplete sessions. Active TOTP and WebAuthn accounts can use only the
matching verification routes before product-route access. Raw email challenge
codes are delivered only by email. TOTP seeds and `otpauth_url` values are
returned only by enrollment. WebAuthn challenge values, client data JSON, and
credential bytes must not be logged or persisted outside the browser ceremony
and server challenge/credential stores. Raw challenge codes, TOTP codes, TOTP
seeds, `otpauth_url` values, and WebAuthn ceremony material must not be
persisted in browser storage, logs, analytics, URLs, or support artifacts.

### Registration And Email Verification

Allowed only for deployments that deliberately enable open self-hosted
registration:

- `POST /v1/auth/register`
- `POST /v1/auth/email/verify`

Open registration requires SMTP configuration and a reviewed
`SAFE_PUBLIC_WEB_ORIGIN`. Verification tokens belong in URL fragments for the
web client to submit in the JSON body; they must not be placed in server paths,
query strings, analytics, logs, or public issue text. Paid registration remains
a fail-closed placeholder.

### Metadata Account Portal

Allowed for a metadata-only public web-client deployment after route review:

- account/device recipient-key metadata routes
- trusted-contact relationship metadata routes
- trusted-contact public-key metadata routes
- owner-scoped sharing-grant metadata routes
- owner-scoped wrapped-key metadata routes
- signed-in trusted-contact wrapped-key metadata read routes
- owner-scoped viewer-token metadata list/read routes for owned incidents
- `GET /v1/incidents`
- `GET /v1/incidents/{incident_id}`

These routes remain authenticated and account-scoped. They do not expose raw
private keys, raw CEKs, raw viewer tokens, token hashes, plaintext, backend
decryption, trusted-contact incident reads, browser decryption, public viewer
privileges, or emergency dispatch. Wrapped-key ciphertext is access-enabling
metadata and must not be logged or sent to analytics.

### Evidence Capture And Encrypted Review

Allowed only for a deployment that explicitly claims browser evidence
capture/review preview readiness and has completed the review items in this
document:

- `POST /v1/incidents`
- `POST /v1/incidents/{incident_id}/close`
- `POST /v1/incidents/{incident_id}/deletion`
- `GET /v1/incidents/{incident_id}/deletion`
- `POST /v1/incidents/{incident_id}/streams`
- `GET /v1/incidents/{incident_id}/streams`
- `GET /v1/incidents/{incident_id}/streams/{stream_id}`
- `POST /v1/incidents/{incident_id}/streams/{stream_id}/complete`
- `POST /v1/incidents/{incident_id}/streams/{stream_id}/fail`
- `POST /v1/incidents/{incident_id}/chunks`
- `POST /v1/incidents/{incident_id}/chunks/reconcile`
- `POST /v1/incidents/{incident_id}/checkins`
- `GET /v1/incidents/{incident_id}/streams/{stream_id}/download`
- `GET /v1/incidents/{incident_id}/download`

Additional review requirements for this class:

- account-scoped committed blob quota or another accepted abuse/cost control
- upload body-size and timeout coordination between edge, proxy, and Go app
- complete-upload retry and idempotency-key guidance for the browser client
- CORS preflight behavior tested for multipart upload and JSON routes
- no raw idempotency keys, request bodies, uploaded bytes, plaintext, raw keys,
  stored paths, staging paths, object keys, GPS/location context, or user safety
  narratives in logs, metrics, traces, analytics, or support exports
- completed bundle downloads stay encrypted and no-store
- no browser decryption claim unless [browser-decryption.md](browser-decryption.md)
  has been satisfied for the deployment

`GET /v1/incidents/{incident_id}/chunks` and
`GET /v1/incidents/{incident_id}/chunks/{media_type}/{chunk_index}` are not part
of this public web-client boundary. They are private/dev metadata and legacy
chunk reads, not the preview browser review path.

### Public Viewer Routes

Current token viewer routes remain their own public-shaped route group:

- `GET /i/{token}`
- `GET /i/{token}/data`
- `GET /i/{token}/streams/{stream_id}/download`
- `GET /i/{token}/incident/download`
- legacy `/e/{token}` aliases
- token-neutral `/static/...`

The web-client viewer replacement route/link decision is documented in
[web-client-viewer-routing.md](web-client-viewer-routing.md). Future canonical
no-account viewer links should point at the web-client origin using:

```text
https://<web-client-origin>/viewer#token=<raw-viewer-token>
```

Current `/i/{token}` server-rendered pages and `/e/{token}` aliases are
prototype/local compatibility routes until a later runtime issue removes or
gates them. Do not route token viewer behavior through the public web client by
assumption. If a public web-client origin receives a viewer token, the token
remains a bearer secret and must not be sent to analytics, referrers, map
providers, logs, public issue text, unrelated origins, or third-party widgets.

## Route Groups That Must Remain Private

These route groups must not be exposed from a public web-client origin:

- `/admin`
- `/admin/...`
- `/admin/static/...` from public entry points
- `/v1/admin/...`
- private admin bootstrap, account creation, session revocation, second-factor
  recovery reset, unowned incident reassignment, and admin-global deletion
  routes
- operator diagnostics, readiness, migration, backup, restore, support,
  escrow, break-glass, raw-key, decryption, or private maintenance routes
- public wildcard routing to the main API
- provider dashboards, object-store consoles, database consoles, Valkey/Redis
  consoles, SMTP consoles, or deployment control planes

Private placement does not replace authentication. Admin routes remain
authenticated and authorized even on private networks.

## Browser Cookie And CSRF Configuration

Production web-client deployments should use:

```toml
[web_auth]
enabled = true
allowed_origins = ["https://app.example.invalid"]
session_cookie_name = "__Host-proofline_session"
session_cookie_secure = true
session_cookie_same_site = "Lax"
```

Requirements:

- serve both the web-client origin and the API over HTTPS
- use `Secure`, `HttpOnly`, `Path=/`, no `Domain`, and a `__Host-` cookie name
  for production cookies
- set `SAFE_WEB_ALLOWED_ORIGINS` to exact origins only
- never use wildcard credentialed CORS
- keep the CSRF header name controlled by server configuration
- fetch a fresh CSRF token after browser login and before unsafe requests
- treat CSRF tokens as token-like values in logs and support workflows
- do not store bearer session tokens in localStorage for production browser use
- reject ambiguous bearer plus cookie credential use

Local development may use a non-`Secure` non-`__Host-` cookie only for local
plain-HTTP origins such as `http://127.0.0.1:5173`.

## Cache, Header, TLS, And Edge Requirements

The Go app sets security headers for its own responses, but it does not by
itself create a complete public deployment. A reviewed public web-client
deployment needs both application and edge controls.

Required expectations:

- TLS at the public HTTPS edge
- HSTS at the HTTPS reverse proxy only after TLS is reliable for the hostname
- `Cache-Control: no-store` for token-protected, authenticated, decrypt-adjacent,
  upload, download, and error responses that may contain sensitive metadata
- `Referrer-Policy: no-referrer` for token, auth, and decrypt-adjacent
  responses
- strict CSP for the web-client static origin and for any server-rendered
  browser-facing pages
- `X-Content-Type-Options: nosniff`
- frame protection through CSP `frame-ancestors 'none'` or equivalent
- restrictive `Permissions-Policy` unless a route deliberately needs a browser
  permission
- no analytics or third-party scripts on token, decrypt, upload, or incident
  review pages unless separately security-reviewed
- map links must not receive viewer tokens, session tokens, incident IDs,
  wrapped-key IDs, object keys, or private deployment details

The web-client static host should publish non-sensitive build provenance, asset
hashes, and post-deploy served-byte verification before a v1 preview claim. For
browser decryption, follow the stronger static/signed gate in
[browser-decryption.md](browser-decryption.md).

## Logging And Observability Review

Before public preview use, review Go app logs, reverse-proxy logs, edge logs,
WAF/rate-limit logs, metrics, traces, analytics, crash reports, support exports,
object-store logs, database logs, Valkey/Redis logs, SMTP logs, and web-client
client-side telemetry.

They must not include:

- raw viewer, incident, session, verification, CSRF, bearer, or idempotency
  tokens
- token-bearing `/i/{token}` or legacy `/e/{token}` paths
- Authorization headers or cookies
- request bodies
- uploaded bytes
- plaintext
- raw keys, raw media keys, ML-KEM shared secrets, derived KEKs, or recipient
  private keys
- wrapped-key ciphertext
- private deployment details
- stored paths, staging paths, object keys, bucket URLs, database DSNs, or
  private endpoint names
- full GPS/location context, speed, heading, route history, notes, or user
  safety narratives
- SMTP credentials, email verification token values, second-factor challenge
  code values, TOTP code values, TOTP seeds, `otpauth_url` values, WebAuthn
  challenge values, client data JSON, credential bytes, recipient email
  addresses, or raw provider errors that quote private endpoints

Use route classes, safe counts, controlled status fields, and low-cardinality
error categories instead.

## Relationship To #223 And #233

Issue #223 remains the viewer-routing decision. It should decide whether
`/i/{token}` redirects to the web client, remains a local/dev fallback, or is
removed after the web-client viewer exists. This document does not implement
that routing change.

Issue #233 established the browser-decryption trust gate. A dynamic same-origin
decrypting viewer is not acceptable as the production trusted-contact decrypt
path by itself. This document relies on that gate and does not implement browser
decryption.

## Validation Expectations For Future Implementation

Future implementation should include tests for:

- exact allowed-origin CORS allow/deny behavior
- credentialed CORS never using wildcard origins
- CSRF enforcement for unsafe cookie-authenticated requests
- bearer-authenticated requests not requiring CSRF
- ambiguous bearer plus cookie credentials rejected
- private `/admin`, `/admin/...`, and `/v1/admin/...` routes unreachable from
  public web entry points
- no-store and no-referrer headers on token, auth, upload, download, and error
  responses
- public/private listener separation
- route-class rate-limit coverage for every exposed route
- token, Authorization, idempotency-key, request-body, uploaded-byte,
  wrapped-key, stored-path, object-key, and location redaction in logs
- multipart upload preflight and body-size behavior from the web-client origin
- public web-client asset integrity or served-byte verification for preview
  claims

For this design-only milestone, validation is `git diff --check` and manual
documentation review. Go tests, `go vet`, browser tests, and deployment smoke
tests are not required unless a later task changes code, configuration,
routes, or runtime behavior.

## Non-Goals

- No web-client implementation in this repository.
- No React, Node, npm, mobile-client, or protocol-repository work.
- No route, schema, handler, CORS, CSRF, cookie, or header runtime change.
- No runtime removal, redirect, or replacement of current built-in viewer
  routes.
- No browser decryption, browser recording, trusted-contact UX, backend
  decryption, server escrow, break-glass, or playable export.
- No notification delivery, emergency-services integration, public admin
  dashboard, billing, or provider-specific deployment automation.
- No claim that the Go app alone provides a complete public deployment.
