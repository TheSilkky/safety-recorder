# Notification Boundary

This document defines the future notification boundary for Proofline trusted
contacts, missed safety checks, and no-account viewer-token link delivery. It
is a design document only. It does not implement SMS, push notifications,
email alerts, Messenger or chat delivery, provider integrations, notification
outbox tables, background jobs, API routes, schema changes, client code,
emergency-services integration, live tracking, or real-time location.

## Summary

Proofline does not currently send trusted-contact alerts or safety-check
notifications. The backend sends email only for implemented account flows such
as registration email verification and second-factor email challenges. Those
email flows are account setup/authentication flows, not incident notification
delivery.

Future notification work must be explicit because notification messages can
carry sensitive user-safety context, token-bearing links, contact identifiers,
and operational metadata through third-party providers. A notification should
help an authorized recipient review the situation. It must not claim that
Proofline contacted emergency services, that help is on the way, that live
tracking is guaranteed, or that real-time location is available.

## Current Implementation Boundary

Implemented today:

- local account/session authentication
- disabled-by-default registration email verification
- email challenge second-factor setup and verification
- owner-scoped trusted-contact relationship metadata
- owner-scoped contact public-key metadata
- owner-scoped sharing grants and wrapped-key records
- signed-in trusted-contact wrapped-key reads under active relationship, key,
  grant, and record filters
- owner-created no-account viewer tokens and read-only viewer routes

Not implemented today:

- trusted-contact notification delivery
- missed-check-in notification delivery
- SMS, push, Messenger, chat, or email-alert providers
- notification outbox, queue, retry worker, or delivery receipts
- notification preferences or opt-out enforcement
- emergency-services dispatch
- guaranteed live tracking or guaranteed real-time location
- notification-driven trusted-contact incident reads
- notification-driven key release, browser decryption, backend decryption, or
  playable export

Optional `incident_mode`, `capture_profile`, `escalation_policy`, and
`sharing_state` metadata fields remain labels unless a later issue explicitly
implements policy behavior. A value such as `trusted_contacts_on_start` or
`trusted_contacts_on_missed_checkin` must not silently send a message.

## Notification Modes To Keep Separate

Future designs must distinguish:

| Mode | Purpose | Boundary |
|---|---|---|
| Account setup email | Verify account registration or second-factor setup. | Implemented for account/auth flows only; not an incident alert channel. |
| No-account viewer-token link delivery | Give a recipient a bearer link to a single read-only incident viewer. | The link is not an account relationship and does not prove trusted-contact identity. |
| Account-based trusted-contact alert | Notify an accepted trusted contact that a grant, safety-check event, or review request exists. | Requires account relationship, authorization, preferences, and audit design before implementation. |
| Missed safety-check notification | Tell selected contacts that a configured check-in was missed after policy and grace-period evaluation. | Must handle false positives, false negatives, cancellation, retry, and suppression. |
| Break-glass or key-release notification | Inform contacts or operators about a separately accepted key-access policy event. | Must follow [break-glass key access](break-glass-key-access.md) and must not release keys by message text. |

Notification does not by itself grant access. Access must come from an
existing viewer token, account session, grant, wrapped-key policy, or other
explicit future authorization mechanism.

## No-Account Viewer Links Versus Trusted Contacts

A no-account viewer token is a bearer secret for one incident. Anyone with the
valid token can use the token-scoped read-only viewer routes until the token
expires or is revoked. A viewer-token recipient is not automatically a trusted
contact, does not get an account relationship, does not get wrapped-key access,
does not get incident write access, and does not become eligible for future
trusted-contact grants.

An account-based trusted contact is a local account that has accepted a
relationship and may later receive grant-scoped access. A trusted-contact
notification may tell that account to sign in and review an authorized item,
but the message should not carry enough authority by itself to bypass account,
relationship, grant, key, or wrapped-key checks.

Future messages that include no-account links must be treated as more
sensitive than messages that merely ask a trusted contact to sign in. Prefer
account-based review links when the recipient already has an account and a
relationship. Use no-account viewer links only when the user explicitly chose a
bearer-link sharing flow.

## Token-Link Risks And Redaction

Token-bearing links can leak through:

- provider message bodies and delivery logs
- bounce messages
- spam or abuse review tooling
- push-notification previews
- mobile lock screens
- browser history
- referrer headers
- analytics, crash reports, and monitoring labels
- support exports or user screenshots
- copied messages in chat or email clients

Future notification code must keep raw viewer tokens, incident tokens, session
tokens, Authorization headers, request bodies, uploaded bytes, plaintext, raw
keys, wrapped-key ciphertext, stored paths, object keys, private deployment
details, and user safety narratives out of logs, tests, public issue text, PR
bodies, and provider-visible diagnostics.

Safe token-link handling should follow these rules:

- Do not put raw viewer tokens in provider event IDs, metric labels, log
  fields, retry keys, idempotency keys, or audit reason text.
- Prefer short-lived viewer-token links for notification delivery unless the
  user explicitly chose a longer-lived public-link sharing flow.
- Prefer web-client fragment-token links for future no-account viewer links, as
  documented in [web-client viewer routing](web-client-viewer-routing.md), so
  the token is not sent in the HTTP request path or query string.
- Keep current `/i/{token}` and `/e/{token}` server routes treated as
  token-bearing prototype/local compatibility paths.
- Redact provider callbacks, webhook payloads, bounce details, and delivery
  status before logging or exposing them to operators.
- Use only controlled status categories in audit and support surfaces.

## Message Content Boundary

Notification templates should be sparse and provider-safe. They should include
only the minimum context needed for the recipient to understand what action to
take.

Allowed message shape:

- product name
- non-sensitive notification type, such as safety check missed or incident
  review requested
- recipient action, such as sign in or open the provided viewer link
- short emergency guidance that tells the recipient to use their own judgment
- generic expiry or revocation warning when applicable

Forbidden message content:

- raw tokens except in the one user-visible link when the user explicitly
  chose a bearer-link flow
- full GPS coordinates, speed, heading, or route history
- uploaded bytes, plaintext, transcript, media descriptions, notes, or user
  safety narratives
- raw keys, wrapped-key ciphertext, contact private keys, or key shares
- private deployment details, internal hostnames, object keys, stored paths, or
  provider secrets
- claims that emergency services were contacted
- claims that the location is live or real-time

Suggested future safety-check wording:

```text
A Proofline safety check was missed. Review the incident, try to contact the user, and call emergency services if you believe there is immediate danger.
```

This wording is guidance for the recipient. It is not emergency dispatch.

## Provider Selection And Secrets

Provider choice is deployment-sensitive. Future implementation must not
hard-code one delivery provider into the product boundary or public docs as a
required service. SMS, push, Messenger, chat, and email-alert providers have
different metadata retention, logging, abuse review, geographic, cost, and
delivery-failure behavior.

Provider configuration must treat these as secret or private:

- API keys, webhook signing secrets, client secrets, private keys, and tokens
- sender identities when they reveal private deployment details
- callback URLs, internal hostnames, private tunnel endpoints, and tenant IDs
- provider event payloads that contain recipient addresses, message bodies, or
  token-bearing links

Future provider credentials should use secret files or secret-management
bindings rather than public examples with real values. Startup and worker logs
must use safe error categories instead of raw provider errors when those errors
can contain message bodies, recipient identifiers, private endpoints, or
provider secrets.

## Retry, Suppression, And Deduplication

Notification delivery is unreliable. Future implementations should assume
providers can delay, duplicate, reorder, throttle, or silently drop messages.

Retry policy must define:

- which failures are retryable
- maximum attempts
- backoff schedule
- total retry window
- suppression window for duplicate events
- whether a policy event can produce one message or several messages
- safe deduplication key shape
- terminal failure behavior

Suppression should prevent accidental message storms during repeated
check-in failures, app reconnect loops, provider outages, or restore/replay
after backup recovery. Suppression state must not include raw tokens, message
bodies, private provider payloads, user safety data, or precise location
values.

Retries must not widen access. Retrying a message must not create a new viewer
token, extend a token, release a wrapped key, or change policy state unless a
separate reviewed state transition allows it.

## Opt-Out, Cancellation, And Preferences

Future notification preferences should be explicit for both account owners and
trusted contacts.

Design questions before implementation:

- Can a trusted contact opt out of all notifications, only non-urgent
  notifications, or specific channels?
- Can an account owner override a contact's channel choice for urgent
  incidents?
- What happens when a contact opts out but still has an active sharing grant?
- Can a user cancel a delayed emergency or missed-check-in notification during
  a grace period?
- Which actor can suppress repeated notifications for one incident?
- How are opt-out, cancellation, and suppression audited without storing safety
  narratives?

Opt-out and cancellation must not silently revoke existing grants unless the
user explicitly chooses that behavior. Conversely, an active grant must not
silently force notification delivery to a contact that has opted out under the
accepted policy.

## Rate Limits And Abuse Controls

Future notification routes or workers need rate limits separate from ordinary
viewer and account traffic.

Controls should cover:

- owner-triggered notification creation
- safety-check missed-event processing
- trusted-contact review requests
- provider retry loops
- token-link creation for notification messages
- provider callback or webhook processing
- abuse reports and suppression actions

Rate-limit keys must be server-controlled and low-cardinality. They must not
include raw email addresses, phone numbers, usernames, raw tokens, incident
IDs, message bodies, object keys, stored paths, private endpoints, or location
values. Logs should record only safe route class, controlled outcome, and safe
counts where needed.

## Audit Expectations

Audit records should let owners, trusted contacts, and operators understand
what happened without becoming a second copy of sensitive messages.

Safe audit fields may include:

- notification ID
- incident ID
- policy ID and policy version
- actor account ID, if an authenticated actor initiated the event
- recipient account ID or trusted-contact relationship ID
- channel class, such as account inbox, email, SMS, push, or chat
- template ID or version
- controlled event type
- controlled reason category
- state transition, such as queued, sent, suppressed, failed, cancelled, or
  expired
- timestamp
- attempt count
- safe provider category, not raw provider payload

Audit records must not include:

- raw viewer tokens or incident tokens
- raw session tokens or Authorization headers
- request bodies
- message bodies that include sensitive context
- uploaded bytes, plaintext, transcripts, or media descriptions
- raw keys, wrapped-key ciphertext, key shares, or private keys
- full GPS coordinates, speed, heading, route history, or user safety narrative
- stored paths, object keys, provider secrets, private deployment details, or
  raw provider webhook payloads

Free-form operator notes should be avoided. If a future implementation needs
notes, they require separate privacy, redaction, retention, and access-control
review.

## Deployment Requirements

Future notification delivery changes deployment risk. Before enabling it, a
deployment must review:

- exact provider and region
- provider data retention and logging
- sender identity and abuse reporting behavior
- secret storage and rotation
- webhook or callback validation
- TLS and public callback routing, if callbacks exist
- provider retry and duplicate behavior
- edge and app rate limits
- message-template review
- viewer-token expiry and revocation policy
- operational support process for failed or delayed notifications
- backup/restore behavior for queued and already-sent notifications
- monitoring that avoids token, message-body, location, or provider-secret
  leakage

Notification delivery must not be described as production-ready emergency
infrastructure. Users and trusted contacts remain responsible for contacting
emergency services.

## Future Implementation Prerequisites

Before implementation, accept designs for:

- account-based trusted-contact incident review
- no-account viewer-link delivery and token expiry defaults
- safety-check state, grace periods, cancellation, and false-positive handling
- notification preferences and opt-out policy
- retry, suppression, and rate-limit behavior
- provider configuration and secret handling
- safe audit records and retention
- deployment warnings and runbooks
- tests for redaction, retries, suppression, opt-out, rate limits, and provider
  failure handling

Implementation must update [API](api.md), [security model](security-model.md),
[threat model](threat-model.md), [deployment](deployment.md), and
[logging requirements](logging-requirements.md) before or alongside runtime
changes.
