# /v1 Access Control

This document defines the current local access-control boundary for Proofline's
authenticated main `/v1` control plane and the future direction for broader
product access.
Local username/password accounts and opaque server-side sessions are implemented
for the main `/v1` API. Account-owner contact public-key registration,
sharing-grant metadata routes, and grant-bound wrapped-key record routes are
implemented behind that same reviewed boundary. OAuth, JWT, public account
portals, trusted-contact accounts, notifications, browser decryption, key
escrow, and server-side decryption are not implemented.

## Summary

The current main `/v1` API requires a local account session for product routes
other than login. Existing `/v1/admin/...` JSON routes are mounted on the main
handler and require an admin account; they are not public-ready routes and must
be blocked from public reverse-proxy routes. First-admin bootstrap is handled by
the private `/admin` dashboard flow, and the private-admin listener serves only
the `/admin` route tree. Local sessions reduce accidental unauthenticated
access; they do not make `/v1` a public product API.

Future trusted-contact access, incident modes, notifications, production key
custody, browser/client-side decryption, and optional break-glass access all need
explicit role and grant boundaries before any broad public API exposure. This
document keeps those boundaries visible without choosing an external identity
provider.

Related source-of-truth docs:

- [Security model](security-model.md)
- [Threat model](threat-model.md)
- [Deployment](deployment.md)
- [Main API public exposure listener split](public-api-listener-split.md)
- [Incident capture modes](incident-modes.md)
- [Key custody and emergency access](key-custody.md)
- [Contact key sharing, grants, and wrapped-key metadata](contact-key-sharing-grants.md)
- [Browser-side decryption](browser-decryption.md)
- [Break-glass key access](break-glass-key-access.md)

## Goals

- Preserve the current reviewed-boundary `/v1` posture even with local account
  sessions.
- Separate account-owner, trusted-contact, public-link, admin/operator, and
  optional escrow access.
- Split future non-admin product API routes from a separately bound private
  admin API listener.
- Keep the public incident viewer read-only and separate from public product API
  routes and private admin routes.
- Distinguish access to metadata, ciphertext, wrapped keys, raw keys, and
  plaintext.
- Define token, session, grant, revocation, and audit expectations for current
  local auth and future implementation work.
- Keep incident-mode, key-custody, deployment, security-model, and threat-model
  docs aligned around one future access-control direction.

## Non-Goals

- No broad public exposure of the current main `/v1` API.
- No OAuth, JWT, public account portal, broad CSRF framework, or
  identity-provider implementation.
- No web-client, iOS-client, Android-client, or protocol implementation.
- No push notification, SMS, Messenger, email, or emergency-services
  integration.
- No backend decryption, browser decryption, key escrow, trusted-contact
  accounts, or break-glass implementation.
- No public admin dashboard.
- No claim that Proofline is production-ready public infrastructure.

## Current Boundary To Preserve

Today the backend has two listener groups:

| Listener group | Current routes | Exposure |
|---|---|---|
| Main API and viewer | `/v1/...` with local account/session auth except login, including existing admin-only JSON APIs; `/i/{token}` plus legacy `/e/{token}` aliases and `/static/...` | Reviewed main API deployment boundary; viewer paths may be routed publicly when only viewer paths are forwarded. Public edges must not route `/v1/admin/...`. |
| Private admin dashboard | `/admin`, `/admin/...`, `/admin/static/...` | Localhost, LAN, WireGuard, firewall, or strict private reverse proxy only. |

Current `/v1` routes are on the main handler. The implemented local auth model has admin and user roles, incident ownership, hashed password storage,
hashed session-token storage, session expiry, logout, account password change,
admin account creation, admin session revocation, owner-scoped contact
public-key metadata, owner-managed sharing grants, and owner-managed
wrapped-key records. Sharing-grant and wrapped-key management are deliberately
stricter than ordinary incident reads: they require the authenticated account
to own the incident, and an admin account cannot manage another account's
grants or wrapped-key records through the product route set unless it is also
the incident owner. Reverse-proxy rate
limiting, separate bind addresses, and private network placement are useful
boundaries, but they are not a public authorization model.

The private `/admin` surface is outside the `/v1` API namespace and remains on
the private-admin listener. Its login and bootstrap forms reuse the same local account
and server-side session store, with the raw session token held in an HttpOnly
SameSite cookie scoped to `/admin`. The authenticated dashboard lists local
accounts and supports current-admin password changes plus admin password resets
for other local accounts. Authenticated state-changing forms use a
session-bound CSRF token. The token-neutral CSS under `/admin/static/...` is
unauthenticated because it contains no incident data, tokens, keys, or
deployment details.

## Listener Topology

Implementation should avoid treating `/v1` as one public control plane. The
current topology is separate listener groups with separate route trees. The
`8080` main API/viewer and `8081` private admin-dashboard split is documented in
[main API public exposure listener split](public-api-listener-split.md):

| Listener group | Future route scope | Exposure |
|---|---|---|
| Main product API | Current account-owner incident, upload, sharing, and account self-service routes, plus future capture-device, trusted-contact, public-link grant, and key-wrapping delivery routes that are safe for public authenticated access after implementation. | Public HTTPS only after authentication, authorization, abuse controls, and audit behavior exist. |
| Private admin API | Operator/admin health, migration, support, abuse response, operational review, and optional deployment management routes. | Own listener and route tree, configurable for loopback, LAN, WireGuard, VPN, firewall, or private reverse proxy access. Still authenticated and authorized. |
| Incident viewer | Read-only incident viewer routes and token-neutral static assets mounted on the main listener. | Public HTTPS/reverse proxy when only viewer paths are exposed. |
| Optional escrow or break-glass API | Higher-trust emergency-access or server-assisted key access routes, if ever implemented. | Disabled by default; separate explicit configuration, strong authentication, audit, rate limiting, and deployment warnings. |

Private network placement is not an authentication substitute. Even when the
private admin API is reachable only over a VPN or WireGuard interface, every
admin/operator route should still authenticate the operator and authorize the
requested action. Public product API routes should not expose admin/operator
actions.

## Access Roles

Future implementation should use explicit roles or equivalent grant types. A
single actor may hold more than one role, but authorization checks should reason
about the role used for the current action.

| Role | Purpose | Default access direction |
|---|---|---|
| Account owner | The person who owns the incident record, devices, contacts, and sharing policy. | May create and manage their own incidents, contacts, grants, retention choices, and exports after authentication. |
| Capture device | A device or client acting for an account owner during recording or upload. | May create incidents, streams, checkins, and encrypted chunk uploads only for the owning account and authorized incident. |
| Trusted contact | A person pre-authorized by the account owner or an explicit escalation policy. | May access selected incident metadata, encrypted bundles, and wrapped key material only according to a grant or escalation policy. |
| Public link | A bearer-link viewer capability for one incident, similar to the current incident viewer token. | Read-only, incident-scoped, time-bound where practical, and not a general account or `/v1` credential. |
| Admin/operator | A deployment or service operator responsible for health, support, migration, and abuse response. | Should not casually access user safety data, raw tokens, raw keys, plaintext, or uploaded bytes. Administrative access must be least-privilege and audited. |
| Optional escrow actor | A separately configured break-glass or dead-man-switch actor or policy. | Disabled by default; may access wrapped keys, raw keys, or plaintext only if an explicit future escrow mode is implemented and audited. |

## Data Classes

Authorization must distinguish what kind of data is being accessed. A grant to
read incident metadata is not automatically a grant to decrypt media.

| Data class | Examples | Access expectation |
|---|---|---|
| Public viewer shell | Static assets and token-neutral viewer UI | Publicly reachable when served by the incident viewer. |
| Incident metadata | Incident status, stream state, timestamps, checkins, chunk counts, non-secret display metadata | Role-scoped; public-link access remains incident-scoped and read-only. |
| Ciphertext evidence | Encrypted chunks and encrypted ZIP bundles | Role-scoped; access to ciphertext alone does not imply decryption capability. |
| Wrapped key material | Media keys encrypted to contacts, devices, recovery methods, or escrow modes | Returned only to roles authorized for that wrapping target and incident. |
| Raw keys | Media keys, contact private keys, escrow keys, key shares | Never logged; never exposed in default mode; only possible in an explicit future break-glass mode. |
| Plaintext | Decrypted audio, video, notes, transcripts, or exports | Out of scope for the current backend; any future plaintext path requires separate design, audit, retention, and deployment warnings. |

## Route Exposure Direction

The future control plane should use explicit exposure classes. Route names below
describe policy shape; they are not implementation commitments.

| Route class | Future exposure | Notes |
|---|---|---|
| Current main `/v1` routes | Main listener with local account/session authentication. | Includes login/logout, account/password routes, incident creation, stream creation, chunk upload, checkins, close/fail/complete actions, incident-token creation/revocation, contact public-key registration, owner-scoped sharing-grant management, authenticated chunk reads, and existing admin-only JSON APIs. Public edges must not route `/v1/admin/...`. |
| Current private-admin dashboard routes | Private only with admin web session authentication or first-admin bootstrap secret. | Includes `/admin` bootstrap, login, logout, account-list, password-change, password-reset forms, and token-neutral `/admin/static/...` assets. |
| Public product API routes | Public-authenticated only after account/device/contact authz, upload abuse controls, request-size controls, and audit are implemented. | Should cover non-admin product flows: account-owner incidents, capture uploads, trusted-contact access, account-owner public-link grant issuance/revocation, sharing, and wrapped-key delivery. |
| Public-link viewer routes | Public read-only viewer routes can remain separate from the public product API. | Current `/i/{token}` and `/e/{token}` paths are bearer-token URLs and must not become write or admin routes. |
| Private admin API routes | Own private listener and route tree, authenticated and authorized even when bound only to VPN, WireGuard, LAN, loopback, firewall, or a private proxy. | Should be narrow, audited, and safe for support without exposing evidence contents, raw tokens, raw keys, or plaintext by default. |
| Escrow/break-glass routes | Not present by default. | Require explicit configuration, policy, audit, warnings, strong authz, and separate implementation. They must not be part of the normal public product API. |

Do not mount `/admin`, operator maintenance, escrow, or break-glass routes on
the main API/viewer listener or on any public viewer edge route. Do not route
`/v1/admin/...` from a public edge. Do not mount unauthenticated write,
account, contact, admin, or escrow routes on any listener.

## Authentication Expectations

The current main `/v1` API uses local username/password accounts, bcrypt
password hashing, and opaque bearer session tokens. Raw session tokens are
returned only to the client and stored only as hashes. Sessions expire and can
be revoked.

The first admin account is created through a one-time bootstrap flow:

- the server fails closed at startup when no admin account exists and
  `SAFE_AUTH_BOOTSTRAP_SECRET` is not configured
- `POST /admin/bootstrap` requires the bootstrap secret as a private admin web
  form field
- bootstrap is disabled after an admin account exists
- operators should remove the bootstrap secret after creating the first admin

A future public product API may choose cookie sessions, delegated identity,
device-bound credentials, or another reviewed mechanism. That choice must be
made in a separate task with tests and deployment guidance.

Authentication must provide:

- stable actor identity for current local accounts, and for future account
  owners, trusted contacts, capture devices, and operators
- credential expiry and rotation
- revocation for lost devices, removed contacts, leaked links, and operator
  access changes
- replay and token-theft risk analysis
- CSRF protection for browser-cookie flows that perform state-changing actions;
  the current private `/admin` authenticated state-changing forms use a
  session-bound token
- avoidance of raw credential logging, including Authorization headers and
  token-bearing URLs
- clear handling for offline or intermittent capture devices

The private admin API should use authentication and authorization that is at
least as strict as the public product API. A VPN, firewall, private bind
address, or reverse-proxy allowlist can reduce exposure, but it must not be the
only admin identity check.

Public-link bearer tokens are not a general authentication system. They should
remain scoped to a single incident, read-only, and revocable.

## Authorization Expectations

Authorization should be deny-by-default and checked close to the operation being
performed. Current implementation binds local account ID, role, and incident
owner. Current incident routes also pass route-level action and
data-class labels, but all current incident actions share the same
owner-or-admin policy. Future policy should also bind:

- actor or device identity
- account owner
- incident ID
- role or grant type
- requested action
- incident mode and escalation policy, when implemented
- grant expiry, revocation, and state
- key-access scope, if wrapped keys or escrow access are involved

Authorization decisions should not rely only on route prefix, listener address,
or client-provided account or incident IDs. Repository or service-layer code
that reads or mutates incident data should receive already-authorized scope, or
perform an equivalent check before returning data.

## Grant And Token Lifecycle

The current implementation separates durable account identity, public-link
viewer tokens, contact public-key metadata, owner-scoped sharing grants, and
grant-bound wrapped-key records. Grant records are authorization metadata: they
do not contain raw keys or plaintext and do not create trusted-contact
sessions. Wrapped-key records contain encrypted media-key material plus public
wrapping metadata and are delivered only while the bound grant and contact key
remain active.

Current and expected grant types:

- account-owner sessions or device credentials
- capture-device upload authorization
- trusted-contact access grants, currently owner-managed records only
- public-link viewer tokens
- optional operator support grants
- optional break-glass or escrow grants

Lifecycle requirements:

- grants should have explicit creation time, actor, scope, and purpose
- grants should have expiry where practical
- explicit no-expiry grants should be visible and reviewable
- revocation should take effect for future requests
- raw token values should be returned only at creation time when bearer tokens
  are used
- stored token material should be hashed or otherwise protected where practical
- grant state should distinguish active, expired, revoked, superseded, and
  consumed one-time grants where applicable
- deleting or removing a trusted contact should stop new access and new key
  wrapping, but older wrapped keys need separate revocation semantics

Viewer-token and session-token lifecycle rules from the current implementation
are a useful starting point: store only token hashes, return raw tokens only at
creation or login time, apply expiry, and make expired, revoked, and invalid
viewer tokens indistinguishable to the public viewer.

Current sharing grants can be scoped to an incident or one stream, can expire,
and can be revoked while retaining minimal audit metadata. Contact public keys
are versioned per contact; only active key versions can receive new grants.
Revoked contact keys cannot be reactivated.
Current wrapped-key delivery is owner-authenticated through private `/v1`
routes. Revoked or expired grants, inactive contact public keys, and revoked or
rotated wrapped-key records are filtered out of list and read responses. Public
viewer tokens do not receive wrapped keys.

## Incident Mode Policy

Incident type labels must not silently imply authorization. Future incident
modes should attach explicit escalation and sharing policies.

Expected defaults:

| Incident mode | Access-control default |
|---|---|
| Emergency incident | May grant urgent trusted-contact access only under an explicit policy. |
| Interaction record | Private by default; sharing and export should be deliberate account-owner actions. |
| Safety check | May grant trusted-contact access after a missed check-in only under an explicit missed-check-in policy with cancellation and grace-period rules. |
| Evidence note | Private by default unless explicitly shared or exported. |

Proofline should not claim that emergency services were contacted unless a
future jurisdiction-specific integration is explicitly implemented and
documented. Trusted contacts should receive enough context to decide what to do,
but the backend should not collapse capture, sharing, decryption, export, and
emergency response into one implicit action.

## Key Custody And Decryption Boundary

Access control and key custody are related but separate.

Default future direction:

- clients encrypt media before upload
- the backend stores ciphertext chunks
- the backend may store wrapped or encrypted copies of media keys
- trusted contacts receive wrapped key material only when authorized
- decryption happens client-side or contact-side where practical
- the server does not store raw media keys in default mode

Optional server escrow or server-side decryption is a separate high-trust mode.
It must be disabled by default or separately configured, audited, rate-limited,
documented with deployment warnings, and implemented only after an explicit
security-sensitive task.

An actor allowed to download encrypted bundles is not automatically allowed to
obtain wrapped keys. An actor allowed to obtain wrapped keys is not necessarily
allowed to obtain raw keys or plaintext.

Trusted-contact sharing follows
[contact-key-sharing-grants.md](contact-key-sharing-grants.md). Current grants
authorize metadata and ciphertext delivery by account owner, incident, stream,
recipient, data class, expiry, and state. Current wrapped-key records are
separate access-enabling metadata and are delivered only under an active
authorized grant. Public viewer tokens do not receive wrapped keys by default.

## Audit And Logging Expectations

Future access-control implementation should add auditability without turning
logs into another copy of sensitive evidence.

Useful audit fields may include:

- timestamp
- action type
- actor ID, contact ID, device ID, or operator ID
- role or grant type
- incident ID
- grant ID or policy version
- decision or outcome
- non-sensitive reason category

Audit logs must not include:

- raw viewer tokens or incident tokens
- raw access tokens, session tokens, refresh tokens, or Authorization headers
- request bodies
- uploaded bytes
- plaintext
- raw keys, key shares, or recovery phrases
- private deployment details
- object-storage credentials, bucket names, private endpoints, or object keys
- unnecessary user safety data

Public issue drafts, support tickets, chat transcripts, screenshots, and
operator dashboards must also avoid raw tokens, keys, plaintext, request bodies,
uploaded bytes, and private deployment details.

## Migration Path

The migration from the current reviewed-boundary deployment model should be
incremental:

1. Keep current main `/v1` product routes behind the deployment's reviewed
   boundary and authenticated with local sessions unless the route is explicitly
   login. Keep `/admin` dashboard routes on the private-admin listener and
   block `/v1/admin/...` from public reverse-proxy routes until a future
   private admin API route group is explicitly designed.
2. Keep contact public-key registration, sharing-grant management, and
   wrapped-key record delivery owner scoped while defining device,
   trusted-contact, public-link, operator, and optional escrow data model
   requirements in protocol/client design tasks.
3. Introduce separate future route groups for public product API, private admin
   API, and public incident viewer behavior in design and tests before changing
   exposure.
4. Extend authentication and authorization behind private deployments first,
   without changing public exposure.
5. Add audited grant lifecycle behavior for incident-scoped access.
6. Preserve the separately bound private-admin listener before adding more
   operator or admin routes, and require admin authentication even for VPN-only
   deployments.
7. Update security, threat, API, deployment, and operational docs before any
   public-authenticated product API route is exposed.
8. Expose only the smallest public-authenticated product route set needed for a
   concrete client milestone.
9. Keep public incident viewer routes read-only and separate from public product
   API and private admin API routes unless a later design deliberately replaces
   them.

Any step that changes key custody, wrapped-key delivery, browser decryption,
server escrow, or server-side decryption must also update the key-custody,
browser-decryption, break-glass, security-model, threat-model, encryption, and
deployment docs before or alongside implementation.

## Implementation Prerequisites

Before any public product API exposure or expanded private admin API
implementation, a future implementation task must define and test:

- concrete authentication mechanism for the new exposure class
- authorization policy and role/grant model beyond the current local user/admin
  roles
- contact, device, and incident ownership data model beyond current incident
  owner account IDs
- session, token, or grant storage and revocation behavior beyond current local
  sessions and viewer tokens
- upload abuse controls and rate-limiting expectations
- CSRF and browser credential rules, if browser sessions are used
- audit log schema and redaction rules
- migration behavior for existing incidents and viewer tokens
- deployment guidance for reverse proxies, TLS, logs, VPN/private admin
  binding, public product API exposure, and listener separation
- tests proving public product API, private admin API, and public incident
  viewer route separation
- tests proving denied cross-account, cross-incident, and non-admin-to-admin
  access

## Open Questions

- Should the first public-authenticated upload path use account-owner sessions,
  device credentials, or short-lived incident upload grants?
- Should current incident viewer tokens remain long-term, or become a legacy
  compatibility layer after account-based viewer access exists?
- How should contact grants interact with contact-wrapped media keys when a
  trusted contact is removed?
- Which metadata can trusted contacts see before decryption or escalation?
- Should operator support ever read incident metadata, or only health and
  storage integrity state?
- Should the private admin API live under a distinct route prefix, a distinct
  port only, or both?
- Should optional escrow access exist in the first production release, or wait
  until contact-wrapped keys are proven?
- How should authorization decisions be represented in future protocol or
  conformance tests?
