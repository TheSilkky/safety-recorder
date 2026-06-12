# One-off Codex Work Order: Add Configurable Account Registration Modes

Historical/reference-only prompt after completion.

This is a one-off implementation work order for `open-proofline/server`.

## Goal

Add configurable account registration modes while preserving Proofline's self-hostable model.

The backend should support:

* disabled public registration
* admin-created accounts only
* open public self-registration for self-hosted deployments
* a paid-registration mode placeholder that fails closed until a future subscription/payment system is implemented

This work order should implement open self-registration with automated email verification.

It should not implement payment processing, subscriptions, billing provider integration, checkout sessions, webhooks, invoices, account recovery, OAuth, JWT, browser decryption, backend decryption, notification delivery beyond registration email verification, or production hosted-service billing.

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
* SQLite migrations
* PostgreSQL migrations
* account/session repository methods
* existing account/admin tests
* existing auth/login/logout/password routes
* existing request logging and recovery middleware

If a sibling checkout of `open-proofline/web-client` exists, inspect its current README/API client docs only for coordination notes. Do not edit the web-client repo in this server work order unless explicitly requested.

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

* Add public account registration configuration.
* Add `open` public self-registration mode.
* Add a `paid` mode placeholder that fails closed until billing is implemented later.
* Add account lifecycle fields needed for registration and email verification.
* Add automated email verification for open registrations.
* Add SMTP-backed email sending or a narrow mailer abstraction with tests.
* Add local/dev-safe test mailer behavior only if explicitly documented and safe.
* Add SQLite/PostgreSQL migrations and parity tests.
* Add API, configuration, deployment, security, threat model, and code-map docs.
* Add rate-limit classification for public registration and verification routes.

Not allowed:

* Do not implement real payment/subscription integration.
* Do not add checkout sessions.
* Do not add billing webhooks.
* Do not add card/payment storage.
* Do not add OAuth.
* Do not add JWT.
* Do not add email/password recovery.
* Do not add browser cookie auth unless it is already in the branch or explicitly scoped by a separate work order.
* Do not expose `/admin` publicly.
* Do not make `/v1/admin/...` public-ready.
* Do not remove existing admin-created account flows.
* Do not remove bearer-token sessions.
* Do not add backend decryption.
* Do not add browser decryption.
* Do not add trusted-contact decryption.
* Do not add raw server-held media keys.
* Do not add key escrow.
* Do not add break-glass access.
* Do not add playable media export.
* Do not add recording/capture behavior.
* Do not add emergency-services integration.
* Do not add push/SMS/Messenger notifications.
* Do not add cloud-provider deployment automation.

## Registration modes

Add a registration mode config.

Suggested setting:

```text
SAFE_ACCOUNT_REGISTRATION_MODE=disabled|admin_only|open|paid
```

Required behavior:

| Mode         | Behavior                                                                                                                                                                                 |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `disabled`   | No public self-registration. Existing accounts and sessions continue to work.                                                                                                            |
| `admin_only` | Public registration is disabled; admins can still create accounts through existing admin account routes.                                                                                 |
| `open`       | Public self-registration is enabled, using email verification before login/activation.                                                                                                   |
| `paid`       | Reserved for future hosted-service payment-gated registration. In this work order, it must fail closed with a safe response because payment/subscription support is not implemented yet. |

Default:

```text
SAFE_ACCOUNT_REGISTRATION_MODE=disabled
```

Do not make open registration the default.

If `paid` is selected before billing is implemented, registration requests must not create an active account. Prefer returning a safe `503 registration_payment_unavailable` or repository-consistent equivalent. Document the exact behavior.

## Account lifecycle

Add account lifecycle fields if they do not already exist.

Suggested account states:

```text
pending_email_verification
active
disabled
suspended
pending_payment
```

For this work order:

* admin-created accounts should preserve current behavior and remain active by default
* open self-registered accounts should start as `pending_email_verification`
* verified self-registered accounts should become `active`
* `paid` registration should not activate accounts
* if `pending_payment` is added, it is reserved for future payment integration and should not be reachable as a completed registration state in this work order

Add optional email fields if missing:

* normalized email
* email verification state or timestamp
* created/updated timestamps if consistent with existing models

Treat email addresses as sensitive account data. Do not expose email addresses outside authenticated own-account/admin-safe contexts.

## Public registration route

Add:

```http
POST /v1/auth/register
```

Authentication:

* unauthenticated
* route-class rate-limited
* available only when registration mode allows it

Suggested request:

```json
{
  "username": "new-user",
  "email": "user@example.com",
  "password": "long local password"
}
```

Validation:

* normalize username using existing account username rules
* validate password using existing password policy
* normalize email case and whitespace
* reject invalid email syntax
* reject oversized fields
* avoid storing raw request bodies
* avoid logging email/password/request body

Response for `open` mode:

Prefer a generic `202 Accepted` response:

```json
{
  "status": "verification_required",
  "message": "If registration can be completed, a verification email will be sent."
}
```

Avoid account enumeration through email or username duplicates where practical.

If the codebase already uses more specific validation errors for username conflicts, document the tradeoff and keep email enumeration protected.

The route must not return a raw session token.

The route must not log raw passwords, email verification tokens, request bodies, or email contents.

## Email verification tokens

Add email verification tokens.

Requirements:

* generate a high-entropy random token
* store only a hash or HMAC-derived token digest
* never store the raw token
* never log the raw token
* single-use token
* configurable TTL, default `24h`
* consumed tokens cannot be reused
* expired tokens fail closed
* token purpose is explicit, for example `email_verification`
* tokens are account-bound
* repeated verification of an already active account returns a safe idempotent or generic response without exposing token internals

Suggested config:

```text
SAFE_EMAIL_VERIFICATION_TTL=24h
SAFE_PUBLIC_WEB_ORIGIN=http://127.0.0.1:5173
```

The email verification link should preferably point to the web client, not directly to an API endpoint.

Preferred link form:

```text
{SAFE_PUBLIC_WEB_ORIGIN}/verify-email#token=<raw-token>
```

The fragment keeps the token out of HTTP requests to the web server. The future web client can read the fragment and submit the token to the API in a JSON body.

Do not put verification tokens in server logs, metrics labels, docs examples, tests, or PR bodies.

## Email verification route

Add:

```http
POST /v1/auth/email/verify
```

Suggested request:

```json
{
  "token": "verification-token-from-email"
}
```

Behavior:

* accepts raw token only in JSON body
* hashes/HMACs the token and looks up the stored verification record
* rejects expired, consumed, wrong-purpose, or missing tokens
* marks the email/account verified
* activates the account if it is pending email verification
* does not create a session automatically in the first implementation unless explicitly justified and tested
* returns a safe success response

Suggested response:

```json
{
  "status": "verified"
}
```

Invalid/expired token response should be generic:

```json
{
  "error": {
    "code": "verification_token_invalid",
    "message": "verification token is invalid or expired"
  }
}
```

Do not reveal whether an account exists, which email address is attached, or whether the token was expired versus already used unless current repository patterns strongly require that detail.

## Verification email sending

Add an email sending abstraction.

Suggested package or boundary:

```text
internal/email
```

or a name consistent with current package layout.

Suggested mailer backends:

```text
none
smtp
```

Optional local/dev test backend may be added if explicitly safe and documented:

```text
stdout_dev
file_dev
```

If a dev backend prints or writes verification links, it must be clearly marked unsafe for production and must not be enabled by default. Avoid adding a dev backend if it risks normalizing token logging.

Suggested config:

```text
SAFE_EMAIL_BACKEND=none|smtp
SAFE_SMTP_HOST=
SAFE_SMTP_PORT=
SAFE_SMTP_USERNAME=
SAFE_SMTP_PASSWORD=
SAFE_SMTP_FROM=
SAFE_SMTP_STARTTLS=required|opportunistic|disabled
SAFE_SMTP_TIMEOUT=10s
SAFE_PUBLIC_WEB_ORIGIN=
```

Startup behavior:

* if `SAFE_ACCOUNT_REGISTRATION_MODE=open` and email verification is required, startup should fail closed unless a usable email backend and public web origin are configured
* `SAFE_EMAIL_BACKEND=none` must not allow public open registration with required email verification
* secrets such as SMTP passwords must never be logged

Email content:

* plain text is sufficient for first implementation
* include the verification link
* include a short expiry note
* include "If you did not create this account, ignore this email."
* do not include passwords
* do not include session tokens
* do not include incident details
* do not include private deployment details
* do not include raw keys or safety data

## Email verification required flag

Prefer requiring email verification for open registration by default.

Optional config may be added:

```text
SAFE_ACCOUNT_EMAIL_VERIFICATION_REQUIRED=true|false
```

Default:

```text
true
```

If `false` is supported, document it as self-host/dev-only and not suitable for public hosted deployments. In this mode, open registration may create active accounts immediately, but the route must still be rate-limited and must not return a raw session token by default.

If supporting this flag makes the work too large, keep verification always required for open mode and defer no-verification self-host mode to a later issue.

## Login behavior

Update login behavior to account for account state.

Requirements:

* `active` accounts can log in
* `pending_email_verification` accounts cannot log in
* `disabled`, `suspended`, and `pending_payment` accounts cannot log in
* failures should use safe generic responses where practical
* do not leak whether an email address exists
* do not log passwords or account state internals in unsafe logs

If a pending account enters correct credentials, it is acceptable to return a safe `email_verification_required` code if this matches product UX and does not create account enumeration risk beyond a valid credential holder.

Existing admin-created active accounts must continue to log in.

Existing password-change and admin reset flows must remain compatible with account states.

## Admin-created accounts

Preserve existing admin-created account behavior.

Requirements:

* existing `POST /v1/admin/accounts` remains admin-only
* admin-created accounts are `active` by default unless the current code or docs say otherwise
* admin-created account creation must not require public email verification in this work order
* if email fields are added to account records, admin-created accounts may omit email unless explicitly scoped
* admin password reset/session revocation behavior remains unchanged

Do not expose admin account creation publicly.

## Paid mode placeholder

Add `paid` as a recognized registration mode but do not implement payment.

Behavior:

* startup accepts `SAFE_ACCOUNT_REGISTRATION_MODE=paid`
* `POST /v1/auth/register` fails closed with a safe response such as `503 registration_payment_unavailable`
* no active account is created through paid mode
* no checkout session is created
* no billing provider is contacted
* docs state that paid registration is reserved for a future work order

Do not add fake payment success flows.

Do not add Stripe or other payment-provider dependencies in this work order.

Do not store payment provider IDs or billing states unless needed as a placeholder for account lifecycle. If placeholder billing/account state fields are added, clearly document them as inactive until future payment integration.

## Rate limiting and abuse controls

Public registration and verification routes must be rate-limited.

Route classes should use safe keys only.

Limiter keys and logs must not include:

* raw email addresses
* raw usernames, if avoidable
* raw verification tokens
* raw session tokens
* Authorization headers
* request bodies
* uploaded bytes
* plaintext
* raw keys
* wrapped-key ciphertext
* stored paths
* object keys
* private deployment details
* user safety data

Suggested route classes:

* `auth_register`
* `auth_email_verify`

Use existing main API rate-limit infrastructure if available.

Consider adding stronger default limits for registration than for normal authenticated routes.

## Data model

Add schema changes carefully.

Potential new fields on accounts:

* `email_normalized`
* `email_verified_at`
* `account_state`
* `billing_state`, optional placeholder only if justified
* `created_at`
* `updated_at`

Potential new table:

```text
account_verification_tokens
```

Suggested fields:

* `id`
* `account_id`
* `purpose`
* `token_hash`
* `expires_at`
* `consumed_at`
* `created_at`

Requirements:

* SQLite and PostgreSQL migrations
* unique constraints where needed
* repository methods through existing metadata boundaries
* no raw token storage
* no raw password storage
* no plaintext verification secrets
* safe handling of duplicate registration attempts
* safe cleanup path or future pruning note for expired/consumed verification tokens

Do not add schema fields that imply payment is complete in this work order unless they are clearly inactive placeholders.

## Tests

Add focused tests for:

### Config

* default registration mode is disabled
* invalid registration mode fails startup/config validation
* `open` mode with required email verification fails startup when email backend is missing
* SMTP config validates required fields without logging secrets
* `paid` mode is accepted but fails closed at registration

### Registration route

* disabled/admin-only mode rejects public registration
* open mode accepts valid registration and creates pending-email-verification account
* open registration does not return a raw session token
* duplicate email/username behavior is safe and documented
* invalid username/password/email returns safe validation errors
* registration route is rate-limit classified
* registration response/logs do not expose password, verification token, request body, SMTP password, or private details

### Email verification

* verification token is generated with high entropy
* only token hash/HMAC is stored
* verification email is sent through a fake test mailer
* verification token activates pending account
* expired token fails
* consumed token cannot be reused
* wrong-purpose token fails
* invalid token returns safe generic error
* token is accepted only in JSON body, not path
* logs/errors do not expose the raw token

### Login/account state

* pending-email-verification account cannot log in
* verified active account can log in
* disabled/suspended/pending-payment accounts cannot log in
* admin-created active accounts preserve existing login behavior
* existing bearer login response still works for active accounts

### Database parity

* SQLite registration and verification behavior
* PostgreSQL registration and verification behavior when `SAFE_POSTGRES_TEST_DSN` is available or PostgreSQL parity tests exist
* migrations preserve existing accounts
* existing accounts receive safe default account state

### Admin compatibility

* `POST /v1/admin/accounts` still requires admin
* admin-created accounts are active by default
* admin password reset/session revocation remains unchanged
* `/admin` bootstrap flow remains unchanged

## Documentation updates

Update:

* `CHANGELOG.md`
* `README.md`
* `docs/api.md`
* `docs/configuration.md`
* `docs/deployment.md`
* `docs/security-model.md`
* `docs/threat-model.md`
* `docs/v1-access-control.md`
* `docs/code-map.md`
* `docs/development.md`, if validation changes
* `SECURITY.md`, if reporting guidance changes

Docs must state:

* self-hosted deployments can keep registration disabled/admin-only or enable open self-registration
* public hosted deployments can later use paid registration mode, but real payment support is not implemented in this work order
* `SAFE_ACCOUNT_REGISTRATION_MODE=paid` currently fails closed
* open self-registration requires email verification by default
* automated email verification requires a configured email backend and public web origin
* verification tokens are secret, single-use, time-limited, and stored only as hashes/HMACs
* registration does not return a session token before verification
* public registration does not imply production readiness
* this does not add OAuth, JWT, password reset, payment processing, browser decryption, backend decryption, key escrow, recording, notifications, or emergency-services integration

## Web-client coordination notes

Do not edit `open-proofline/web-client` in this server work order unless explicitly requested.

Document expected future web-client follow-up:

* add registration page
* add email verification page that reads token from URL fragment and POSTs it in JSON body
* add registration mode/status copy
* add payment-required placeholder UI only when backend reports paid registration is unavailable or reserved
* remove any wording that says account registration is not backend-supported after this is released
* do not store verification token after use
* do not log verification tokens
* do not imply payment integration exists before it does

## Suggested implementation phases

Prefer small commits in this order:

1. Config parsing and docs skeleton for registration modes.
2. Account state/email fields and migrations.
3. Verification token repository methods and tests.
4. Mailer abstraction and fake test mailer.
5. Public registration route.
6. Email verification route.
7. Login/account-state enforcement.
8. Rate-limit classification.
9. SQLite/PostgreSQL parity tests.
10. Documentation and changelog updates.

If this becomes too large for one reviewable PR, stop after config/design/migrations or after open-registration core behavior and create follow-up issues. Do not push a giant risky auth/account lifecycle PR just to complete the work order.

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

Because this touches auth/account lifecycle, public unauthenticated routes, email config, migrations, and deployment-sensitive behavior, inspect current Compose smoke docs and run relevant local Compose smoke tests if Docker is available.

Do not run smoke tests against production services.

Do not use real SMTP credentials in tests.

Do not use real deployment credentials.

## Pull request

Open a PR against `develop`.

Suggested PR title:

```text
auth: add configurable account registration
```

PR body must include:

```md
## Summary

- Added configurable account registration modes for disabled/admin-only/open/paid deployments.
- Implemented open self-registration with email verification.
- Added paid registration as a fail-closed placeholder for future billing integration.
- Preserved existing admin-created account flows and bearer-token sessions.
- Updated API, configuration, security, threat model, access-control, and deployment docs.

## Validation

- [ ] `gofmt -w ./cmd ./internal ./migrations`
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `git diff --check`
- [ ] PostgreSQL parity tests, if relevant
- [ ] Compose smoke tests, if available/relevant

## Security and scope

- Public registration is disabled by default.
- Open registration requires email verification by default.
- Verification tokens are stored only as hashes/HMACs.
- Verification tokens are single-use and time-limited.
- Registration does not return a raw session token before verification.
- Paid registration is a fail-closed placeholder only; no payment provider was added.
- Existing admin-created account flows remain available.
- Existing bearer-token login/session behavior remains available for active accounts.
- `/admin` remains private-only.
- `/v1/admin/...` remains admin-only and not public-ready.
- No OAuth/JWT, password reset, real billing, backend decryption, browser decryption, trusted-contact decryption, raw server-held media keys, key escrow, break-glass access, playable media export, recording/capture behavior, push/SMS/Messenger notifications, or emergency-services integration was added.
- No raw passwords, verification tokens, session tokens, Authorization headers, request bodies, SMTP secrets, plaintext, raw keys, wrapped-key ciphertext, stored paths, object keys, private deployment details, or user safety data are logged or exposed.
```

## Final output

Report:

1. files changed
2. config added
3. registration modes implemented
4. public registration route behavior
5. paid placeholder behavior
6. email verification route behavior
7. mailer backend behavior
8. account state behavior
9. SQLite/PostgreSQL migration status
10. tests added
11. validation commands run
12. docs updated
13. web-client follow-up needed
