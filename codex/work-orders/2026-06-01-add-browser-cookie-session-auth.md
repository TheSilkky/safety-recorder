# One-off Codex Work Order: Add Browser Cookie Session Auth

Historical/reference-only prompt after completion.

This is a one-off implementation work order for `open-proofline/server`.

## Goal

Add production-shaped browser session support for `open-proofline/web-client` without replacing the existing bearer-token API flow used by CLI/simulator/API clients.

The backend should support a browser-safe authentication mode where the web client can log in, receive an HttpOnly session cookie, reload the page without losing authentication, call authenticated `/v1` routes with `credentials: "include"`, and log out cleanly.

This work must preserve current bearer-token sessions, owner/admin authorization, private admin dashboard boundaries, public viewer behavior, ciphertext-only backend behavior, and existing local development flows.

## Source of truth

Before editing, read the current versions of:

* `AGENTS.md`
* `README.md`
* `SECURITY.md`
* `CHANGELOG.md`
* `docs/README.md`
* `docs/api.md`
* `docs/architecture.md`
* `docs/configuration.md`
* `docs/deployment.md`
* `docs/security-model.md`
* `docs/threat-model.md`
* `docs/v1-access-control.md`
* `docs/code-map.md`
* `docs/development.md`
* `docs/codex-change-control.md`
* `codex/README.md`
* `codex/prompts/00-project-context-check.md`
* `codex/prompts/05-codex-change-control.md`
* `codex/prompts/20-code-review.md`
* `codex/prompts/30-security-review.md`
* `codex/prompts/40-documentation-update.md`
* `codex/prompts/50-mdn-web-security-header-review.md`
* `codex/prompts/70-work-on-github-issue.md`
* `codex/prompts/75-create-draft-pr-from-current-branch.md`
* `codex/prompts/76-request-codex-pr-review.md`

Relevant code areas likely include:

* `cmd/api`
* `internal/config`
* `internal/auth`
* `internal/httpapi`
* `internal/incidents`
* `internal/postgresdb`
* SQLite migrations, if schema changes are required
* PostgreSQL migrations, if schema changes are required
* existing account/session tests
* existing auth/session routes
* existing admin dashboard cookie/session code
* existing request logging and recovery middleware

Do not rely on stale assumptions from this prompt if current repository files disagree.

## Required prompt stack

Before implementation, apply:

* `codex/prompts/00-project-context-check.md`
* `codex/prompts/05-codex-change-control.md`

After implementation, apply:

* `codex/prompts/20-code-review.md`
* `codex/prompts/30-security-review.md`
* `codex/prompts/50-mdn-web-security-header-review.md`
* `codex/prompts/40-documentation-update.md`
* `codex/prompts/75-create-draft-pr-from-current-branch.md`
* `codex/prompts/76-request-codex-pr-review.md`

## Scope

Allowed:

* Add browser cookie-session login/logout support for the main `/v1` API.
* Add CSRF protection for cookie-authenticated unsafe requests.
* Add credentialed CORS support for configured web-client origins.
* Add configuration for browser session cookies, allowed web origins, and CSRF behavior.
* Update API, configuration, deployment, security, threat model, and code-map docs.
* Add SQLite/PostgreSQL schema changes only if needed for CSRF/session state.
* Add tests for cookie auth, CSRF, CORS, logout, expiry, and bearer compatibility.

Not allowed:

* Do not remove existing bearer-token auth.
* Do not replace CLI/simulator auth flows.
* Do not expose `/admin` publicly.
* Do not make `/v1/admin/...` public-ready.
* Do not move admin/operator routes onto a public surface.
* Do not add OAuth.
* Do not add JWT.
* Do not add email/password reset.
* Do not add payment-gated registration in this work order.
* Do not add browser decryption.
* Do not add backend decryption.
* Do not add trusted-contact decryption.
* Do not add raw server-held media keys.
* Do not add key escrow.
* Do not add break-glass access.
* Do not add playable media export.
* Do not add recording/capture behavior.
* Do not add emergency-services integration.
* Do not add push/SMS/Messenger notifications.
* Do not add cloud-provider deployment automation.

## Target behavior

Browser auth should support this flow:

1. Web client submits username/password to a browser-specific login route.
2. Server validates credentials using the existing account/password system.
3. Server creates a normal server-side session.
4. Server stores only the hashed session token in metadata, as current session flows do.
5. Server sets an HttpOnly session cookie.
6. Server returns safe account/session metadata, but does not return the raw session token in the JSON body for browser-cookie login.
7. Web client calls `GET /v1/account` with credentials included.
8. Server authenticates from the session cookie when no bearer token is present.
9. Web client can reload and recover auth state through `GET /v1/account`.
10. State-changing cookie-authenticated requests require CSRF protection.
11. Logout revokes the server-side session and clears the browser cookie.
12. Existing bearer-token login and bearer-authenticated API clients continue working.

## Route shape

Prefer a distinct browser-auth route family rather than changing the existing bearer login contract.

Default proposed routes:

* `POST /v1/auth/web/login`
* `POST /v1/auth/web/logout`
* `GET /v1/auth/web/csrf`

Existing routes should continue to work:

* `POST /v1/auth/login` returns bearer session token for non-browser clients.
* `POST /v1/auth/logout` continues to support bearer logout.
* `GET /v1/account` should authenticate with bearer auth or browser cookie auth.

If the current API docs or route conventions suggest better names, choose names consistent with the repository and document the decision.

## Cookie requirements

Use a dedicated browser session cookie.

Preferred production cookie shape:

* Name: `__Host-proofline_session`
* `HttpOnly`
* `Secure`
* `SameSite=Lax` or `SameSite=Strict`
* `Path=/`
* no `Domain` attribute
* expiry aligned with server-side session expiry

Local development may need a non-`Secure` cookie when running on plain `http://127.0.0.1`. If so:

* make the dev behavior explicit
* document that production deployments must use secure cookies over HTTPS
* avoid silently allowing insecure cookies for public origins
* include tests for config parsing and cookie attributes where practical

Suggested configuration names, adjust to match repository style:

* `SAFE_WEB_AUTH_ENABLED`
* `SAFE_WEB_ALLOWED_ORIGINS`
* `SAFE_WEB_SESSION_COOKIE_NAME`
* `SAFE_WEB_SESSION_COOKIE_SECURE`
* `SAFE_WEB_SESSION_COOKIE_SAMESITE`
* `SAFE_WEB_CSRF_COOKIE_NAME`
* `SAFE_WEB_CSRF_HEADER_NAME`

Defaults should preserve local development and existing non-browser clients.

## CORS requirements

Credentialed browser requests need explicit allowed origins.

Implement CORS for browser API use only when configured.

Requirements:

* no wildcard `Access-Control-Allow-Origin` when credentials are allowed
* exact allowed-origin matching
* `Access-Control-Allow-Credentials: true` only for allowed origins
* correct handling of preflight requests
* `Vary: Origin` where responses depend on origin
* allowed methods and headers limited to what the API requires
* `Authorization` remains supported for bearer clients
* CSRF header is allowed for browser unsafe requests
* disallowed origins do not receive credentialed CORS approval

Do not treat CORS as authentication.

## CSRF requirements

Cookie-authenticated unsafe requests must require CSRF protection.

Unsafe methods include at least:

* `POST`
* `PUT`
* `PATCH`
* `DELETE`

Safe methods such as `GET`, `HEAD`, and `OPTIONS` should not require CSRF.

Acceptable approaches:

1. server-stored synchronizer token bound to the session
2. signed double-submit token bound to session-specific data with HMAC

Do not implement naive double-submit cookie validation.

Preferred API shape:

* `GET /v1/auth/web/csrf` returns a CSRF token for the active cookie session.
* The web client sends that token in a custom header such as `X-CSRF-Token`.
* The server validates the token for unsafe cookie-authenticated requests.
* Bearer-authenticated requests do not require CSRF unless current docs or security review require otherwise.

The CSRF token must not be logged.

CSRF failure responses must be safe and must not reveal raw session tokens, CSRF token values, request bodies, private deployment details, or internal validation details.

Consider adding Fetch Metadata header checks as defense-in-depth, but do not rely on them as the only CSRF control unless this is explicitly designed and documented.

## Auth middleware behavior

Preserve the existing bearer-auth path.

Suggested behavior:

* If `Authorization: Bearer ...` is present, use the existing bearer session path.
* If no bearer token is present, try browser session cookie auth.
* If both bearer token and browser cookie are present, choose one deterministic behavior and document it. Prefer rejecting ambiguous mixed credentials unless existing patterns strongly favor bearer precedence.
* Return the same safe unauthenticated/forbidden shapes as current API routes.
* Do not log raw bearer tokens or cookie values.
* Do not expose raw session token hashes.
* Do not include cookie values in error responses.

## Admin/API boundary

This work must not make admin/operator routes public-ready.

Existing `/v1/admin/...` JSON routes remain authenticated admin-only routes and must still be blocked from public edges.

The private `/admin` dashboard listener remains separate and private.

Do not route `/admin` from public edges.

Do not add public admin dashboards.

If browser cookie sessions can authenticate admin accounts on main `/v1`, document that public reverse proxies must still block `/v1/admin/...` unless a future audited public-admin API is explicitly designed.

## Data model

Reuse existing session storage where possible.

If CSRF state requires schema changes:

* add SQLite migration
* add PostgreSQL migration
* preserve migration ordering
* add repository methods through existing metadata boundaries
* test SQLite/PostgreSQL parity
* avoid storing raw session tokens
* avoid storing raw CSRF tokens unless the design explicitly justifies it
* prefer hashes or HMAC/session-bound tokens

Do not store browser cookies directly in metadata.

## Logging and sensitive data

Never include these in logs, docs, tests, PR descriptions, issue comments, or error responses:

* raw session tokens
* cookie values
* CSRF token values
* Authorization headers
* request bodies
* uploaded bytes
* plaintext
* raw keys
* raw media keys
* contact private keys
* wrapped-key ciphertext
* object-store credentials
* stored paths
* object keys
* private deployment details
* user safety data

Add regression tests for safe logging/error behavior where practical.

## Tests

Add focused tests for:

* browser login sets a session cookie
* browser login response does not include a raw session token
* cookie has expected attributes in production-style config
* local development cookie behavior is explicit and tested
* `GET /v1/account` works with browser cookie auth
* `GET /v1/account` still works with bearer auth
* bearer login still returns token as before
* bearer clients still authenticate without cookies
* logout revokes the session and clears the cookie
* expired cookie session fails closed
* revoked cookie session fails closed
* unsafe cookie-authenticated request without CSRF token fails
* unsafe cookie-authenticated request with invalid CSRF token fails
* unsafe cookie-authenticated request with valid CSRF token succeeds
* safe cookie-authenticated `GET` request does not require CSRF
* bearer-authenticated unsafe request behavior remains unchanged
* credentialed CORS accepts configured origins
* credentialed CORS rejects unconfigured origins
* no wildcard origin is sent with credentials
* mixed bearer/cookie behavior is deterministic and documented
* `/admin` private dashboard listener remains private
* public viewer behavior remains unchanged
* public viewer tokens remain unrelated to account session cookies

If PostgreSQL session/auth tests are available or a `SAFE_POSTGRES_TEST_DSN` is configured, add or run PostgreSQL parity coverage.

## Documentation updates

Update relevant docs:

* `README.md`
* `docs/api.md`
* `docs/configuration.md`
* `docs/deployment.md`
* `docs/security-model.md`
* `docs/threat-model.md`
* `docs/v1-access-control.md`
* `docs/code-map.md`
* `docs/development.md`, if validation guidance changes
* `SECURITY.md`, if browser cookie handling changes reporting guidance
* `CHANGELOG.md`

Docs must clearly state:

* browser cookie sessions are for the web client
* bearer sessions remain supported for CLI/simulator/API clients
* public deployment still requires TLS and edge hardening
* cookie auth does not make `/v1/admin/...` public-ready
* production must use secure cookie settings
* credentialed CORS must use explicit allowed origins
* browser token persistence should not use localStorage in production
* CSRF applies to cookie-authenticated unsafe requests
* this does not add paid registration, password reset, browser decryption, backend decryption, key escrow, playable export, or emergency-services integration

## Web-client coordination notes

Do not edit `open-proofline/web-client` in this server work order unless explicitly requested.

However, document the expected future web-client follow-up:

* add cookie-session mode
* use `fetch(..., { credentials: "include" })`
* call `GET /v1/account` on app boot
* call the CSRF route before unsafe cookie-authenticated requests
* attach `X-CSRF-Token` for unsafe methods
* clear UI session state on `401`/`403`
* stop requiring raw bearer token storage in browser cookie mode
* add reload persistence and logout tests

## Suggested implementation phases

Prefer small commits in this order:

1. Config and docs skeleton for browser cookie auth.
2. Cookie helpers and route wiring.
3. Browser login/logout routes.
4. Cookie auth path for existing account/session middleware.
5. CSRF token creation/validation.
6. Credentialed CORS for configured web origins.
7. Tests.
8. Docs and changelog.

If this becomes too large for one reviewable PR, stop after the design/config/docs phase and create follow-up issues for implementation. Do not push a giant risky auth PR just to complete the work order.

## Validation

For Go changes, run from the server repo:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

If PostgreSQL tests are relevant and a test DSN is available:

```bash
SAFE_POSTGRES_TEST_DSN='<test dsn>' go test ./... -count=1
```

Because this touches authentication, browser API behavior, CORS/CSRF, and deployment-sensitive configuration, also run the relevant local Compose smoke tests if available.

At minimum, inspect Compose smoke docs and run the full smoke path when Docker is available.

Do not run smoke tests against production services.

Do not use real deployment credentials.

## Pull request

Open a PR against `develop`.

Suggested PR title:

```text
auth: add browser cookie session support
```

PR body must include:

```md
## Summary

- Added browser cookie-session login/logout support for the web client.
- Preserved existing bearer-token sessions for CLI/simulator/API clients.
- Added CSRF protection for cookie-authenticated unsafe requests.
- Added explicit credentialed CORS handling for configured web origins.
- Updated API, configuration, security, threat model, deployment, and code-map docs.

## Validation

- [ ] `gofmt -w ./cmd ./internal ./migrations`
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `git diff --check`
- [ ] PostgreSQL parity tests, if relevant
- [ ] Compose smoke tests, if available/relevant

## Security and scope

- Existing bearer-token auth remains supported.
- Browser login does not return the raw session token in JSON.
- Browser session cookie is HttpOnly.
- Production cookie settings require Secure/SameSite/path/host-only behavior.
- Cookie-authenticated unsafe requests require CSRF protection.
- Credentialed CORS allows only configured origins.
- `/admin` remains private-only.
- `/v1/admin/...` remains authenticated admin-only and is not public-ready.
- Public viewer behavior is unchanged.
- No backend decryption, browser decryption, raw server-held media keys, key escrow, break-glass access, playable media export, recording/capture behavior, OAuth/JWT, notification delivery, payment registration, or emergency-services integration was added.
- No raw tokens, cookie values, CSRF tokens, Authorization headers, request bodies, uploaded bytes, plaintext, raw keys, wrapped-key ciphertext, stored paths, object keys, private deployment details, or user safety data are logged or exposed.
```

## Final output

Report:

1. files changed
2. routes added
3. config added
4. cookie attributes implemented
5. CSRF approach chosen
6. CORS behavior
7. bearer-auth compatibility status
8. admin/private boundary status
9. tests added
10. validation commands run
11. docs updated
12. follow-up needed in `open-proofline/web-client`
