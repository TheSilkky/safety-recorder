# Stripe Subscription UI Boundary

Status: design and implementation plan only. This document does not implement
Stripe Checkout, Stripe Billing, Stripe Customer Portal, donations, payment
processing, or production deployment.

This document defines the intended web-client boundary for cost-recovery
subscriptions to the official hosted Proofline server.

## Funding Intent And Product Wording

Official Proofline hosted accounts should be described as **cost-recovery access
to the hosted main server**, not as a for-profit product funnel.

Proofline remains an open source project maintained by one person. The
subscription exists because the hosted server, object storage, database, email,
monitoring, backups, maintenance time, and release work cannot sustainably be
paid from the maintainer's limited personal income.

User-facing copy should be plain and honest:

- subscriptions help cover hosting and maintenance costs;
- self-hosting remains part of the open source model;
- the official hosted service is not an emergency dispatch service;
- users and trusted contacts remain responsible for contacting emergency
  services;
- one-time donations are a future separate website flow and are out of scope for
  this subscription design.

Avoid language that implies venture-backed growth, profit maximization,
emergency response guarantees, production safety certification, or charity/tax
status that has not been legally established.

## Provider Boundary

The planned backend provider is Stripe Checkout, Stripe Billing, and Stripe
Customer Portal. The web client should not collect card details, render custom
card inputs, store Stripe secret keys, verify webhooks, or decide entitlement
state from browser-returned parameters.

The browser should only:

- show pricing and cost-recovery context;
- call the Proofline server to create a Checkout Session;
- redirect the user to Stripe-hosted Checkout;
- show a safe return page after Stripe redirects back;
- call the Proofline server to create a Customer Portal Session;
- redirect the user to Stripe-hosted Customer Portal;
- display local billing state returned by the Proofline server.

The server remains the source of truth for account state and billing entitlement.
Stripe return URLs are informational only.

## Proposed Routes

Route names are proposed and should align with the server implementation when it
exists.

| Route | Purpose |
|---|---|
| `/pricing` | Public cost-recovery subscription explanation and account entry. |
| `/account/billing` | Authenticated billing state and manage-subscription page. |
| `/account/billing/success` | Return page after Checkout success redirect. |
| `/account/billing/cancelled` | Return page after Checkout cancellation redirect. |
| `/account/payment-required` | Safe account state page for `pending_payment` or restricted access. |

The registration and login pages may link to `/pricing` or
`/account/payment-required`, but they should not claim that payment processing is
implemented until the backend routes exist.

## Proposed API Calls

The web client should expect server-owned routes similar to:

```text
GET  /v1/account/billing
POST /v1/billing/checkout-sessions
POST /v1/billing/customer-portal-sessions
```

The web client must not call Stripe secret APIs directly. It may receive a
hosted redirect URL from the server and set `window.location.href` or use a safe
navigation helper to leave the app.

The web client should not implement or call the Stripe webhook route. Webhooks
are server-only.

## Billing State Model

The UI should present a small local state model from the server, not raw Stripe
objects.

Suggested display states:

| UI state | User-facing behavior |
|---|---|
| `none` | Explain that hosted server access requires a subscription. Show start-subscription action. |
| `pending_checkout` | Explain that checkout was started but not confirmed yet. Offer retry/start-over action. |
| `active` | Show active hosted access, renewal date if available, and manage-billing action. |
| `grace_period` | Show payment issue warning, date access may become restricted, and manage-billing action. |
| `restricted` | Explain hosted access is restricted until billing is resolved. Show manage-billing or start-subscription action. |
| `canceled` | Explain subscription ended and hosted access is unavailable or will end at period close. |
| `unknown` | Show safe fallback and ask the user to retry later. |

Keep messages calm and non-shaming. Billing failure states should not imply moral
failure, fraud, or emergency abandonment.

## Checkout Flow

1. User lands on `/pricing` or `/account/payment-required`.
2. User selects the hosted-server subscription action.
3. Web client calls the server to create a Checkout Session.
4. Server returns a hosted Checkout URL.
5. Browser redirects to Stripe Checkout.
6. Stripe redirects back to success or cancellation URL.
7. Return page tells the user the account is being checked and fetches
   `GET /v1/account/billing`.
8. UI reflects server-confirmed billing state only.

The success page must not say the account is active solely because the browser
returned from Stripe. A webhook may still be pending.

Suggested success wording:

```text
Payment submitted. Proofline is checking your subscription status. If your
account does not update immediately, wait a moment and refresh this page.
```

## Customer Portal Flow

1. Authenticated user opens `/account/billing`.
2. User selects Manage billing.
3. Web client calls the server to create a Customer Portal Session.
4. Server returns a hosted portal URL.
5. Browser redirects to Stripe Customer Portal.
6. Stripe redirects back to `/account/billing`.
7. Web client refreshes local billing state from the Proofline server.

The UI should explain that payment method, invoice, subscription update, and
cancellation flows happen on Stripe-hosted pages.

## Accessibility And UX Requirements

- Use clear headings and labels.
- Do not rely on color alone for active, warning, grace, or restricted states.
- Provide loading, success, cancellation, pending, and error states.
- Disable submit buttons while requests are in flight.
- Show retry actions where safe.
- Preserve prototype and emergency-reliance warnings.
- Keep cost-recovery wording visible near pricing/subscription actions.
- Keep self-hosting/open-source context visible without overwhelming the payment
  flow.

## Mock Mode

Mock mode may simulate billing states for UI and Playwright coverage, but must
remain explicit prototype data. Mock mode must not imply real payment processing
or a real Stripe account.

Recommended mock states:

- no subscription;
- pending checkout;
- active subscription;
- grace period;
- restricted/past payment issue;
- canceled.

Mock checkout and portal calls should return safe local placeholder URLs or
controlled mock state transitions only. They must not embed real Stripe Session
IDs, customer IDs, subscription IDs, secrets, or production URLs.

## Live Mode Boundary

Live billing UI must remain disabled until `open-proofline/server` documents and
implements the required billing routes. The web client should fail closed with a
safe message if billing routes return `404`, `403`, or an unsupported-provider
error.

The client should continue to treat `open-proofline/server` as the source of
truth for:

- registration modes;
- account state;
- billing state;
- browser-cookie or bearer credential mode;
- allowed origins and CSRF behavior;
- public API exposure readiness.

## Security And Privacy Rules

The web client must not log or expose:

- raw session tokens;
- browser session cookies;
- CSRF tokens;
- Authorization headers;
- request bodies;
- Stripe secret keys;
- webhook signing secrets;
- raw Stripe webhook payloads;
- Checkout Session IDs unless explicitly safe and needed for support;
- customer email addresses in public logs or issue text;
- invoice or payment method details;
- incident IDs, object keys, stored paths, uploaded bytes, plaintext, raw keys,
  raw media keys, contact private keys, wrapped-key ciphertext, private
  deployment details, or user safety data.

Do not add analytics around billing flows unless a separate privacy review
covers exactly what is collected and why.

## Out Of Scope

- One-time donations.
- GitHub Sponsors, Open Collective, Liberapay, or other support pages.
- Custom card forms.
- Stripe webhook verification.
- Server database migrations.
- Backend billing entitlement decisions.
- OAuth or JWT.
- Public admin dashboards.
- Browser decryption or key unwrapping.
- Recording/capture behavior.
- Emergency dispatch, SMS, push, or Messenger notifications.
- Claims that subscriptions make Proofline production-ready or suitable for
  emergency reliance.

## Implementation Phases

1. **Design and docs only.** Create this document and the matching server billing
   boundary document.
2. **Mock UI.** Add pricing and billing pages in mock mode with cost-recovery
   wording and safe states.
3. **API contracts.** Add typed client contracts after the server documents
   billing routes.
4. **Checkout redirect.** Call server-created Checkout Session route and redirect
   to hosted Stripe Checkout.
5. **Return pages.** Add success/cancel pages that refresh server billing state
   and avoid trusting URL parameters.
6. **Customer Portal.** Add manage-billing action that calls the server and
   redirects to hosted Stripe Customer Portal.
7. **Account state integration.** Display `pending_payment`, restricted, and
   grace-period states once backend behavior is finalized.
8. **E2E coverage.** Add Playwright flows for mock pricing, checkout start,
   return states, billing page, and restricted-account UI.

## Validation Expectations

Docs-only changes:

```bash
git diff --stat
git diff -- docs README.md CHANGELOG.md
git diff --check
```

Future frontend changes:

```bash
npm run typecheck
npm run lint
npm run test
npm run build
git diff --check
```

Run Playwright when routes, billing screens, navigation, auth/session behavior,
or visible browser flows change:

```bash
npm run test:e2e
```
