# API

This is the current backend-only HTTP surface for Proofline. The API binary starts a main API/viewer listener and a private-admin listener on one or more configured bind addresses. Main `/v1` routes require local account authentication except for login and the disabled-by-default registration/email-verification routes, and they use app-level route-class rate limits. Existing `/v1/admin/...` JSON routes require an admin account and are mounted only on the private-admin listener. The private-admin listener also serves the `/admin` dashboard route tree. Incident viewer routes are token-gated, read-only, and mounted on the main listener. The future canonical no-account viewer link belongs to the web-client origin as documented in [web-client-viewer-routing.md](web-client-viewer-routing.md); planned web, iOS, and Android clients are not part of this repository yet.

Media bundle downloads are encrypted chunk bundles. The backend does not
decrypt, merge, or produce playable media. Current encrypted uploads use the
accepted post-quantum payload envelope documented in
[encryption.md](encryption.md) and [post-quantum-envelope.md](post-quantum-envelope.md);
the API validates public envelope metadata but still treats payload bytes as
opaque ciphertext.

The current API stores incidents owned by local accounts. Incidents are generic
by default and may include optional `incident_mode`, `capture_profile`,
`escalation_policy`, and `sharing_state` metadata on the main create/read
routes. Those fields are metadata only: they do not grant access, send
notifications, change key custody, expose trusted-contact workflows, or change
public viewer and bundle behavior. Mode-specific retention behavior is not
implemented; deletion and closed-incident retention enforcement are documented
in [retention-backup-deletion.md](retention-backup-deletion.md). Planned
mode-driven behavior is documented in [incident-modes.md](incident-modes.md).
Authenticated account-owner routes can store account/device recipient public-key
metadata, trusted-contact relationship lifecycle metadata, trusted-contact
public-key metadata, owner-scoped sharing-grant records, and wrapped
CEK/media-key metadata for active trusted-contact grants. Trusted-contact
relationship records do not grant keys, plaintext, notifications, emergency
dispatch, or viewer-token privileges by themselves. Authenticated
trusted-contact wrapped-key reads require an accepted relationship, a
recipient-bound active contact public key, an active unexpired ciphertext grant,
and an active wrapped-key record. Account/device wrapped-key delivery, browser
or backend decryption, notifications, raw key storage, and key escrow do not
exist yet. The main API does include a narrow public-safe owner incident
list/detail read surface for the future web client, but this does not make
every `/v1` route group public-ready without route-level deployment review.
The public web-client route, CORS, CSRF, cookie, cache, edge, and logging
boundary is documented in
[public-web-client-deployment-boundary.md](public-web-client-deployment-boundary.md).

Future capture stream groups, stream variant roles, source-timeline identity,
and evidence supersession are planning-only in
[capture-stream-variants.md](capture-stream-variants.md). Future full-fidelity
GPS, speed, heading, and location freshness context is planning-only in
[encrypted-location-context.md](encrypted-location-context.md). The current API
still treats each media stream as one concrete upload lane and does not select
canonical evidence across variants or implement encrypted location sidecars.

Default bind addresses:

- main API and incident viewer listener: `127.0.0.1:8080`
- private admin dashboard listener: `127.0.0.1:8081`

Use `SAFE_MAIN_BIND_ADDRS` and `SAFE_ADMIN_BIND_ADDRS` for comma-separated
bind-address lists. Legacy `SAFE_PRIVATE_BIND_ADDRS` still maps to the main
listener, but legacy `SAFE_PUBLIC_BIND_ADDRS` now fails startup so an old
public viewer bind cannot become the private-admin listener by accident.

## Common Responses

Errors use:

```json
{
  "error": {
    "code": "invalid_json",
    "message": "request body must be valid JSON"
  }
}
```

Non-upload JSON bodies are limited to 64 KiB. Upload file bytes are limited by `SAFE_MAX_UPLOAD_BYTES`; multipart metadata has a small fixed overhead allowance. Accepted encrypted chunk bytes are also limited by the account-scoped committed blob quota configured with `SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES`, which defaults to 10 GB per owner account. Local temp-upload staging bytes are limited by `SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES`, which defaults to 1 GB and applies to regular `upload-*` staging files before local or S3-compatible final commit. These byte settings accept a positive byte count or binary unit suffixes `B`, `K`/`KB`, `M`/`MB`, and `G`/`GB`. Fractional unit values are allowed when they resolve to at least one byte. Non-positive, sub-byte, invalid, and oversized values are rejected during startup.

Main API route classes are rate limited by default before authentication using
safe server-controlled keys based on route class and a hash of the socket peer
identity. Login/logout, public registration, registration email verification,
email second-factor challenge routes, TOTP setup/verification routes, and
WebAuthn register/verify routes have separate authentication-related route
classes. Existing main API limit classes
also cover account metadata, account/device recipient-key metadata, contact-key
metadata, trusted-contact relationship metadata, incident metadata reads and
writes, sharing-grant metadata, wrapped-key metadata, uploads, reconciliation,
streams, incident tokens, and private encrypted downloads. Rate-limit keys do
not include raw email addresses, raw usernames, verification tokens,
second-factor challenge codes, TOTP codes, TOTP seeds, WebAuthn challenge or
client-data values, raw session tokens, Authorization headers, raw idempotency
keys, request bodies, uploaded bytes,
incident IDs, stored paths, object keys, plaintext, raw keys, wrapped-key
ciphertext, or private deployment details. Exhausted limits return
`429 rate_limited` with `Retry-After`. A configured coordination limiter
failure returns `503 rate_limit_unavailable` with a generic response. See
[configuration](configuration.md) for `SAFE_MAIN_API_RATE_LIMIT_*` settings.

## Health And Readiness

The current listener split does not mount `/v1/health/live` or
`/v1/health/ready` on either listener. The private-admin listener is an
admin-only `/v1/admin/...` and `/admin` surface, and the main listener must not publish
operator readiness details on the same origin as future public product API
routes. Local and CI smoke checks use token-neutral static assets plus the
admin bootstrap/login flow to prove both listener trees are serving.

## Authentication And Accounts

Private `/v1` routes require a bearer session token by default:

```http
Authorization: Bearer <session_token>
```

Session tokens are opaque server-side credentials. The raw token is returned only by login, while the metadata backend stores only its SHA-256 hash. Sessions expire after `SAFE_SESSION_TTL`, defaulting to `12h`, and can be revoked by logout, password reset, or the admin session-revocation route.

When `SAFE_WEB_AUTH_ENABLED=true`, the main API also accepts a dedicated
browser session cookie for `/v1` routes when no bearer token is present. Browser
cookie mode is intended for the future `open-proofline/web-client`: web clients
should call with `credentials: "include"` and should not store raw bearer
tokens in localStorage in production. If a request sends both
`Authorization: Bearer ...` and the browser session cookie, the server rejects
it with `400 ambiguous_credentials`.

Cookie-authenticated unsafe requests such as `POST` and `PATCH` require a
session-bound CSRF token in the configured header, defaulting to
`X-CSRF-Token`. Bearer-authenticated requests keep their existing behavior and
do not require this CSRF header. Credentialed CORS is sent only for exact
origins configured with `SAFE_WEB_ALLOWED_ORIGINS`; CORS is not authentication.

On startup, the server fails closed unless an admin account already exists or a
bootstrap secret is configured. With that secret configured, create the first
admin through the private `/admin` bootstrap screen or by posting form fields
to `POST /admin/bootstrap`, then remove the bootstrap secret from TOML, the
environment, or the secret mount and restart or redeploy without it. JSON
`POST /v1/bootstrap/admin` is not mounted on either listener.

Public account registration is controlled by
`SAFE_ACCOUNT_REGISTRATION_MODE`, defaulting to `disabled`. Supported modes are:

| Mode | Public registration behavior |
|---|---|
| `disabled` | `POST /v1/auth/register` returns `403 registration_disabled`. Existing accounts and sessions continue to work. |
| `admin_only` | Public registration returns `403 registration_disabled`; admins can still create accounts through existing admin-only routes. |
| `open` | Public self-registration accepts a username, email, and password, creates a pending account, and sends an email verification link before login is allowed. |
| `paid` | Reserved for future hosted-service billing. Registration returns `503 registration_payment_unavailable` and does not create an active account. |

Open registration requires an email backend and public web origin at startup.
Verification links use:

```text
{SAFE_PUBLIC_WEB_ORIGIN}/verify-email#token=<raw-token>
```

The token is placed in the URL fragment so a future web client can submit it in
the JSON request body without sending it to the web server as a path or query
value. The backend stores only token hashes, not raw verification tokens.

Accounts also carry a factor-neutral `second_factor_setup_state`:

| State | Meaning |
|---|---|
| `not_required` | Product-route access is not blocked by second-factor setup. Existing migrated accounts default to this state for preview compatibility. |
| `setup_required` | Primary login can create bearer or browser-cookie sessions, but main product routes fail closed until setup is completed by email challenge, TOTP, or WebAuthn second-factor verification. |
| `complete` | Required second-factor setup is complete for product-route access. |

New accounts created through the private admin API, the `/admin` bootstrap
surface, or open registration start in `setup_required`. Email verification
only moves `account_state` from `pending_email_verification` to `active`; it
does not complete second-factor setup. While setup is required, authenticated
clients can inspect `GET /v1/account`, obtain browser CSRF metadata, request or
verify email/TOTP/WebAuthn second-factor setup, and log out. Other main product routes
return:

```json
{
  "error": {
    "code": "second_factor_setup_required",
    "message": "second factor setup is required before account access"
  }
}
```

Email challenge, TOTP, and disabled-by-default WebAuthn/FIDO2 are the
implemented second-factor setup methods. Accounts with active TOTP or WebAuthn
factors also require per-session second-factor verification after primary login
before product routes are available. Until that challenge is satisfied,
authenticated sessions can inspect `GET /v1/account`, obtain browser CSRF
metadata, verify the active TOTP or WebAuthn challenge, and log out. Other main
product routes return:

```json
{
  "error": {
    "code": "second_factor_verification_required",
    "message": "second factor verification is required before account access"
  }
}
```

Lost-factor recovery is private-admin assisted only. There is no self-service
recovery code, password recovery, email reset link, public recovery portal, or
factor bypass. A private-admin reset can remove enrolled email, TOTP, and
WebAuthn factors for a local account, mark second-factor setup required again,
revoke that account's active sessions, and write controlled audit metadata.
It does not alter account/device recipient keys, trusted-contact keys,
sharing grants, wrapped CEK/media-key records, incidents, blobs, key custody,
or decryption behavior. Lost account/device recipient keys remain managed by
the existing account recipient-key lost/revoke/replace routes after the account
is recovered. WebAuthn routes remain unavailable with
`503 webauthn_unavailable` until `[webauthn]` is explicitly enabled with an RP
ID and exact allowed origins.

### `POST /v1/account/second-factor/email/challenge`

Authenticated setup route for starting email second-factor setup after primary
login. Setup-incomplete bearer and browser-cookie sessions may call this route;
cookie-authenticated requests still require the configured CSRF header.

Request:

```json
{
  "email": "user@example.com"
}
```

If `email` is omitted, the server uses the account's already verified
registration email address when one exists. The registration email verification
record itself is not a second factor. Admin-created accounts without a verified
account email must provide an email address here. The server validates and
normalizes the address, creates or refreshes a pending `email_challenge`
second-factor record, stores only a challenge-code hash, and sends the raw code
only through the configured email sender. The response never includes the raw
code or destination email address.

Response `202`:

```json
{
  "status": "challenge_sent",
  "message": "If the email challenge can be completed, a verification email will be sent.",
  "expires_at": "2026-06-10T12:10:00Z"
}
```

Challenge codes use `SAFE_SECOND_FACTOR_EMAIL_CHALLENGE_TTL`, defaulting to
`10m`. The request and verification routes use the
`SAFE_MAIN_API_RATE_LIMIT_AUTH_EMAIL_VERIFY` route class. Email delivery
failures return `503 email_unavailable`; invalid or missing email input returns
`400 email_required` or `400 invalid_email`; an account that already has an
active email factor returns `409 second_factor_already_configured`.

### `POST /v1/account/second-factor/email/verify`

Authenticated setup route for consuming an email second-factor challenge code.
The code is account-bound, single-use, expires, and is looked up by hash.

Request:

```json
{
  "code": "challenge-code-from-email"
}
```

`token` is accepted as a compatibility alias for `code`. Successful verification
marks the email factor active, consumes other pending challenges for that
factor, and sets the account `second_factor_setup_state` to `complete`.

Response `200`:

```json
{
  "status": "verified",
  "second_factor": {
    "id": "sf_...",
    "factor_type": "email_challenge",
    "state": "active",
    "verified_at": "2026-06-10T12:04:00Z"
  },
  "account": {
    "id": "acct_...",
    "username": "user",
    "account_state": "active",
    "second_factor_setup_state": "complete",
    "second_factor_setup_required": false,
    "role": "user",
    "created_at": "2026-06-10T11:00:00Z",
    "updated_at": "2026-06-10T12:04:00Z",
    "password_changed_at": "2026-06-10T11:00:00Z"
  }
}
```

Expired, reused, wrong-account, or invalid codes all return the same
`400 second_factor_challenge_invalid` response. Challenge codes, token hashes,
destination email addresses, request bodies, session tokens, CSRF tokens, and
other credential material must not be logged.

### `POST /v1/account/second-factor/totp/enroll`

Authenticated setup route for starting TOTP authenticator-app enrollment after
primary login. Setup-incomplete sessions may call this route; cookie
authenticated requests still require the configured CSRF header. Calling enroll
again while a TOTP factor is still pending refreshes the pending seed and
invalidates setup codes from the older seed. Accounts with an active TOTP
factor receive `409 second_factor_already_configured`.

Request:

```json
{}
```

Response `201`:

```json
{
  "id": "sf_...",
  "factor_type": "totp",
  "state": "pending",
  "secret": "BASE32SECRET...",
  "otpauth_url": "otpauth://totp/Proofline:user?secret=BASE32SECRET...",
  "issuer": "Proofline",
  "account_name": "user",
  "period_seconds": 30,
  "digits": 6,
  "algorithm": "SHA1"
}
```

The `secret` and `otpauth_url` are setup credentials. They are returned only in
this enrollment response and must not be logged, stored in browser storage,
placed in URLs outside the `otpauth_url`, pasted into support artifacts, or
included in public issues.

### `POST /v1/account/second-factor/totp/confirm`

Authenticated setup route for confirming a pending TOTP enrollment. The server
accepts six-digit SHA-1 TOTP codes with a 30-second step and one adjacent step
of clock skew on either side. Successful confirmation marks the factor active,
records the used time step to reject replay, sets the account
`second_factor_setup_state` to `complete`, and marks the current session as
second-factor verified.

Request:

```json
{
  "code": "123456"
}
```

Response `200`:

```json
{
  "status": "verified",
  "second_factor": {
    "id": "sf_...",
    "factor_type": "totp",
    "state": "active",
    "verified_at": "2026-06-10T12:04:00Z"
  },
  "account": {
    "id": "acct_...",
    "username": "user",
    "account_state": "active",
    "second_factor_setup_state": "complete",
    "second_factor_setup_required": false,
    "role": "user",
    "created_at": "2026-06-10T11:00:00Z",
    "updated_at": "2026-06-10T12:04:00Z",
    "password_changed_at": "2026-06-10T11:00:00Z"
  },
  "session": {
    "session_id": "ses_...",
    "second_factor_verified_at": "2026-06-10T12:04:00Z",
    "second_factor_method": "totp"
  }
}
```

Invalid, stale, wrong-seed, or replayed setup codes return
`400 totp_challenge_invalid`.

### `POST /v1/account/second-factor/totp/verify`

Authenticated session challenge route for accounts that already have an active
TOTP factor. Primary login can create a bearer or browser-cookie session, but
product routes remain blocked with `403 second_factor_verification_required`
until this route verifies a fresh TOTP code. A code whose time step is equal to
or older than the last accepted TOTP step is rejected to limit replay.

Request:

```json
{
  "code": "123456"
}
```

Response `200`:

```json
{
  "status": "verified",
  "second_factor": {
    "id": "sf_...",
    "factor_type": "totp",
    "state": "active",
    "verified_at": "2026-06-10T12:04:00Z"
  },
  "session": {
    "session_id": "ses_...",
    "second_factor_verified_at": "2026-06-10T12:09:00Z",
    "second_factor_method": "totp"
  }
}
```

Invalid, replayed, missing-factor, or wrong-account codes all return
`400 totp_challenge_invalid`. TOTP codes, seeds, `otpauth_url` values, request
bodies, session tokens, CSRF tokens, and other credential material must not be
logged.

### `POST /v1/account/second-factor/webauthn/register/start`

Authenticated setup route for starting WebAuthn/FIDO2 passkey or roaming
security-key registration after primary login. Setup-incomplete bearer and
browser-cookie sessions may call this route; cookie-authenticated requests
still require the configured CSRF header. WebAuthn must be enabled with a valid
RP ID and exact allowed origins or the route returns `503 webauthn_unavailable`.

Request:

```json
{
  "authenticator_attachment": "cross-platform"
}
```

`authenticator_attachment` is optional. When present it must be `platform` or
`cross-platform`; the server uses it only to shape authenticator selection and
client hints.

Response `201`:

```json
{
  "status": "registration_challenge_created",
  "credential_creation": {
    "publicKey": {}
  },
  "expires_at": "2026-06-10T12:05:00Z",
  "authenticator_attachment": "cross-platform"
}
```

The `credential_creation.publicKey` object is produced by the WebAuthn library
and includes the challenge and RP/user options needed by browser
`navigator.credentials.create()`. Challenge session data is stored server-side,
expires according to `SAFE_WEBAUTHN_CHALLENGE_TTL`, and is consumed on finish.

### `POST /v1/account/second-factor/webauthn/register/finish`

Authenticated setup route for finishing WebAuthn registration. The request body
is the browser WebAuthn attestation response from
`navigator.credentials.create()`. Successful verification stores public
credential material, sign counter, transports, attachment and backup flags,
sets the account `second_factor_setup_state` to `complete`, and marks the
current session as WebAuthn-verified.

Response `200`:

```json
{
  "status": "verified",
  "second_factor": {
    "id": "sf_...",
    "factor_type": "webauthn",
    "attachment": "cross-platform",
    "backup_eligible": true,
    "backup_state": false,
    "user_verified": true,
    "clone_warning": false,
    "verified_at": "2026-06-10T12:04:00Z"
  },
  "account": {
    "id": "acct_...",
    "username": "user",
    "account_state": "active",
    "second_factor_setup_state": "complete",
    "second_factor_setup_required": false,
    "role": "user",
    "created_at": "2026-06-10T11:00:00Z",
    "updated_at": "2026-06-10T12:04:00Z",
    "password_changed_at": "2026-06-10T11:00:00Z"
  },
  "session": {
    "session_id": "ses_...",
    "second_factor_verified_at": "2026-06-10T12:04:00Z",
    "second_factor_method": "webauthn"
  }
}
```

Expired, reused, wrong-account, malformed, or origin/RP-invalid ceremonies all
return the same generic `400 webauthn_challenge_invalid` response. Raw
challenge values, client data JSON, credential bytes, request bodies, session
tokens, CSRF tokens, and other credential material must not be logged.

### `POST /v1/account/second-factor/webauthn/verify/start`

Authenticated session challenge route for accounts that already have an active
WebAuthn factor. Primary login can create a bearer or browser-cookie session,
but product routes remain blocked with
`403 second_factor_verification_required` until this route and the finish route
complete successfully.

Request:

```json
{}
```

Response `201`:

```json
{
  "status": "verification_challenge_created",
  "credential_assertion": {
    "publicKey": {}
  },
  "expires_at": "2026-06-10T12:10:00Z"
}
```

The `credential_assertion.publicKey` object is produced by the WebAuthn library
and includes the challenge and allowed credential IDs needed by browser
`navigator.credentials.get()`.

### `POST /v1/account/second-factor/webauthn/verify/finish`

Authenticated session challenge route for finishing WebAuthn assertion
verification. The request body is the browser WebAuthn assertion response from
`navigator.credentials.get()`. Successful verification updates credential
sign-count and backup/user-presence flags and marks the current session as
WebAuthn-verified.

Response `200`:

```json
{
  "status": "verified",
  "second_factor": {
    "id": "sf_...",
    "factor_type": "webauthn",
    "backup_eligible": true,
    "backup_state": false,
    "user_verified": true,
    "clone_warning": false,
    "verified_at": "2026-06-10T12:04:00Z",
    "last_used_at": "2026-06-10T12:09:00Z"
  },
  "session": {
    "session_id": "ses_...",
    "second_factor_verified_at": "2026-06-10T12:09:00Z",
    "second_factor_method": "webauthn"
  }
}
```

Expired, reused, wrong-account, unknown-credential, malformed, or
origin/RP-invalid ceremonies all return the same generic
`400 webauthn_challenge_invalid` response.

### `POST /v1/auth/login`

Authenticates a local account and returns a raw session token once.

Request:

```json
{
  "username": "admin",
  "password": "long local password"
}
```

Response `201`:

```json
{
  "session_id": "ses_...",
  "account": {
    "id": "acct_...",
    "username": "admin",
    "account_state": "active",
    "second_factor_setup_state": "not_required",
    "second_factor_setup_required": false,
    "role": "admin",
    "created_at": "2026-05-31T10:00:00Z",
    "updated_at": "2026-05-31T10:00:00Z",
    "password_changed_at": "2026-05-31T10:00:00Z"
  },
  "token": "...",
  "second_factor_verification_required": false,
  "created_at": "2026-05-31T10:00:00Z",
  "expires_at": "2026-05-31T22:00:00Z"
}
```

For accounts with an active TOTP or WebAuthn factor, primary login still
returns a session token, but `second_factor_verification_required` is `true`
and product routes fail closed until the matching TOTP or WebAuthn verification
route succeeds.

### `POST /v1/auth/register`

Unauthenticated public registration endpoint. It is disabled unless
`SAFE_ACCOUNT_REGISTRATION_MODE=open` or `paid`.

Request for open registration:

```json
{
  "username": "new-user",
  "email": "user@example.com",
  "password": "long local password"
}
```

In `open` mode the server validates the username, email, and password, creates
a `pending_email_verification` user account, stores a single-use
`email_verification` token hash, sends a verification email, and returns
`202`:

```json
{
  "status": "verification_required",
  "message": "If registration can be completed, a verification email will be sent."
}
```

The response does not include a session token. Duplicate username or email
registrations return the same generic `202` response and do not expose whether
the username or email already exists. Invalid fields can still return specific
validation errors such as `invalid_username`, `invalid_email`, or
`invalid_password`. A repeated registration with the same username and email
for a still-pending account may send a fresh verification email so transient
mail delivery failures can be retried without exposing the account state in the
HTTP response. Runtime mail delivery failures are logged with safe error
categories and keep the same generic `202` response shape.

In `paid` mode this route returns:

```json
{
  "error": {
    "code": "registration_payment_unavailable",
    "message": "paid registration is not available"
  }
}
```

No payment provider, checkout session, subscription state, active account, or
billing webhook is created by this placeholder mode.

### `POST /v1/auth/email/verify`

Unauthenticated email verification endpoint. The raw verification token is
accepted only in the JSON body:

```json
{
  "token": "verification-token-from-email"
}
```

The server hashes the token, looks up an unexpired and unconsumed
`email_verification` record, consumes it once, marks the email verified, and
activates the account if it was `pending_email_verification`. It does not
create a session automatically.

Success response:

```json
{
  "status": "verified"
}
```

Invalid, expired, already consumed, wrong-purpose, or missing tokens return the
same generic `400 verification_token_invalid` response.

### `POST /v1/auth/logout`

Revokes the current session.

### `POST /v1/auth/web/login`

Enabled only when `SAFE_WEB_AUTH_ENABLED=true`. Authenticates a local account,
creates a normal hashed server-side session, sets the browser session cookie,
and returns safe session metadata without returning the raw session token in
JSON.

Response metadata includes `second_factor_verification_required`,
`second_factor_verified_at`, and `second_factor_method` with the same meaning
as bearer login. For accounts with an active TOTP or WebAuthn factor,
`second_factor_verification_required` is `true` until the matching
second-factor verification route succeeds with a valid CSRF header.

The preferred production cookie is `__Host-proofline_session` with `HttpOnly`,
`Secure`, `SameSite=Lax`, `Path=/`, no `Domain` attribute, and expiry aligned
with the server-side session. Local plain-HTTP development can opt into a
non-`Secure` non-`__Host-` cookie only for local web origins.

### `GET /v1/auth/web/csrf`

Enabled only when browser cookie auth is enabled and the request is
authenticated by the browser session cookie. Returns the CSRF header name and a
session-bound CSRF token for unsafe cookie-authenticated requests.

### `POST /v1/auth/web/logout`

Enabled only when browser cookie auth is enabled. Requires the browser session
cookie and a valid CSRF header for an active session, revokes that server-side
session, and clears the browser session cookie. Existing `POST /v1/auth/logout`
continues to support bearer-token logout.

### `GET /v1/account`

Returns the authenticated account for either bearer authentication or, when
enabled, browser cookie authentication. This route remains available to
setup-incomplete accounts so clients can display account and setup state.

### `POST /v1/account/password`

Changes the authenticated account password after verifying `current_password`; other sessions for the account are revoked.

### Private Admin Web Routes

The private-admin listener serves a small admin web surface outside the
`/v1` API namespace:

- `GET /admin`
- `POST /admin/login`
- `POST /admin/bootstrap`
- `POST /admin/logout`
- `POST /admin/password`
- `POST /admin/accounts/{account_id}/password`
- `GET /admin/static/styles.css`

`GET /admin` renders either the first-admin bootstrap form, the admin login
form, or the authenticated admin dashboard. The form handlers reuse the same
local account records and opaque server-side session store as the JSON API, but
the browser flow stores the raw session token in an HttpOnly, SameSite=Strict
cookie scoped to `/admin`.

The bootstrap screen is available only when no admin account exists and
`SAFE_AUTH_BOOTSTRAP_SECRET` is configured. It requires the bootstrap secret,
admin username, and admin password. After an admin exists, `/admin` shows the
login screen and requires an admin account. Non-admin sessions are rejected.

The authenticated dashboard lists local accounts and offers password workflows.
`POST /admin/logout` revokes the current admin web session. `POST
/admin/password` changes the current admin account password after verifying the
current password, then revokes other sessions for that account. `POST
/admin/accounts/{account_id}/password` lets an admin reset another local
account password and revokes all sessions for that account. These authenticated
state-changing forms use a session-bound CSRF token.

`/admin/static/styles.css` is unauthenticated because it is token-neutral static
CSS from the AGPL-licensed source tree. It does not contain incident data,
tokens, deployment details, keys, or evidence metadata. The admin HTML pages
use `Cache-Control: no-store` and conservative browser security headers.

The admin web surface shows only route-boundary status, safe navigation stubs,
and local account-management data. It does not expose incident evidence, viewer
tokens, session tokens, password hashes, request bodies, uploaded bytes,
Authorization headers, plaintext, raw keys, stored paths, object keys, private
deployment details, or sensitive evidence metadata. It is not a public admin
dashboard and must stay on the private-admin listener.

### Admin API Routes

The following routes are mounted only on the private-admin listener and require
an admin account session:

- `GET /v1/admin/accounts`
- `POST /v1/admin/accounts`
- `POST /v1/admin/accounts/{account_id}/password`
- `POST /v1/admin/accounts/{account_id}/second-factor/recovery/reset`
- `POST /v1/admin/accounts/{account_id}/sessions/revoke`
- `GET /v1/admin/incidents/unowned`
- `GET /v1/admin/incidents/{incident_id}/deletion`
- `POST /v1/admin/incidents/{incident_id}/deletion`
- `POST /v1/admin/incidents/{incident_id}/reassignment`

`POST /v1/admin/accounts` accepts `username`, `password`, and `role`, where
`role` is `user` or `admin`. Admin password reset and explicit session
revocation revoke all sessions for the selected account.

`POST /v1/admin/accounts/{account_id}/second-factor/recovery/reset` accepts a
controlled `reason` value:

- `lost_email_access`
- `lost_totp_device`
- `lost_webauthn_credential`
- `lost_all_factors`
- `admin_created_setup`
- `operator_review`

The route removes enrolled email, TOTP, and WebAuthn factors and pending
second-factor challenges for the selected account, sets
`second_factor_setup_state` back to `setup_required`, revokes active sessions
for that account, and records an `account_recovery_events` audit row with
controlled counts. It does not accept free-text notes and does not return or
log raw challenge codes, TOTP seeds, WebAuthn challenge material, session
tokens, request bodies, key material, wrapped-key ciphertext, stored paths, or
private deployment details. It does not change password hashes, account/device
recipient keys, contact keys, sharing grants, wrapped-key records, incident
metadata, encrypted blobs, key custody, or decryption behavior.

These routes share the private-admin listener boundary with the `/admin`
dashboard but keep JSON bearer or browser-cookie session authentication and
admin-role checks. Private placement does not replace authentication, and
separate bind addresses are not a complete security model. Keep private-admin
listeners behind localhost, LAN, WireGuard, firewall rules, or a strict private
reverse proxy. Expose the main API only after deployment-specific TLS, path
routing, abuse controls, browser credential rules, CSRF decisions, logging
review, and production operations are explicitly designed and reviewed.

## Contact Public Keys

Contact public-key routes are mounted on the main API listener and require a
valid local account session. They are scoped to the authenticated account only;
admins do not use these routes to manage another account's contact keys. The
server stores public-key metadata, optional `recipient_account_id` binding,
wrapping algorithm names, fingerprints, state, and optional display labels. It
does not store contact private keys, raw CEKs, raw media keys, wrapped
CEKs/media keys, browser fragment secrets, plaintext, request bodies, uploaded
bytes, stored paths, staging paths, object keys, or private deployment details.

Contact key states are:

| State | Meaning |
|---|---|
| `pending_verification` | Registered but not eligible for new sharing grants. |
| `active` | Eligible for new sharing grants. |
| `replaced` | Superseded by another key version. |
| `revoked` | Revoked and not eligible for future grants. |
| `lost` | Marked lost and not eligible for future grants. |

New sharing grants and new wrapped-key records require an `active` contact
public key. The API can register a new contact by omitting `contact_id`, rotate
an existing contact by providing an account-owned `contact_id`, or replace a
specific key with `POST /v1/contact-public-keys/{public_key_id}/replace`.
Rotated or replaced keys receive the next version and mark prior nonterminal
versions for that contact `replaced`. Replacement does not mutate old
sharing-grant or wrapped-key records; those records remain bound to the
original contact public-key ID and version. Contact public-key routes store
owner-scoped public-key metadata only. The accepted v1 preview wrapped-key
profile is documented in
[post-quantum-envelope.md](post-quantum-envelope.md) and uses
`proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1`. The contact-key route does not
prove possession of a recipient private key by itself; wrapped-key record
creation validates the accepted PQ wrapping profile.

### `POST /v1/contact-public-keys`

Registers trusted-contact public-key metadata for the authenticated account.
The optional `key_state` defaults to `pending_verification`.
`recipient_account_id` is optional for legacy or unbound contact-key metadata,
but trusted-contact wrapped-key read routes require it to match the signed-in
recipient account.

Request:

```json
{
  "recipient_account_id": "acct_contact...",
  "display_label": "Trusted contact",
  "wrapping_algorithm": "proofline-pq-mlkem768-hkdfsha384-aes256gcm",
  "public_key": "base64url-or-profile-encoded-public-key",
  "public_key_fingerprint": "fingerprint-...",
  "key_state": "pending_verification"
}
```

Response `201`:

```json
{
  "contact_public_key": {
    "public_key_id": "cpk_...",
    "owner_account_id": "acct_...",
    "contact_id": "ctc_...",
    "recipient_account_id": "acct_contact...",
    "version": 1,
    "display_label": "Trusted contact",
    "wrapping_algorithm": "proofline-pq-mlkem768-hkdfsha384-aes256gcm",
    "public_key": "base64url-or-profile-encoded-public-key",
    "public_key_fingerprint": "fingerprint-...",
    "key_state": "pending_verification",
    "created_at": "2026-06-01T10:00:00Z",
    "updated_at": "2026-06-01T10:00:00Z"
  }
}
```

### `GET /v1/contact-public-keys`

Lists contact public-key metadata owned by the authenticated account.

### `GET /v1/contact-public-keys/{public_key_id}`

Returns one account-owned contact public-key record. Records owned by another
account return `404 contact_public_key_not_found`.

### `PATCH /v1/contact-public-keys/{public_key_id}`

Updates mutable contact-key metadata. The request may change `display_label`
and may move nonterminal keys between `pending_verification` and `active`.
Terminal states use the dedicated revoke, lost, and replace routes. Replaced,
revoked, or lost keys cannot be reactivated.

Request:

```json
{
  "display_label": "Verified contact",
  "key_state": "active"
}
```

### `POST /v1/contact-public-keys/{public_key_id}/revoke`

Marks one account-owned contact public key revoked. Revocation prevents the key
from receiving new sharing grants or future wrapped-key records. It does not
delete already accepted ciphertext, bundle contents, or any material a future
authorized actor may already have downloaded.

### `POST /v1/contact-public-keys/{public_key_id}/lost`

Marks one account-owned contact public key lost after the trusted contact or
client reports private-key/device loss. Lost keys cannot receive new sharing
grants or future wrapped-key records and cannot be reactivated. Existing
sharing-grant and wrapped-key records are not rewritten.

### `POST /v1/contact-public-keys/{public_key_id}/replace`

Creates a successor public-key version for the same `contact_id` and marks
prior nonterminal versions for that contact `replaced`. The request body uses
the same public-key metadata fields as creation except `contact_id`.

Request:

```json
{
  "display_label": "Trusted contact replacement",
  "wrapping_algorithm": "proofline-pq-mlkem768-hkdfsha384-aes256gcm",
  "public_key": "base64url-or-profile-encoded-public-key",
  "public_key_fingerprint": "fingerprint-...",
  "key_state": "pending_verification"
}
```

Response `201` returns the new `contact_public_key`. The old record remains
readable to the owner with `key_state: "replaced"`, `replaced_at`, and
`replaced_by_public_key_id`. Existing wrapped-key records keep the old
`contact_public_key_id` and `contact_public_key_version`; delivery filters omit
them while the referenced key is no longer active.

## Trusted Contact Relationships

Trusted-contact relationship routes are mounted on the main API listener and
require a valid local account session. They model the account-to-account
relationship lifecycle used by signed-in trusted-contact authorization. They
are separate from public viewer tokens, contact public-key records, sharing
grants, and wrapped-key records; relationship state alone does not deliver
wrapped keys.

A trusted-contact relationship records owner account, recipient account, role,
state, timestamps, optional display label, and revocation or replacement
metadata. It does not store contact private keys, raw CEKs, raw media keys,
wrapped-key ciphertext, plaintext, browser fragment secrets, notification
payloads, emergency-dispatch state, request bodies, uploaded bytes, stored
paths, object keys, or private deployment details.

Relationship states are:

| State | Meaning |
|---|---|
| `pending_invite` | Owner invited the recipient account; the recipient has not accepted or declined. |
| `active` | Recipient account accepted the relationship. |
| `declined` | Recipient account declined the invite. |
| `revoked` | Owner account revoked the relationship. |
| `replaced` | Owner account replaced this relationship with a successor invite. |

Only the owner account can create, revoke, or replace relationships. Only the
recipient account can accept or decline an invite. Opening an incident viewer
link does not create or accept a trusted-contact relationship, and a viewer
token is not a `/v1` account credential.

The only implemented relationship role is:

| Role | Meaning |
|---|---|
| `trusted_contact` | General trusted-contact relationship metadata for future account-based access design. |

### `POST /v1/trusted-contact-relationships`

Creates a pending trusted-contact invite for another active local account. The
owner must supply `recipient_account_id`; `relationship_role` defaults to
`trusted_contact`.

Request:

```json
{
  "recipient_account_id": "acct_...",
  "relationship_role": "trusted_contact",
  "display_label": "Emergency contact"
}
```

Response `201`:

```json
{
  "trusted_contact_relationship": {
    "relationship_id": "tcr_...",
    "owner_account_id": "acct_owner...",
    "recipient_account_id": "acct_contact...",
    "relationship_role": "trusted_contact",
    "relationship_state": "pending_invite",
    "display_label": "Emergency contact",
    "created_at": "2026-06-10T10:00:00Z",
    "updated_at": "2026-06-10T10:00:00Z",
    "invited_at": "2026-06-10T10:00:00Z"
  }
}
```

Duplicate open relationships for the same owner, recipient, and role return
`409 trusted_contact_relationship_duplicate`.

### `GET /v1/trusted-contact-relationships`

Lists relationships where the authenticated account is the owner or recipient.

### `GET /v1/trusted-contact-relationships/{relationship_id}`

Returns one relationship visible to the authenticated owner or recipient.
Unrelated accounts receive `404 trusted_contact_relationship_not_found`.

### `POST /v1/trusted-contact-relationships/{relationship_id}/accept`

Marks a pending invite active. Only the authenticated recipient account can use
this route.

### `POST /v1/trusted-contact-relationships/{relationship_id}/decline`

Marks a pending invite declined. Only the authenticated recipient account can
use this route.

### `POST /v1/trusted-contact-relationships/{relationship_id}/revoke`

Marks a pending or active relationship revoked and records the owner account as
the revoking account. Only the owner account can use this route. Revocation
does not delete contact public keys, sharing grants, wrapped-key records,
encrypted chunks, bundle contents, or material already downloaded by a future
authorized actor.

### `POST /v1/trusted-contact-relationships/{relationship_id}/replace`

Marks a pending or active relationship replaced and creates a successor
`pending_invite` relationship. The replacement request may supply a new
`recipient_account_id`, `relationship_role`, and `display_label`; omitted
recipient and role values reuse the previous relationship values.

## Account And Device Recipient Keys

Account/device recipient-key routes are mounted on the main API listener and
require a valid local account session. They are scoped to the authenticated
account only; admins do not use these product routes to manage another
account's device keys unless the admin account owns those keys as its own
account. The server stores public recipient-key metadata, non-secret key IDs,
scheme/suite identifiers, fingerprints, state, timestamps, and optional display
labels. It does not store private keys, raw media keys, raw CEKs, ML-KEM shared
secrets, derived KEKs, plaintext, decrypted caches, browser fragment secrets,
request bodies, uploaded bytes, stored paths, staging paths, object keys, or
private deployment details.

These records are separate from trusted-contact public-key routes. They model
the owner's own account-level or device-level recipient keys so future clients
can wrap CEKs for the user's account or devices without treating every incident
as a new identity. Current wrapped-key creation remains trusted-contact
grant-scoped; account/device wrapped-key delivery is future work and must use
only `active` recipient-key versions when it is implemented.

Supported recipient types:

| Type | Meaning |
|---|---|
| `account` | Account-level recipient key metadata. |
| `device` | One owner device or client recipient key. |

Recipient key states:

| State | Meaning |
|---|---|
| `pending_verification` | Registered but not eligible for future wrapping. |
| `active` | Eligible for future account/device wrapping. |
| `replaced` | Superseded by another key version and not eligible for future wrapping. |
| `revoked` | Revoked and not eligible for future wrapping. |
| `lost` | Marked lost because the device/private-key copy is unavailable and not eligible for future wrapping. |

Revocation, replacement, and lost-device marking are forward-looking controls.
They stop future wrapping to the affected key version. They cannot claw back
wrapped keys, encrypted chunks, bundle contents, downloaded records, or any
plaintext produced by a future authorized client before the state change.

The accepted v1 preview recipient-key profile is documented in
[post-quantum-envelope.md](post-quantum-envelope.md). The current route accepts:

```text
scheme = proofline-pq-envelope-v1
suite_id = proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1
```

The route stores the caller-supplied public `key_id` and public key material as
metadata. It does not derive keys, prove private-key possession, unwrap CEKs, or
decrypt media.

### `POST /v1/account-recipient-keys`

Registers account/device recipient public-key metadata for the authenticated
account. `recipient_type` is required and must be `account` or `device`.
`recipient_id` is optional for a new recipient identity; if omitted, the server
creates one. New records default to `pending_verification`, and may start as
`active` only when the client has already completed its own verification.
Terminal states must be set with the dedicated revoke, replace, or lost routes.

Request:

```json
{
  "recipient_type": "device",
  "key_id": "recipient-key-id",
  "display_label": "Owner phone",
  "scheme": "proofline-pq-envelope-v1",
  "suite_id": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1",
  "public_key": "base64url-or-profile-encoded-public-key",
  "public_key_fingerprint": "fingerprint-...",
  "key_state": "pending_verification"
}
```

Response `201`:

```json
{
  "account_recipient_key": {
    "recipient_key_id": "ark_...",
    "owner_account_id": "acct_...",
    "recipient_id": "arcp_...",
    "recipient_type": "device",
    "key_id": "recipient-key-id",
    "version": 1,
    "display_label": "Owner phone",
    "scheme": "proofline-pq-envelope-v1",
    "suite_id": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1",
    "public_key": "base64url-or-profile-encoded-public-key",
    "public_key_fingerprint": "fingerprint-...",
    "key_state": "pending_verification",
    "created_at": "2026-06-10T10:00:00Z",
    "updated_at": "2026-06-10T10:00:00Z"
  }
}
```

Duplicate `recipient_id` or `key_id` values for the same account return
`409 account_recipient_key_duplicate`. Use the replacement route for a new
version of an existing recipient identity.

### `GET /v1/account-recipient-keys`

Lists account/device recipient-key metadata owned by the authenticated account.

### `GET /v1/account-recipient-keys/{recipient_key_id}`

Returns one account-owned recipient-key record. Records owned by another
account return `404 account_recipient_key_not_found`.

### `PATCH /v1/account-recipient-keys/{recipient_key_id}`

Updates mutable metadata. The request may change `display_label` and can move a
`pending_verification` key to `active`. It cannot reactivate `revoked`,
`replaced`, or `lost` keys, and it cannot use `PATCH` to set terminal states.

Request:

```json
{
  "display_label": "Verified owner phone",
  "key_state": "active"
}
```

Invalid state transitions return `409 invalid_account_recipient_key_state`.

### `POST /v1/account-recipient-keys/{recipient_key_id}/revoke`

Marks one account-owned recipient key revoked and records `revoked_at`.
Revoked keys are not eligible for future account/device wrapped-key records.

### `POST /v1/account-recipient-keys/{recipient_key_id}/lost`

Marks one account-owned recipient key lost and records `lost_at`. Use this when
a device or local private-key copy is unavailable, destroyed, taken, or
otherwise cannot be trusted for future wrapping.

### `POST /v1/account-recipient-keys/{recipient_key_id}/replace`

Creates a successor version for the same recipient identity and marks the old
record `replaced`. Replacement is rejected for already revoked, replaced, or
lost keys.

Request:

```json
{
  "key_id": "replacement-recipient-key-id",
  "display_label": "Replacement owner phone",
  "scheme": "proofline-pq-envelope-v1",
  "suite_id": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1",
  "public_key": "base64url-or-profile-encoded-public-key",
  "public_key_fingerprint": "fingerprint-...",
  "key_state": "active"
}
```

Response `201` contains the new `account_recipient_key` with the next version.
The old record remains readable as owner metadata with `key_state: "replaced"`,
`replaced_at`, and `replaced_by_recipient_key_id`.

## Sharing Grants

Sharing-grant routes are mounted on the main API listener and require a valid
local account session. Grant creation and incident-scoped listing are
account-owner actions: admins are not allowed to manage another account's
sharing grants through these product routes unless the admin account owns the
incident. Public viewer routes remain read-only and do not use these grant
records.

Sharing grants authorize metadata, ciphertext, and wrapped-key delivery
decisions. They do not decrypt media, create trusted-contact sessions, send
notifications, alter incident-mode behavior, or change public viewer and bundle
responses.

Grant data classes are:

| Data class | Meaning |
|---|---|
| `metadata` | Metadata access authorization. |
| `ciphertext` | Encrypted evidence access authorization. |
| `metadata_ciphertext` | Metadata and encrypted evidence access authorization. |

### `POST /v1/incidents/{incident_id}/sharing-grants`

Creates an active sharing-grant record for an incident owned by the
authenticated account. `stream_id` is optional; omit it for incident scope.
The referenced contact must have an active contact public key owned by the same
account. If `contact_public_key_id` is omitted, the latest active key version
for the contact is used. `recipient_type` defaults to `trusted_contact`,
`data_class` defaults to `metadata_ciphertext`, and `expires_at`, when present,
must be in the future.

Request:

```json
{
  "stream_id": "str_...",
  "contact_id": "ctc_...",
  "data_class": "metadata_ciphertext",
  "expires_at": "2026-06-08T10:00:00Z"
}
```

Response `201`:

```json
{
  "sharing_grant": {
    "grant_id": "sgr_...",
    "owner_account_id": "acct_...",
    "incident_id": "inc_...",
    "stream_id": "str_...",
    "recipient_type": "trusted_contact",
    "contact_id": "ctc_...",
    "contact_public_key_id": "cpk_...",
    "contact_public_key_version": 1,
    "data_class": "metadata_ciphertext",
    "grant_state": "active",
    "created_at": "2026-06-01T10:00:00Z",
    "updated_at": "2026-06-01T10:00:00Z",
    "expires_at": "2026-06-08T10:00:00Z"
  }
}
```

Missing incidents, streams, or active contact public keys return
`404 sharing_grant_dependency_not_found` without revealing which dependency was
missing outside the owner boundary.

### `GET /v1/incidents/{incident_id}/sharing-grants`

Lists sharing grants for an incident owned by the authenticated account.

### `GET /v1/sharing-grants/{grant_id}`

Returns one sharing grant owned by the authenticated account.

### `POST /v1/sharing-grants/{grant_id}/revoke`

Marks one account-owned sharing grant revoked and records the revoking account.
Revocation stops future grant-based authorization or delivery. It does not
delete encrypted chunks, incident metadata, bundle contents, or anything an
authorized actor may already have downloaded.

## Wrapped Keys

Wrapped-key routes are mounted on the main API listener and require a valid
local account session. They are account-owner routes: admins are not allowed to
store, list, read, or revoke another account's wrapped-key records through
these product routes unless the admin account owns the incident or record.
Separate trusted-contact read-only routes deliver wrapped-key records only to
the authenticated recipient account authorized by an accepted relationship,
recipient-bound active contact key, active unexpired ciphertext grant, and
active wrapped-key record.

The backend stores encrypted CEK/media-key material only when it is bound to an
active sharing grant that authorizes ciphertext access. The record includes the
incident, optional stream, `media_key_id` compatibility identifier, grant ID,
trusted-contact public key ID and version, wrapping algorithm/version,
wrapped-key ciphertext, and public wrapping metadata. In the future key model,
that identifier names the CEK scoped to the incident, stream, or bounded chunk
group. The record must not include raw CEKs, raw media keys, contact private
keys, plaintext, unwrapped shared secrets, browser fragment secrets, raw tokens,
or server escrow material.

Bundle manifests remain key-free. The current public incident viewer does not
deliver wrapped keys, and public viewer bundle downloads keep their existing
ciphertext-only behavior.

Wrapped-key records must use the accepted post-quantum profile in
[post-quantum-envelope.md](post-quantum-envelope.md) with these field values:

```text
wrapping_algorithm = proofline-pq-mlkem768-hkdfsha384-aes256gcm
wrapping_algorithm_version = 1
```

The API validates the accepted public metadata and wrapped-key frame shape
without unwrapping CEKs or decrypting media.

### `POST /v1/incidents/{incident_id}/wrapped-keys`

Stores one wrapped CEK/media-key record for an incident owned by the authenticated
account. `grant_id` must name an active owner-scoped sharing grant for the same
incident. The grant must authorize `ciphertext` or `metadata_ciphertext`, must
not be expired, and must point at an active contact public key. If the grant is
stream-scoped, `stream_id` must match the grant's stream.

Request:

```json
{
  "stream_id": "str_...",
  "grant_id": "sgr_...",
  "media_key_id": "media-key-2026-06-01-audio",
  "wrapping_algorithm": "proofline-pq-mlkem768-hkdfsha384-aes256gcm",
  "wrapping_algorithm_version": "1",
  "wrapped_key_ciphertext": "base64url-no-padding-PLPQWK1-frame",
  "public_wrapping_metadata": {
    "profile": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1",
    "scheme": "proofline-pq-envelope-v1",
    "suite_id": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1"
  }
}
```

Response `201`:

```json
{
  "wrapped_key": {
    "wrapped_key_id": "wkey_...",
    "owner_account_id": "acct_...",
    "incident_id": "inc_...",
    "stream_id": "str_...",
    "grant_id": "sgr_...",
    "recipient_type": "trusted_contact",
    "contact_id": "ctc_...",
    "contact_public_key_id": "cpk_...",
    "contact_public_key_version": 1,
    "media_key_id": "media-key-2026-06-01-audio",
    "wrapping_algorithm": "proofline-pq-mlkem768-hkdfsha384-aes256gcm",
    "wrapping_algorithm_version": "1",
    "wrapped_key_ciphertext": "base64url-no-padding-PLPQWK1-frame",
    "public_wrapping_metadata": {
      "profile": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1",
      "scheme": "proofline-pq-envelope-v1",
      "suite_id": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1"
    },
    "wrapped_key_state": "active",
    "created_at": "2026-06-01T10:00:00Z",
    "updated_at": "2026-06-01T10:00:00Z"
  }
}
```

Missing incidents, streams, active grants, or active contact public keys return
`404 wrapped_key_dependency_not_found`. Metadata-only grants return
`409 wrapped_key_grant_not_authorized`. Reusing the same owner, incident,
stream scope, grant, `media_key_id`, and contact public key returns
`409 wrapped_key_duplicate`. Unsupported wrapping algorithms, malformed
wrapped-key frames, malformed public wrapping metadata, or profile mismatches
return `400` with a generic profile-validation error.

### `GET /v1/incidents/{incident_id}/wrapped-keys`

Lists active wrapped-key records for an incident owned by the authenticated
account. The list is delivery-filtered: records are omitted when their grant is
revoked or expired, their contact public key is no longer active, or the
wrapped-key record itself is revoked or rotated.

### `GET /v1/wrapped-keys/{wrapped_key_id}`

Returns one active wrapped-key record owned by the authenticated account,
subject to the same delivery filter as the incident list route.

### `GET /v1/trusted-contact/incidents/{incident_id}/wrapped-keys`

Lists active wrapped-key records for the authenticated trusted-contact account.
The response includes only records where all of the following are true:

- an active accepted trusted-contact relationship links the owner account to
  the authenticated recipient account
- the wrapped-key record uses a contact public key whose
  `recipient_account_id` matches the authenticated recipient account
- the sharing grant is active, unexpired, and authorizes `ciphertext` or
  `metadata_ciphertext`
- the contact public key is active
- the wrapped-key record is active

Unrelated accounts receive an empty list. Public viewer-token routes do not
mount this endpoint.

### `GET /v1/trusted-contact/wrapped-keys/{wrapped_key_id}`

Returns one active wrapped-key record authorized for the authenticated
trusted-contact account under the same filters as the incident trusted-contact
list route. Unauthorized, expired, revoked, replaced, lost, rotated, or
unbound records return `404 wrapped_key_not_found`.

### `POST /v1/wrapped-keys/{wrapped_key_id}/revoke`

Marks one account-owned wrapped-key record revoked and records the revoking
account. Revocation stops future delivery of that wrapped-key record. It does
not delete encrypted chunks, incident metadata, bundle contents, or material an
authorized actor may already have downloaded.

Contact-key, sharing-grant, wrapped-key, and incident deletion-pruning
lifecycle changes also write private repository-level audit metadata. The
current API does not expose audit routes. Audit records use controlled IDs,
actor/owner account IDs, action names, outcome categories, and timestamps only;
owner-scoped automatic pruning decisions without an explicit account actor use
the owner account as the private audit actor. They do not store raw
keys, wrapped-key ciphertext, public wrapping metadata, plaintext, tokens,
request bodies, uploaded bytes, stored paths, object keys, private deployment
details, or user safety narratives.

## Incidents

Incident routes are mounted on the main API listener and require a valid
session. Incidents are owned by the account that creates them. The account
incident list and detail routes below return only public-safe metadata for
incidents owned by the authenticated account. They do not include notes,
chunks, checkins, stored paths, object keys, owner account IDs, wrapped keys,
ciphertext, raw keys, plaintext, or user safety narrative. Admin accounts do
not get cross-account reads through these account routes unless the admin
account also owns the incident. Legacy unowned incidents are hidden from account
list/detail reads unless an admin assigns one incident to an existing account
through the private reassignment workflow; see
[legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md).

### `POST /v1/incidents`

Creates an open incident. When mode fields are omitted, the incident remains a
generic legacy incident. The request may include optional mode metadata, but
these fields do not grant access, create public links, send notifications,
change retention, change key custody, expose trusted-contact workflows, or change
public viewer and bundle behavior.

Request:

```json
{
  "client_label": "iphone",
  "notes": "test incident",
  "incident_mode": "interaction_record",
  "capture_profile": "audio_location",
  "escalation_policy": "none",
  "sharing_state": "private"
}
```

Optional mode values:

| Field | Accepted values |
|---|---|
| `incident_mode` | `emergency`, `interaction_record`, `safety_check`, `evidence_note` |
| `capture_profile` | `audio_video_location`, `audio_location`, `location_checkin`, `note_or_attachment`, `custom` |
| `escalation_policy` | `none`, `trusted_contacts_on_start`, `trusted_contacts_on_missed_checkin`, `urgent_trusted_contact_alert` |
| `sharing_state` | `private`, `trusted_contact_access`, `public_link_created`, `legal_export_created`, `revoked_or_expired` |

Response `201`:

```json
{
  "incident_id": "inc_...",
  "status": "open",
  "incident_mode": "interaction_record",
  "capture_profile": "audio_location",
  "escalation_policy": "none",
  "sharing_state": "private"
}
```

### `GET /v1/incidents`

Lists public-safe metadata for non-deleted incidents owned by the authenticated
account. The response is ordered newest-updated first. Incidents owned by other
accounts and legacy unowned incidents are omitted.

Response `200`:

```json
{
  "incidents": [
    {
      "id": "inc_...",
      "created_at": "2026-05-21T10:00:00Z",
      "updated_at": "2026-05-21T10:00:00Z",
      "status": "open",
      "client_label": "iphone",
      "incident_mode": "interaction_record",
      "capture_profile": "audio_location",
      "escalation_policy": "none",
      "sharing_state": "private",
      "deletion_state": "active"
    }
  ]
}
```

### `GET /v1/incidents/{incident_id}`

Returns public-safe metadata for one incident owned by the authenticated
account. Missing incidents, incidents owned by another account, legacy unowned
incidents, and deleted incidents all return `404 incident_not_found`.

Response `200`:

```json
{
  "incident": {
    "id": "inc_...",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z",
    "status": "open",
    "client_label": "iphone",
    "incident_mode": "interaction_record",
    "capture_profile": "audio_location",
    "escalation_policy": "none",
    "sharing_state": "private",
    "deletion_state": "active"
  }
}
```

### `POST /v1/incidents/{incident_id}/close`

Marks an incident closed. Later chunk uploads return `409 incident_closed`.

Response `200` is the updated incident object. If the incident has
optional mode metadata, the same fields shown in the `GET` incident object can
be present. Closing an incident does not change sharing, retention, viewer,
notification, or key-custody behavior.

### `POST /v1/incidents/{incident_id}/deletion`

Requests deletion for an incident owned by the authenticated account. Admins
can use this route only for incidents they own; use the admin route below for
global deletion. The route creates durable deletion state and snapshots
server-controlled stored paths from metadata before any blob is deleted. It is
mounted only on the main API listener.

Request:

```json
{
  "reason_code": "account_delete",
  "allow_open": true
}
```

`reason_code` is optional and must be a short non-sensitive code using letters,
digits, `_`, `-`, `.`, or `:`. It must not contain raw tokens, request bodies,
evidence notes, private deployment details, plaintext, raw keys, stored paths,
object keys, or user safety narrative. Open incidents are rejected unless
`allow_open` is true. Repeating a deletion request for the same incident returns
the existing deletion decision.

Response `202`:

```json
{
  "deletion": {
    "decision_id": "del_...",
    "incident_id": "inc_...",
    "source": "account_request",
    "reason_code": "account_delete",
    "actor_account_id": "acct_...",
    "allow_open": true,
    "state": "deletion_pending",
    "item_count": 2,
    "requested_at": "2026-05-31T10:00:00Z",
    "updated_at": "2026-05-31T10:00:00Z"
  }
}
```

### `GET /v1/incidents/{incident_id}/deletion`

Returns the non-sensitive deletion status for an incident visible to the
authenticated account.

### `GET /v1/admin/incidents/unowned`

Lists legacy incidents whose `owner_account_id` is empty. This route is mounted
only on the private-admin listener and requires an admin account. It is for
one-incident-at-a-time operator review before reassignment or quarantine.

The optional `limit` query parameter defaults to `100` and is capped at `500`.

The response is count-oriented and intentionally omits notes, chunk paths,
object keys, `original_filename`, location coordinates, raw tokens, request
bodies, uploaded bytes, plaintext, raw keys, and user safety narrative:

```json
{
  "incidents": [
    {
      "incident_id": "inc_...",
      "status": "open",
      "deletion_state": "active",
      "created_at": "2026-06-02T10:00:00Z",
      "updated_at": "2026-06-02T10:00:00Z",
      "stream_count": 1,
      "chunk_count": 5,
      "checkin_count": 1,
      "incident_token_count": 1,
      "has_active_viewer_tokens": true,
      "incident_mode": "interaction_record",
      "capture_profile": "audio_location",
      "escalation_policy": "none",
      "sharing_state": "private"
    }
  ]
}
```

### `POST /v1/admin/incidents/{incident_id}/reassignment`

Records one private admin decision for an active legacy unowned incident. The
route either assigns the incident to an existing account or records a reviewed
`keep_unowned` decision. It updates only incidents whose `owner_account_id` is
still empty and does not change public viewer tokens, bundle behavior, deletion
state, retention state, encrypted blobs, token hashes, or key custody.

Assignment request:

```json
{
  "action": "assign_owner",
  "new_owner_account_id": "acct_...",
  "reason_code": "owner_verified"
}
```

Keep-unowned request:

```json
{
  "action": "keep_unowned",
  "reason_code": "keep_admin_only"
}
```

Supported `reason_code` values are `owner_verified`, `owner_request`,
`operator_review`, `keep_admin_only`, `unknown_owner`, and
`other_controlled`. Free-form notes are not accepted.

Successful responses return safe audit metadata only:

```json
{
  "event": {
    "id": "lra_...",
    "incident_id": "inc_...",
    "new_owner_account_id": "acct_...",
    "actor_account_id": "acct_admin_...",
    "action": "assign_owner",
    "reason_code": "owner_verified",
    "source": "admin_api",
    "created_at": "2026-06-02T10:00:00Z",
    "completed_at": "2026-06-02T10:00:00Z"
  }
}
```

### `POST /v1/admin/incidents/{incident_id}/deletion`

Requests deletion for any incident visible to an admin account. The request and
response shape match the account route, but the `source` is `admin_request`.

### `GET /v1/admin/incidents/{incident_id}/deletion`

Returns the non-sensitive deletion status for any incident by ID. This route
requires an admin account.

Deletion states are:

| State | Meaning |
|---|---|
| `active` | Incident is not being deleted. |
| `deletion_pending` | A deletion decision exists and blob deletion items have been prepared. |
| `deleting` | The background worker is deleting encrypted blobs and metadata. |
| `deletion_failed` | Blob deletion failed and the deletion is retryable. |
| `deleted` | Encrypted blobs and sensitive child metadata have been removed or confirmed absent. |

While an incident is not `active`, write routes, bundle routes, chunk upload,
and new incident-token creation fail closed. Public viewer token lookups for
the incident return the same `404 incident_token_invalid` shape used for
invalid, expired, or revoked tokens and do not reveal deletion state.

## Chunks

Chunk routes are mounted on the main API listener.

### `POST /v1/incidents/{incident_id}/chunks`

Uploads one already-encrypted chunk as `multipart/form-data`.

Optional header:

- `Idempotency-Key`: stable key for this intended complete chunk upload. The
  value must be 1-255 visible ASCII characters. The server treats it as
  token-like: raw values are not logged, returned in errors, or stored raw.

Fields:

- `file`: encrypted chunk bytes
- `stream_id`: media stream ID
- `chunk_index`: positive stream-local integer
- `media_type`: `audio`, `video`, `location`, or `metadata`
- `started_at`: RFC3339 timestamp
- `ended_at`: RFC3339 timestamp, not before `started_at`
- `sha256_hex`: lowercase SHA-256 hex of the encrypted bytes
- `original_filename`: optional client-supplied display metadata

Response `201`:

```json
{
  "id": "chk_...",
  "incident_id": "inc_...",
  "stream_id": "str_...",
  "chunk_index": 1,
  "media_type": "audio",
  "started_at": "2026-05-21T10:00:00Z",
  "ended_at": "2026-05-21T10:00:10Z",
  "original_filename": "chunk.enc",
  "stored_path": "incidents/inc_.../streams/str_.../audio_000001.enc",
  "byte_size": 23,
  "sha256_hex": "...",
  "created_at": "2026-05-21T10:00:11Z"
}
```

When `stream_id` is provided, the stream must exist, belong to the same incident, be open, and have the same `media_type` as the uploaded chunk. Streamed chunks must use indexes starting at `1`; `chunk_index <= 0` returns `400 invalid_chunk_index`. Uploads to completed or failed streams return `409 stream_not_open`.

Clients must create a media stream and upload chunks with `stream_id`. Legacy
unstreamed chunk rows may still be read where they already exist, but new
uploads without a valid PQ payload frame and stream-bound identity fail closed.

Streamed chunk identity is `(incident_id, stream_id, chunk_index)`, so each
stream can use normal stream-local chunk numbering.

Duplicate streamed `(incident_id, stream_id, chunk_index)` uploads without an
idempotency key return `409 duplicate_chunk`. Hash mismatches return
`400 hash_mismatch` and do not commit a final file.

Before committing a new encrypted chunk, the server checks the owning account's
committed encrypted blob usage from accepted chunk metadata. If the additional
chunk would exceed `SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES`, the route returns
`507 account_storage_quota_exceeded` with a generic error. Equivalent duplicate
or idempotent retries do not add new committed bytes. Chunks continue to count
while incident deletion is pending or retrying and stop counting only after
durable blob deletion completes and chunk metadata is removed. Failed, staged,
or orphan temp uploads are not committed quota and are handled by separate temp
upload controls.

When regular local temp-upload staging files reach
`SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES`, chunk upload returns
`507 upload_staging_quota_exceeded` with a generic error. The response does not
include temp paths, stored paths, object keys, bucket names, or uploaded bytes.

When `Idempotency-Key` is supplied, the server hashes the key and stores durable
upload-operation state in the configured metadata backend. The key is bound to
the `upload_chunk` operation and a request fingerprint covering normalized
chunk identity, `media_type`, `started_at`, `ended_at`, normalized
`original_filename`, ciphertext byte size, and ciphertext `sha256_hex`.

The first successful idempotent upload returns the normal `201` chunk response.
An equivalent retry with the same key and same complete encrypted chunk upload
can return `200 OK` with the same chunk metadata shape and:

```http
Idempotency-Replayed: true
```

Reusing the same `Idempotency-Key` with a different chunk identity, metadata
fingerprint, byte size, or ciphertext hash returns `409 idempotency_conflict`.
The conflict response is intentionally small and does not include uploaded
bytes, stored paths, object keys, raw keys, tokens, raw idempotency keys, or
private deployment details. Replays still upload the complete encrypted chunk;
this is not a resumable upload or partial-commit protocol.

When `SAFE_COORDINATION_BACKEND=valkey` or `redis` is configured, complete
chunk uploads also acquire a short-lived server-controlled coordination lease
for the normalized chunk identity after the server has read and validated the
multipart upload. If another API node is already processing that chunk
identity, the route returns a retryable response:

```json
{
  "error": {
    "code": "upload_in_progress",
    "message": "upload for this chunk identity is already in progress"
  }
}
```

The response status is `409 Conflict` and includes `Retry-After` based on the
configured `SAFE_UPLOAD_COORDINATION_LEASE_TTL`. If configured coordination is
unavailable during upload, the route returns `503 upload_coordination_unavailable`
with a safe `Retry-After` hint. Valkey lease keys are hashes of server-normalized
chunk identity; they do not contain raw tokens, raw idempotency keys, request
bodies, uploaded bytes, stored paths, object keys, plaintext, or raw keys.
These leases are in-progress hints only. Metadata uniqueness constraints,
upload-operation rows, and blob no-overwrite behavior remain final truth.

The repository rechecks incident state, stream state, and committed account
quota when chunk metadata is inserted. If an upload races with incident close,
stream completion, or quota exhaustion, the final metadata insert is rejected
and the committed blob path is removed.

For clients using the PQ envelope, `sha256_hex` is the SHA-256 of the complete
uploaded `PLPQENC1` payload frame bytes, not the plaintext. Missing, legacy,
downgraded, or mismatched envelope frames fail closed with
`400 invalid_envelope`.

`original_filename` is metadata, not a storage path. The server trims the value,
normalizes slash and backslash separators to a basename, falls back to the
multipart upload filename when the explicit field is empty, and stores the
resulting basename with chunk metadata. The value may be returned by private
chunk metadata routes, token-scoped public incident viewer summaries, and
completed stream or incident bundle manifests. Server stored paths, staging
paths, local filesystem paths, and object-storage keys are separate
server-controlled values and are not derived from `original_filename`.

Future clients should omit `original_filename` by default or send a generic,
non-identifying basename unless the user or a future protocol mode explicitly
chooses to preserve filename context as evidence metadata. Filenames can still
contain personal or contextual information even after path stripping. Do not use
`original_filename` for identity, authorization, storage lookup, decryption,
legal-record guarantees, or download path construction.

The current API does not implement resumable uploads, partial-upload lease
sessions, or client-side queue summary endpoints. Clients should retry
complete encrypted chunks, use `Idempotency-Key` for ambiguous complete-upload
outcomes, and use the duplicate chunk reconciliation route when they need to
compare a duplicate accepted chunk with a local expected fingerprint. The
resumable-upload planning decision is documented in
[resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md).
Future upload telemetry remains client-local before v1 preview unless a later
issue implements the safe coarse-code boundary documented in
[upload-telemetry-boundary.md](upload-telemetry-boundary.md).

The current API can issue short-lived regional relay upload and fanout
capabilities for authorized open streams and exposes narrow
service-authenticated core relay preflight, commit, and fanout authorization
routes. The separate `cmd/stream-ingress` relay can accept configured complete
encrypted chunk uploads at `POST /upload/complete-chunk`, stage ciphertext
temporarily, verify the declared SHA-256, forward exact bytes to the core
routes, and serve optimistic encrypted `GET /fanout/subscribe` SSE events
marked `near_live_unconfirmed` before emitting bounded `relay_chunk_state`
events for `confirmed`, `rejected`, or `terminal_failure` core commit
outcomes. It does not implement replay, relay metrics, production
service-identity rotation, or deployment automation. The relay upload and
fanout design is documented in
[regional-stream-ingress-relay.md](regional-stream-ingress-relay.md).
Any relay implementation must keep the core API authoritative for
authorization, idempotency, final blob commits, and metadata, and must not
expose the full `/v1` control plane or admin routes through the relay.

### `POST /v1/relay/preflight`

Service-authenticated relay-to-core preflight route for cheap complete-chunk
metadata checks before the stream-ingress relay accepts a large body where
practical.
This route is mounted on the main API mux, not on the public incident viewer,
not on the private-admin listener, and not on `cmd/stream-ingress`.

The trusted relay must send:

```http
X-Proofline-Relay-Service-Token: <relay-service-token>
```

The service token is configured with `SAFE_RELAY_SERVICE_AUTH_TOKEN` or
`SAFE_RELAY_SERVICE_AUTH_TOKEN_FILE` and is separate from user bearer sessions,
browser cookies, viewer tokens, incident tokens, and relay upload
capabilities. The route also requires a signed relay upload capability issued
by `POST /v1/incidents/{incident_id}/streams/{stream_id}/relay-session`.

Request:

```json
{
  "relay_session_id": "Lzhc7ZXZQD6bLztwBBqJ8A",
  "capability": "proofline-relay-capability-v1...",
  "incident_id": "inc_...",
  "stream_id": "str_...",
  "chunk_index": 1,
  "media_type": "audio",
  "started_at": "2026-06-11T10:00:00Z",
  "ended_at": "2026-06-11T10:00:10Z",
  "byte_size": 32768,
  "sha256_hex": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "original_filename": "chunk-0001.pq"
}
```

The core validates relay service auth, capability signature, expiry, `upload`
role, relay session binding, incident binding, stream binding, capability byte
and chunk limits, media type, incident state, stream state, duplicate chunk
identity, configured upload byte limit, and account committed-blob quota.

Response `200`:

```json
{
  "relay_preflight": {
    "status": "accepted",
    "incident_id": "inc_...",
    "stream_id": "str_...",
    "chunk_index": 1,
    "media_type": "audio",
    "max_chunk_bytes": 52428800,
    "max_chunks": 64
  }
}
```

Preflight success is a hint only. It does not stage bytes, reserve durable
storage, insert metadata, create evidence, or guarantee a later commit.

Common safe failures include `401 relay_service_auth_required`, `503
relay_service_auth_not_configured`, `503 relay_capability_not_configured`,
`401 relay_capability_invalid`, `401 relay_capability_expired`, `403
relay_capability_wrong_role`, `403 relay_capability_wrong_binding`, `403
relay_capability_limit_exceeded`, `409 duplicate_chunk`, `409 incident_closed`,
`409 incident_deleting`, `409 stream_not_open`, `404 incident_not_found`, `404
stream_not_found`, `413 upload_too_large`, and `507
account_storage_quota_exceeded`.

### `POST /v1/relay/commit`

Service-authenticated relay-to-core durable commit route for a complete
encrypted chunk. The route uses the same `X-Proofline-Relay-Service-Token`
header and relay capability validation as preflight, then commits through the
existing core storage and metadata path.

Request is `multipart/form-data` with these fields:

| Field | Meaning |
|---|---|
| `relay_session_id` | Relay session ID returned with the capability. |
| `capability` | Signed relay upload capability. |
| `incident_id` | Incident being uploaded to. |
| `stream_id` | Target open media stream. |
| `chunk_index` | Positive stream-local chunk index. |
| `media_type` | `audio`, `video`, `location`, or `metadata`; must match the stream. |
| `started_at` / `ended_at` | RFC3339 chunk time range. |
| `byte_size` | Declared encrypted byte size. |
| `sha256_hex` | Declared lowercase SHA-256 of encrypted bytes. |
| `original_filename` | Optional metadata basename; not a storage path. |
| `file` | Complete encrypted PQ payload frame bytes. |

The core streams `file` through the existing temporary upload path, enforces
`SAFE_MAX_UPLOAD_BYTES` and temp staging quota, verifies declared byte size,
verifies computed SHA-256 against `sha256_hex`, validates the accepted PQ
payload frame, applies the existing complete-upload coordination lease when
configured, commits the encrypted blob to local or S3-compatible durable
storage, and writes chunk metadata in SQLite or PostgreSQL.

Response `201`:

```json
{
  "relay_commit": {
    "status": "committed",
    "chunk_id": "chk_...",
    "incident_id": "inc_...",
    "stream_id": "str_...",
    "chunk_index": 1,
    "media_type": "audio",
    "byte_size": 32768,
    "sha256_hex": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "created_at": "2026-06-11T10:00:11Z"
  }
}
```

The response intentionally omits `stored_path`, staging paths, object keys,
raw tokens, uploaded bytes, plaintext, raw keys, wrapped-key ciphertext, and
private deployment details. Hash mismatches return `400 hash_mismatch` without
committing metadata. Declared byte-size mismatches return `400
byte_size_mismatch`. Existing direct authenticated chunk upload behavior is
unchanged.

### `POST /v1/relay/fanout-authorize`

Service-authenticated relay-to-core authorization route for a relay fanout
subscriber. This route is mounted on the main API mux, not on the public
incident viewer, not on the private-admin listener, and not on
`cmd/stream-ingress`.

The trusted relay must send:

```http
X-Proofline-Relay-Service-Token: <relay-service-token>
```

Request:

```json
{
  "relay_session_id": "Lzhc7ZXZQD6bLztwBBqJ8A",
  "capability": "proofline-relay-capability-v1...",
  "incident_id": "inc_...",
  "stream_id": "str_..."
}
```

The core validates relay service auth, capability signature, expiry, `fanout`
role, relay session binding, incident binding, stream binding, incident state,
and stream state. A successful response authorizes only optimistic encrypted
fanout for that relay session/incident/stream context:

```json
{
  "relay_fanout": {
    "status": "authorized",
    "incident_id": "inc_...",
    "stream_id": "str_..."
  }
}
```

The response does not commit evidence, create replay state, confirm chunk
durability, or grant viewer-token, trusted-contact, admin, decryption, or key
access.

The separate `cmd/stream-ingress` SSE fanout route remains outside the core API.
After successful fanout authorization, it sends `relay_chunk` events containing
base64 ciphertext and safe ciphertext metadata with state
`near_live_unconfirmed`. If the relay forwarded that exact ciphertext to the
core commit route, it then sends a `relay_chunk_state` event without
`payload_b64`: `confirmed` for core `201` or `200`, `rejected` with
`terminal: true` and `retryable: false` for non-retryable core rejection, or
`terminal_failure` with `terminal: true` and `retryable: true` for timeout,
network loss, core `429`, core `5xx`, or invalid core success response.
Rejected and terminal-failure events close the affected in-memory fanout
stream/session. Hash mismatches are rejected before fanout publication.

### `POST /v1/incidents/{incident_id}/chunks/reconcile`

Reconciles a duplicate chunk identity against already accepted metadata without
re-uploading ciphertext. The route is mounted on the main API listener.

This is a separate private query workflow, not a public route and not an
enriched `409 duplicate_chunk` upload response. A separate route lets clients
compare expected metadata without re-uploading ciphertext, keeps duplicate
upload errors small, and coexists with the current idempotency-key
retry-success path.

Request:

```json
{
  "stream_id": "str_...",
  "chunk_index": 1,
  "media_type": "audio",
  "started_at": "2026-05-21T10:00:00Z",
  "ended_at": "2026-05-21T10:00:10Z",
  "byte_size": 23,
  "sha256_hex": "...",
  "original_filename": "chunk.enc"
}
```

For streamed chunks, `stream_id` is required and identity is
`(incident_id, stream_id, chunk_index)`. `media_type` remains required and must
match the stream media type. Legacy unstreamed chunks are not accepted by the
current PQ upload default.

The comparison fingerprint is:

- normalized chunk identity
- `media_type`
- `started_at`
- `ended_at`
- normalized `original_filename`, including empty value
- ciphertext `byte_size`
- ciphertext `sha256_hex`

The route allows reconciliation after an incident is closed or a stream is
complete or failed, because it is read-only and only confirms already accepted
metadata. It does not overwrite, replace, delete, rewrite, or re-commit stored
chunks.

Matched response `200`:

```json
{
  "reconciliation": {
    "status": "matched",
    "identity": {
      "incident_id": "inc_...",
      "stream_id": "str_...",
      "chunk_index": 1,
      "media_type": "audio"
    },
    "chunk_id": "chk_...",
    "byte_size": 23,
    "sha256_hex": "...",
    "started_at": "2026-05-21T10:00:00Z",
    "ended_at": "2026-05-21T10:00:10Z",
    "created_at": "2026-05-21T10:00:11Z"
  }
}
```

Conflict response `409`:

```json
{
  "error": {
    "code": "duplicate_chunk_conflict",
    "message": "existing chunk does not match expected ciphertext or metadata"
  },
  "reconciliation": {
    "status": "conflict",
    "identity": {
      "incident_id": "inc_...",
      "stream_id": "str_...",
      "chunk_index": 1,
      "media_type": "audio"
    },
    "mismatched_fields": ["sha256_hex", "byte_size"]
  }
}
```

The conflict response should identify mismatched field names, not the existing
stored values. If no accepted chunk exists for the requested identity, return
`404 chunk_not_found`. Invalid identity or fingerprint fields should reuse the
existing upload validation error codes where practical, such as
`400 invalid_chunk_index`, `400 invalid_media_type`, or
`400 invalid_sha256_hex`. Invalid or missing `byte_size` returns
`400 invalid_byte_size`.

Safe reconciliation responses may return server-generated chunk ID, normalized
identity fields, timestamps, byte size, ciphertext hash, creation time, and
field names that matched or mismatched. They must not return uploaded bytes,
plaintext, raw keys, raw tokens, request bodies, local filesystem paths,
`stored_path`, staging paths, object-storage keys, or object-storage
credentials.

HTTP coverage in `internal/httpapi/uploads_test.go` includes:

- matched streamed duplicate reconciliation
- conflicting streamed duplicate reconciliation
- fail-closed behavior for legacy unstreamed upload attempts
- omission of `stored_path` and stored conflicting values from reconciliation
  responses
- read-only reconciliation after stream completion, stream failure, or incident
  close

### `GET /v1/incidents/{incident_id}/chunks`

Lists chunk metadata for one incident. This is not part of the public-safe
account incident list/detail surface.

### `GET /v1/incidents/{incident_id}/chunks/{media_type}/{chunk_index}`

Returns encrypted bytes for an existing legacy unstreamed chunk as
`application/octet-stream`. This route is private/dev-only and is not used by
the incident viewer. New uploads should be stream-scoped PQ payload frames.
Streamed chunks are read through completed stream bundle downloads rather than
this legacy media/index route.

## Media Streams

Media stream routes are mounted on the main API listener.

The current `MediaStream` model represents one concrete upload lane with one
`media_type`, stream-local chunk indexes, and `open`, `complete`, or `failed`
state. Future browser and native capture may group multiple concrete streams
under one capture source timeline, with explicit variant roles such as
`live_preview`, `evidence_master`, or `audio_priority`; that design is tracked
in [capture-stream-variants.md](capture-stream-variants.md). Do not encode
critical future variant semantics only in the free-form `label` field.

### `POST /v1/incidents/{incident_id}/streams`

Creates an open media stream for an incident.

Request:

```json
{
  "media_type": "audio",
  "label": "main audio recording"
}
```

Response `201`:

```json
{
  "stream": {
    "id": "str_...",
    "incident_id": "inc_...",
    "media_type": "audio",
    "label": "main audio recording",
    "status": "open",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z"
  }
}
```

Invalid media types return `400 invalid_media_type`.

### `GET /v1/incidents/{incident_id}/streams`

Lists media streams for an incident.

Response `200`:

```json
{
  "streams": []
}
```

### `GET /v1/incidents/{incident_id}/streams/{stream_id}`

Returns one stream as:

```json
{
  "stream": {
    "id": "str_...",
    "incident_id": "inc_...",
    "media_type": "audio",
    "status": "open",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z"
  }
}
```

### `POST /v1/incidents/{incident_id}/streams/{stream_id}/complete`

Marks an open stream complete after verifying chunks `1..expected_chunk_count` exist contiguously and each stored file is readable. Completion revalidates chunk rows in the repository before committing the state change.

Request:

```json
{
  "expected_chunk_count": 12
}
```

Response `200`:

```json
{
  "stream": {
    "id": "str_...",
    "incident_id": "inc_...",
    "media_type": "audio",
    "status": "complete",
    "expected_chunk_count": 12,
    "completed_at": "2026-05-21T10:02:00Z",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:02:00Z"
  }
}
```

Missing or non-contiguous chunks return `409 stream_chunks_incomplete` or `409 stream_chunks_not_contiguous`. Completing an already complete or failed stream returns `409`.

### `POST /v1/incidents/{incident_id}/streams/{stream_id}/fail`

Marks an open stream failed while preserving uploaded chunks.

Request:

```json
{
  "failure_reason": "client stopped recording unexpectedly"
}
```

Response `200` is the updated stream object with `status: "failed"` and `failed_at` set.

Failed streams can still contain preserved backend-confirmed evidence. Future
evidence resolution should account for failed or incomplete stream variants
instead of treating them as disposable previews.

### `POST /v1/incidents/{incident_id}/streams/{stream_id}/relay-session`

Issues short-lived regional relay upload and fanout capabilities for one
authorized open stream. This route is mounted on the authenticated main API
listener and uses the same incident write authorization as encrypted chunk
upload. It does not upload bytes, stage ciphertext, call a relay, commit to
storage, fan out live chunks, or create a durable evidence record.

Relay capability issuance is disabled until a secret is configured with
`SAFE_RELAY_CAPABILITY_SECRET` or `SAFE_RELAY_CAPABILITY_SECRET_FILE`. The
secret must be at least 32 bytes. The issued capabilities are HMAC-signed,
expire after `SAFE_RELAY_CAPABILITY_TTL`, are bound to the returned
`relay_session_id`, incident ID, stream ID, and either the `upload` or
`fanout` role, and carry bounded upload limits. They are bearer-like
credentials and must not be logged or copied into public artifacts.

Response `201`:

```json
{
  "relay_session": {
    "relay_session_id": "Lzhc7ZXZQD6bLztwBBqJ8A",
    "capability": "proofline-relay-capability-v1...",
    "fanout_capability": "proofline-relay-capability-v1...",
    "role": "upload",
    "incident_id": "inc_...",
    "stream_id": "str_...",
    "expires_at": "2026-06-11T10:05:00Z",
    "max_chunk_bytes": 52428800,
    "max_chunks": 64,
    "allowed_media_types": ["audio"]
  }
}
```

Closed incidents return `409 incident_closed`; missing streams return
`404 stream_not_found`; completed or failed streams return `409
stream_not_open`; missing capability secret returns `503
relay_capability_not_configured`.

### `GET /v1/incidents/{incident_id}/streams/{stream_id}/download`

Downloads a completed stream as a ZIP bundle. Open or failed streams return `409 stream_not_complete`.

Successful responses include:

```http
Content-Type: application/zip
Content-Disposition: attachment; filename="incident_inc_..._audio_str_....zip"
Content-Security-Policy: default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'
X-Content-Type-Options: nosniff
Cache-Control: no-store
Referrer-Policy: no-referrer
Permissions-Policy: geolocation=(), microphone=(), camera=()
X-Frame-Options: DENY
```

ZIP contents:

```text
manifest.json
chunks/audio_000001.enc
chunks/audio_000002.enc
```

The manifest is generated from trusted database metadata and includes incident
ID, stream ID, media type, status, chunk count, total bytes, chunk SHA-256
metadata, and any stored `original_filename` basename for each chunk. Server
filesystem paths are not included.
It also includes a non-secret `encryption` hint indicating expected client-side encryption and `server_decrypts: false`.

Future live or partial stream access is planning-only and should not be inferred
from this completed bundle route. See
[live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md).

### `GET /v1/incidents/{incident_id}/download`

Downloads a ZIP bundle containing all completed streams for an incident:

```text
manifest.json
streams/{stream_id}/manifest.json
streams/{stream_id}/chunks/audio_000001.enc
```

Open, failed, and legacy unstreamed chunks are omitted from this initial bundle format.

If any completed stream cannot be reconstructed, the incident bundle request fails with `409 incident_bundle_inconsistent` rather than returning a partial bundle. The error response does not include server filesystem paths, stored chunk paths, or ZIP entry names.

Future canonical bundle or export manifests may use source-timeline evidence
resolution to choose the best backend-confirmed variant per segment. That must
be a separate manifest/API design and must not delete lower-quality fallback
chunks merely because a higher-quality variant was planned.

## Checkins

Checkin routes are mounted on the main API listener.

### `POST /v1/incidents/{incident_id}/checkins`

Adds optional device status and location metadata. These fields are existing
server-visible check-in metadata. They are not the future full-fidelity
encrypted GPS/location evidence model documented in
[encrypted-location-context.md](encrypted-location-context.md).

Request:

```json
{
  "device_battery_percent": 82,
  "device_network": "wifi",
  "latitude": -37,
  "longitude": 145,
  "accuracy_meters": 20
}
```

Response `201` is the created checkin.

## Viewer Tokens

Incident-token creation, owner metadata reads, and revocation routes are
mounted on the main API listener.

### `POST /v1/incidents/{incident_id}/incident-tokens`

Creates a read-only viewer token for one incident. The raw token is returned only in this response; the configured metadata backend stores only a SHA-256 hash.

`expires_at` is optional. When omitted, the API applies the configured default token lifetime, which is 24 hours unless `SAFE_DEFAULT_INCIDENT_TOKEN_TTL` is changed. Explicit `expires_at` values are preserved; send `null` to explicitly create a token that remains valid until revoked. Setting `SAFE_DEFAULT_INCIDENT_TOKEN_TTL=0` disables the default and lets omitted expiries remain valid until revoked.

Request:

```json
{
  "label": "trusted contact",
  "expires_at": "2030-01-01T00:00:00Z"
}
```

Response `201`:

```json
{
  "token_id": "itk_...",
  "incident_id": "inc_...",
  "token": "...",
  "label": "trusted contact",
  "created_at": "2026-05-21T10:00:00Z",
  "expires_at": "2030-01-01T00:00:00Z"
}
```

The response includes `Cache-Control: no-store`.

### `GET /v1/incidents/{incident_id}/incident-tokens`

Lists non-secret viewer-token metadata for an incident owned by the
authenticated account.

Response `200`:

```json
{
  "incident_tokens": [
    {
      "token_id": "itk_...",
      "incident_id": "inc_...",
      "label": "trusted contact",
      "token_state": "active",
      "created_at": "2026-05-21T10:00:00Z",
      "expires_at": "2030-01-01T00:00:00Z"
    }
  ]
}
```

`token_state` is one of `active`, `expired`, or `revoked`. The response never
includes the raw viewer token or token hash. It does not allow token replay and
does not expose public token lookup by token ID. Non-owner accounts cannot read
another account's viewer-token metadata.

### `GET /v1/incidents/{incident_id}/incident-tokens/{token_id}`

Reads one non-secret viewer-token metadata record for an incident owned by the
authenticated account.

Response `200`:

```json
{
  "incident_token": {
    "token_id": "itk_...",
    "incident_id": "inc_...",
    "label": "trusted contact",
    "token_state": "revoked",
    "created_at": "2026-05-21T10:00:00Z",
    "expires_at": "2030-01-01T00:00:00Z",
    "revoked_at": "2026-05-21T11:00:00Z"
  }
}
```

If the token metadata record is not found for that incident, the route returns
`404 incident_token_not_found`. The response never includes the raw viewer token
or token hash.

### `POST /v1/incident-tokens/{token_id}/revoke`

Revokes a viewer token by ID.

Response `200`:

```json
{
  "token_id": "itk_...",
  "revoked": true
}
```

## Incident Viewer

Incident viewer routes are mounted on the main API/viewer listener. The current
server-rendered `/i/{token}` page and pre-rename `/e/{token}` aliases are
implemented prototype/local compatibility routes, not the long-term canonical
viewer surface. Proofline has no current public deployments that require a
long-lived compatibility window for these route shapes. Future no-account
viewer links should point at the web-client origin using the fragment-token
shape documented in [web-client-viewer-routing.md](web-client-viewer-routing.md).

The backend may still keep token-scoped data and encrypted download primitives
that a web-client viewer needs. Do not treat this section as approval for broad
public `/v1` exposure or for routing private write/admin routes from public
viewer edges.

### `GET /i/{token}`

Renders a read-only HTML summary for a valid, unexpired, unrevoked token. The page includes embedded static CSS/JS files served from `/static/`.

### `GET /i/{token}/data`

Returns the same read-only summary as JSON for polling.

Response `200`:

```json
{
  "incident": {
    "id": "inc_...",
    "status": "open",
    "client_label": "iphone",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z"
  },
  "latest_checkin": null,
  "chunk_count_by_media_type": {},
  "latest_chunk_by_media_type": {},
  "media": [
    {
      "media_type": "audio",
      "chunk_count": 0
    },
    {
      "media_type": "video",
      "chunk_count": 0
    },
    {
      "media_type": "location",
      "chunk_count": 0
    },
    {
      "media_type": "metadata",
      "chunk_count": 0
    }
  ],
  "streams": [],
  "completed_streams": [],
  "warning": "If you are concerned about immediate safety, call emergency services now.",
  "generated_at": "2026-05-21T10:00:12Z"
}
```

Incident viewer responses include `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, `Permissions-Policy: geolocation=(), microphone=(), camera=()`, `X-Frame-Options: DENY`, and a strict `Content-Security-Policy` with `frame-ancestors 'none'`. Token-protected pages, JSON, errors, and downloads include `Cache-Control: no-store`. Invalid, expired, and revoked tokens all return `404 incident_token_invalid`. App-level public viewer rate limits return `429 rate_limited` with a safe JSON error body and `Retry-After`; limiter backend failures return `503 rate_limit_unavailable`.

The Go app does not set `Strict-Transport-Security` in local/dev HTTP mode. Set HSTS at the HTTPS reverse proxy or deployment edge for production hostnames.

### `GET /i/{token}/viewer-payload`

Returns the stable, token-scoped no-account viewer payload intended for the
future web-client viewer. It is narrower than `/i/{token}/data`: it provides
incident status, latest check-in time, safe device state when present, and a
single latest shared or last reported location context when a check-in contains
both latitude and longitude. It does not expose chunk summaries, stream
summaries, encrypted bundle metadata, raw viewer tokens, token hashes, session
tokens, Authorization headers, request bodies, uploaded bytes, stored paths,
object keys, plaintext, raw keys, wrapped-key ciphertext, admin/operator
details, backend diagnostics, private deployment details, user safety
narrative, map-provider links, map API keys, or decryption material.

Response `200`:

```json
{
  "payload_version": "proofline.viewer.basic.v1",
  "incident": {
    "id": "inc_...",
    "status": "open",
    "client_label": "iphone",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z"
  },
  "latest_checkin": {
    "server_received_at": "2026-05-21T10:02:00Z",
    "safe_device_state": {
      "device_battery_percent": 82,
      "device_network": "wifi"
    }
  },
  "latest_shared_location": {
    "latitude": -37.0,
    "longitude": 145.0,
    "accuracy_meters": 20,
    "source": "checkin",
    "server_received_at": "2026-05-21T10:02:00Z",
    "freshness_status": "recent"
  },
  "warning": "If you are concerned about immediate safety, call emergency services now.",
  "generated_at": "2026-05-21T10:02:12Z"
}
```

`latest_shared_location` is omitted when no check-in contains both coordinates.
`client_reported_at` is reserved and omitted until clients submit a separate
client-reported check-in timestamp. `freshness_status` is based on the
server-received check-in timestamp and is either `recent` or `stale` for a
reported location. This is not a live-tracking contract, does not imply
emergency dispatch, and does not authorize map-provider backend integration.

Invalid, expired, and revoked tokens return the same
`404 incident_token_invalid` response as the other public viewer routes. The
route is read-only and uses the public viewer data rate-limit class.

### `GET /i/{token}/streams/{stream_id}/download`

Downloads a completed stream bundle for the token's incident. The route is read-only and never accepts a client-provided file path. Invalid, expired, and revoked tokens return `404 incident_token_invalid`.

Open and failed streams are visible only as metadata in the current viewer
summary. The current token-scoped viewer does not expose live chunk bytes or
partial stream manifests. See
[live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md).

### `GET /i/{token}/incident/download`

Downloads all completed streams for the token's incident as one encrypted evidence ZIP. Failed/open streams and legacy unstreamed chunks are omitted.

If any completed stream cannot be reconstructed, the incident bundle request fails with `409 incident_bundle_inconsistent` rather than returning a partial bundle. Invalid, expired, and revoked tokens still return `404 incident_token_invalid`.
