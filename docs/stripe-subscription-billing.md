# Stripe Subscription Billing Boundary

Status: design and implementation plan only. This document does not implement
Stripe integration, payment-gated access, billing webhooks, donations, or public
production deployment.

This document defines the intended Stripe Checkout, Stripe Billing, and Stripe
Customer Portal boundary for the official hosted Proofline server.

## Project Funding Intent

Official Proofline hosted accounts are intended as **cost-recovery subscription
access** for the shared main server, not as a for-profit product strategy.
Proofline remains an open source project maintained by one person across the
planned server, web-client, iOS-client, Android-client, and protocol surfaces.

The subscription requirement for the official hosted service exists because the
server, object storage, database, email delivery, monitoring, backups,
maintenance time, and release work cannot sustainably be paid from the
maintainer's limited personal income.

This boundary should be reflected in product wording and implementation:

- subscriptions fund hosted server and maintenance costs;
- self-hosting remains part of the open source model;
- the hosted service should not be described as an emergency-response service;
- users and trusted contacts remain responsible for contacting emergency
  services;
- donations may be added later as a separate one-time support path, but donations
  are out of scope for this design.

## Provider Decision

Use Stripe hosted surfaces first:

- Stripe Checkout for starting subscription signup or subscription activation;
- Stripe Billing for subscription lifecycle state;
- Stripe Customer Portal for self-service payment method, invoice, subscription,
  and cancellation management.

The backend should not collect card details, embed custom card forms, or attempt
to become a payment UI. The web client should redirect to Stripe-hosted Checkout
or Customer Portal sessions created by the backend.

Stripe-hosted redirect flows are preferred for the first implementation because
they minimize frontend payment surface area and keep card handling outside the
Proofline browser app. Stripe webhook events, not checkout return URLs, are the
source of truth for account entitlement changes.

## Design Principles

- Keep billing provider integration isolated from account, incident, upload,
  storage, and key-custody logic.
- Treat Stripe identifiers as provider metadata, not authorization by themselves.
- Grant or revoke hosted-server access only after verified webhook processing or
  a deliberate private operator reconciliation path.
- Store webhook event IDs and process events idempotently.
- Keep raw Stripe API keys, webhook signing secrets, raw webhook payloads,
  checkout URLs, customer email addresses, billing metadata, session tokens,
  request bodies, uploaded bytes, object keys, plaintext, raw keys, and user
  safety data out of public logs, public issue text, tests, and docs.
- Preserve the current ciphertext-only backend posture. Billing must not imply
  backend decryption, key escrow, playable export, notifications, or emergency
  dispatch.
- Preserve public/private listener separation and do not expose `/admin`,
  `/admin/api/...`, private diagnostics, or operator-only routes through billing
  work.

## Account Lifecycle Model

The current server supports local accounts, email verification, account state,
and a fail-closed `paid` registration placeholder. A future hosted billing
implementation can extend that placeholder into a real Stripe-backed lifecycle.

Recommended hosted lifecycle:

```text
register
  -> pending_email_verification
  -> email verified
  -> pending_payment
  -> Stripe Checkout subscription completed
  -> active
```

Self-hosted deployments may keep disabled, admin-only, or open registration
without enabling Stripe. The hosted Official Proofline deployment can require an
active billing entitlement before account access to the main server is enabled.

Do not rely on the user's browser returning to a `success_url` to activate the
account. Return URLs may be used for user feedback only. The server should update
local account or entitlement state only after verified webhook processing.

## Server Responsibilities

The server owns:

- Stripe configuration loading and validation;
- provider-secret storage through environment variables or secret files;
- creating Checkout Sessions for authenticated or verified pending accounts;
- creating Customer Portal Sessions for authenticated accounts with a billing
  customer;
- verifying Stripe webhook signatures against the raw request body;
- storing webhook event IDs and processing events idempotently;
- mapping Stripe customers and subscriptions to local accounts;
- maintaining local billing entitlement state;
- exposing safe account billing state to the authenticated web client;
- providing private operator reconciliation tooling if a webhook is missed or
  delayed.

The server does not own:

- card entry UI;
- custom payment form rendering;
- hosted donation flow in this milestone;
- tax advice or public financial claims;
- emergency notifications or emergency-services contact;
- browser decryption, backend decryption, key escrow, break-glass access, or
  playable media export.

## Proposed Configuration

Configuration names are illustrative and should be aligned with current config
style during implementation. TOML should be the primary documented shape, with
`SAFE_*` environment variables retained as compatibility inputs and deployment
overrides.

Proposed TOML shape:

```toml
[account_registration]
mode = "paid"

[billing]
provider = "stripe"
grace_period = "72h"

[billing.stripe]
secret_key_file = "/run/secrets/proofline-stripe-secret-key"
webhook_secret_file = "/run/secrets/proofline-stripe-webhook-secret"
price_id = "price_..."
success_url = "https://app.example.invalid/account/billing/success"
cancel_url = "https://app.example.invalid/account/billing/cancelled"
portal_return_url = "https://app.example.invalid/account/billing"
api_version = ""
```

Proposed environment override mapping:

```text
SAFE_BILLING_PROVIDER=disabled|stripe
SAFE_STRIPE_SECRET_KEY_FILE=/run/secrets/stripe-secret-key
SAFE_STRIPE_WEBHOOK_SECRET_FILE=/run/secrets/stripe-webhook-secret
SAFE_STRIPE_PRICE_ID=price_...
SAFE_STRIPE_SUCCESS_URL=https://app.example/account/billing/success
SAFE_STRIPE_CANCEL_URL=https://app.example/account/billing/cancelled
SAFE_STRIPE_PORTAL_RETURN_URL=https://app.example/account/billing
SAFE_BILLING_GRACE_PERIOD=...
SAFE_STRIPE_API_VERSION=...
```

If `stripe-go` is used, match the Stripe webhook endpoint API version to the
version pinned by the selected `stripe-go` release unless a deliberate API
version override is designed and tested. Keep API-version changes explicit in
tests and release notes because webhook event shapes and generated SDK types can
otherwise drift apart.

Defaults must fail closed:

- billing disabled unless explicitly configured;
- paid registration unavailable unless Stripe is configured and passes local
  startup validation;
- missing webhook secret prevents webhook endpoint activation;
- missing price ID prevents checkout session creation.

## Proposed HTTP Routes

Route shapes are proposed and should be finalized during implementation.

### `GET /v1/account/billing`

Authenticated account route. Returns safe local billing state only.

Example response:

```json
{
  "billing": {
    "provider": "stripe",
    "entitlement_state": "active",
    "subscription_state": "active",
    "current_period_end": "2026-07-01T00:00:00Z",
    "cancel_at_period_end": false,
    "portal_available": true
  }
}
```

Do not return raw Stripe API objects, webhook payloads, payment method details,
invoice PDFs unless deliberately scoped, or provider secrets.

### `POST /v1/billing/checkout-sessions`

Authenticated account route or verified pending-account route, depending on the
final account lifecycle. Creates a Stripe Checkout Session for the configured
subscription price and returns only the redirect URL or session ID needed by the
web client.

The server should attach safe provider metadata such as an opaque billing
correlation ID mapped to the local account server-side where useful for webhook
reconciliation, but it must not expose internal deployment details or user safety
data to Stripe metadata.

### `POST /v1/billing/customer-portal-sessions`

Authenticated account route. Creates a Stripe Customer Portal Session for an
account already mapped to a Stripe customer and returns the hosted redirect URL.

### `POST /v1/billing/webhooks/stripe`

Unauthenticated HTTP route protected by Stripe webhook signature verification,
not account authentication. It must read and verify the raw request body, reject
invalid signatures, store the provider event ID, and process events
idempotently.

The webhook route may need public reachability for Stripe delivery, but exposing
that single signed webhook path must not become approval to expose the whole
main `/v1` tree or any private-admin route group. It needs a dedicated
deployment review, strict body limits, an appropriate rate-limit class, and safe
logging. It must not log raw event bodies, customer email addresses, payment
method details, subscription metadata, request bodies, headers, or secrets.

## Proposed Data Model

Provider-neutral table names are preferred so a future non-Stripe provider can be
added without rewriting account state.

```text
billing_customers
  id
  account_id
  provider
  provider_customer_id
  created_at
  updated_at

billing_subscriptions
  id
  account_id
  provider
  provider_subscription_id
  provider_price_id
  subscription_state
  entitlement_state
  current_period_start
  current_period_end
  cancel_at_period_end
  created_at
  updated_at

billing_events
  id
  provider
  provider_event_id
  event_type
  provider_api_version
  provider_created_at
  livemode
  received_at
  processed_at
  processing_error_code
```

Do not store raw payment card details. Do not store raw webhook payloads unless a
separate encrypted, access-controlled audit store is explicitly designed.
Provider event IDs and safe state summaries are enough for the first milestone.

## Entitlement Mapping

The local entitlement state should be simple and conservative.

Suggested local states:

| Entitlement state | Meaning |
|---|---|
| `none` | No active hosted-server entitlement. |
| `pending_checkout` | Checkout started but no verified subscription activation yet. |
| `active` | Hosted-server access is allowed. |
| `grace_period` | Temporarily allowed while payment recovery is in progress. |
| `restricted` | Login or feature access should be limited until billing is resolved. |
| `canceled` | Subscription ended or access should no longer be provided. |

Initial Stripe mapping should be explicit and documented in tests. For example,
`active` and `trialing` can grant access, while `canceled`, `unpaid`,
`incomplete_expired`, and `paused` should generally restrict access. `past_due`
requires a product decision: either a short configured grace period or immediate
restriction. If a grace period is used, it must be local, explicit, and visible
in account billing state. Do not treat `active` as proof that every outstanding
invoice is paid; the initial access decision should require the expected
subscription state and a successful payment signal such as `invoice.paid`.

## Webhook Event Handling

Initial event handling should cover the minimum set needed to keep local state
accurate:

- checkout completion for initial customer/subscription correlation;
- paid invoice events for initial access provisioning and renewal confirmation;
- subscription created/updated/deleted events for lifecycle state;
- payment failed events for entitlement updates and recovery state.

Implementation should verify event signatures, deduplicate by provider event ID,
prefer Stripe's documented Go webhook signature verification path over custom
signature code, and process updates transactionally. Events such as
`invoice.payment_action_required` and `invoice.finalization_failed` should at
least be safely recorded and surfaced to operator reconciliation before live
hosted billing is relied on. Unknown event types should be acknowledged only
after safe recording or ignored with a safe count/log category, depending on
Stripe retry behavior and implementation choice.

## Private Operator Reconciliation

Billing integrations fail in boring but expensive ways: webhook retries can be
delayed, provider metadata can be wrong, and accounts can be migrated. Add a
future private operator path for safe reconciliation before relying on hosted
billing in production.

A private operator command may:

- show count-only billing state summaries;
- reconcile one account by local account ID and provider subscription ID;
- retry failed billing event processing by provider event ID;
- mark an account restricted only through explicit operator action.

It must not print raw provider secrets, raw webhook payloads, payment method
information, full customer emails unless deliberately required, session tokens,
request bodies, object keys, private deployment details, or user safety data.

## Web-Client Contract

The web client should:

- show cost-recovery subscription wording, not for-profit business wording;
- call the server to create Checkout and Portal sessions;
- redirect to hosted Stripe pages;
- display local server billing state from `GET /v1/account/billing`;
- treat return URLs as informational only;
- never store Stripe secret keys or webhook secrets;
- never decide entitlement state from browser-visible Stripe parameters.

The web-client design should be documented separately in
`open-proofline/web-client` when that companion-repository work is explicitly
scoped.

## Donations

One-time donations for the main website are a future separate flow. They should
not be mixed into subscription access control.

Future donation design should define whether donations are handled by Stripe
Payment Links, Checkout one-time payments, GitHub Sponsors, Open Collective,
Liberapay, or another support path. Donation receipts, public supporter wording,
and tax language need separate review. Donations must not unlock account access
unless a separate sponsorship entitlement is deliberately designed.

## Security And Privacy Review Checklist

Before implementation or PR review, verify:

- Stripe secret keys and webhook secrets are secret-file capable and never logged;
- webhook signature verification uses the raw request body;
- event processing is idempotent;
- provider metadata does not include incident IDs, viewer tokens, object keys,
  private deployment details, user safety data, plaintext, raw keys, or wrapped
  key ciphertext;
- public account routes do not expose account existence through billing flows;
- checkout and portal routes require the intended account state;
- public/private listener and admin route separation is preserved;
- docs do not claim production readiness;
- user-facing copy explains cost recovery and open source maintenance without
  implying emergency response guarantees.

## Implementation Phases

1. **Design and docs only.** Create this server document. Create a matching
   web-client document only in the companion repository and only when that work
   is explicitly scoped.
2. **Configuration scaffold.** Add disabled-by-default billing config and safe
   startup validation without calling Stripe.
3. **Data model.** Add billing customer, subscription, and event tables with
   SQLite/PostgreSQL parity.
4. **Webhook receiver.** Add signature-verified Stripe webhook ingestion and
   idempotent event recording.
5. **Checkout and portal routes.** Add account-authenticated routes for Checkout
   and Customer Portal sessions.
6. **Account lifecycle integration.** Convert paid-registration placeholder into
   a Stripe-backed hosted lifecycle when explicitly configured.
7. **Operator reconciliation.** Add private count-safe billing reconciliation
   commands or routes.
8. **Web-client integration.** Add pricing, subscription, checkout return, and
   billing portal UI after server routes are implemented.

## Validation Expectations

Docs-only changes:

```bash
git diff --stat
git diff -- docs README.md CHANGELOG.md
git diff --check
```

Future Go changes:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

Run Compose smoke tests when billing changes touch listener topology,
configuration loading, deployment docs, or public/private route exposure.
