# Break-Glass And Dead-Man-Switch Key Access

This document designs possible future server-assisted key access for Proofline.
It is a design document only. It does not implement server-side decryption, key
escrow, API routes, database schema changes, background jobs, notifications,
or dead-man-switch logic.

## Summary

Break-glass key access is an explicit mode where a server, operator,
deployment secret store, or policy-controlled process can help recover or use
media keys when normal client-side or trusted-contact access is unavailable. A
dead-man switch is a related policy that grants or escalates access after
configured conditions are met, such as missed checkins or a device being
offline too long.

This may be needed because Proofline is meant to preserve incident evidence when a phone is lost, damaged, powered off, taken, destroyed, or otherwise unavailable. Contact-wrapped keys and browser/client-side decryption should be the default future path, but some deployments may want an additional recovery path for cases where trusted contacts cannot decrypt quickly enough.

This is not implemented today. The current backend remains ciphertext-only: it stores encrypted chunk bytes, validates hashes over ciphertext, and produces encrypted ZIP bundles. It does not store raw media keys or decrypt media.

Any future break-glass mode would increase backend, operator, and deployment trust requirements. It must be explicit, separately configured, auditable, tested, threat-modeled, and documented with clear deployment warnings. It must never appear as an incidental side effect of unrelated key custody, viewer, simulator, or incident-mode work.

The first implementation should be wrapped-key release only: it may authorize
delivery of already stored contact-wrapped, device-wrapped, or recovery-wrapped
CEKs to an authorized reviewer under an accepted policy. It must not unwrap
keys on the server, store raw server-held media keys, decrypt evidence, create
plaintext exports, or contact emergency services.

## Incident Mode Boundary

Break-glass and dead-man-switch behavior should be policy attached to an incident or account, not an automatic property of recording.

| Incident mode | Break-glass implication |
|---|---|
| Emergency incident | May justify urgent trusted-contact access or explicit break-glass policy if configured. |
| Interaction record | Should not trigger emergency escalation by default. Sharing, export, or decryption should remain deliberate. |
| Safety check | May trigger trusted-contact access after a missed check-in, but requires careful grace periods, cancellation, and false-alarm handling. |
| Evidence note | Usually private by default. Break-glass access is unlikely unless the user explicitly configured it. |

Do not use labels such as `police mode` as access-control policy. Future clients may allow user-selected tags, but tags must not silently change key custody or escalation behavior.

## Access Modes To Keep Separate

Future design and implementation must keep these modes separate:

| Mode | What it means | What it must not imply |
|---|---|---|
| Ordinary sharing | The account owner deliberately grants access to an incident, stream, bundle, or wrapped-key record. | No missed-check-in trigger, dead-man-switch timer, server unwrap, notification, or emergency response. |
| Missed safety-check escalation | A user-configured timer or check-in policy changes state after the user misses a check-in or a device remains offline past a grace period. | No automatic raw-key release, plaintext export, or emergency-services contact. |
| Trusted-contact review | A trusted contact can review authorized metadata, encrypted evidence, and eligible wrapped-key material according to grants and policy state. | No admin/operator authority, no public admin dashboard, and no access to incidents or keys outside the grant. |
| Optional break-glass key access | A higher-trust emergency mode that may allow server-assisted key access only after separate policy, audit, deployment, and threat-model review. | No default behavior, no incidental server escrow, no broad public `/v1` gateway, and no unreviewed raw-key or plaintext path. |

The current backend implements none of these future escalation behaviors. The
existing optional `incident_mode`, `capture_profile`, `escalation_policy`, and
`sharing_state` fields remain labels unless a later issue explicitly adds
reviewed behavior.

## Availability Requirement

Production key custody must assume the user's phone may be unavailable during the moment when evidence is needed most. The device may be:

- lost
- damaged
- powered off
- taken
- destroyed
- disconnected from the network
- unable to complete a final key-share upload

Phone-only keys are therefore not sufficient for the full Proofline product. They protect confidentiality well when the backend or blob storage is compromised, but they can turn preserved evidence into unusable ciphertext if the phone is gone.

The preferred availability baseline is contact-wrapped key material: the client encrypts media, uploads ciphertext, and uploads wrapped media keys for trusted contacts. Break-glass access is a stronger availability option that should be treated as optional and higher risk, not as the default path.

## Candidate Access Models

### 1. Server Stores Wrapped Key Material Only

The backend stores encrypted or wrapped copies of incident or stream media keys. Those keys are wrapped for trusted contacts, future devices, recovery keys, or other explicitly designed recipients. The server can deliver wrapped material but cannot unwrap it by itself.

What the server can access:

- ciphertext chunks
- metadata
- wrapped keys
- token-gated incident summaries
- no raw media keys in normal operation

This is the strongest default fit. It preserves the current ciphertext-only backend posture while improving availability when trusted contacts are enrolled and have a working decrypt path.

Failure modes include missing trusted-contact setup, failed wrapped-key upload, contact private-key loss, server omission/corruption of wrapped keys, and misunderstood revocation semantics.

### 2. Server Can Unwrap Keys Under Break-Glass Policy

The backend or deployment environment stores media keys wrapped to a server escrow key. Under an explicit break-glass policy, the server can unwrap the media key and either return it to an authorized decrypting client or use it in a closely controlled operation.

What the server can access:

- wrapped keys at rest
- raw media keys during authorized break-glass operations
- potentially plaintext if it uses those keys to decrypt

Operator trust requirements are high. Every unwrap attempt must be audited with incident ID, timestamp, triggering policy, caller or actor, decision, and outcome. Logs must never include raw keys, plaintext, raw viewer tokens, uploaded bytes, or sensitive safety data beyond minimal audit metadata.

This may be useful for self-hosted deployments where the user explicitly trusts the operator or is the operator. It should be disabled by default and clearly marked as a higher-trust mode.

### 3. Server Decrypts Or Transcodes Media Under Break-Glass Policy

After a break-glass trigger, the server unwraps media keys and decrypts chunks server-side. It may also merge, transcode, or produce a playable media export for authorised contacts.

This is the highest-risk model because the backend becomes part of the plaintext handling path. It can be useful for non-technical contacts, but it creates major logging, caching, retention, backup, and operator-misuse risks.

This is not recommended as an early production mode. It may be acceptable later only as a deliberate, high-warning feature with retention, deletion, logging, and audit design completed first.

### 4. n-of-m Trusted Contacts

Media recovery requires a threshold of trusted contacts or shares. For example, two of three contacts may need to approve or contribute key material before a media key can be recovered.

This can reduce unilateral misuse but may slow urgent access if enough people are unavailable. It may be useful later for high-risk users, but it is too complex for the first production key custody milestone.

### 5. Maintainer Or Operator Assisted Recovery

A trusted maintainer or operator manually performs recovery steps, such as validating a request, unlocking a local secret, approving break-glass access, or helping a contact retrieve wrapped key material.

This is high-trust and deployment-specific. Manual actions require clear audit trails: actor identity, request source, incident ID, decision, timestamp, reason, and post-incident review status. Public issue trackers, support logs, and chat transcripts must not receive sensitive incident data, raw tokens, raw keys, plaintext, or private deployment details.

### 6. External KMS, HSM, Or Secret Store

A deployment may put escrow key material behind an external KMS, HSM, hardware
token, or local locked secret store. The server would request use of the
escrow key only after policy allows it.

This can improve separation between the application database and escrow secret,
but it does not remove operator trust. The operator, KMS policy, access logs,
backup process, and emergency runbook become part of the security boundary.
For self-hosted deployments, this may be too complex for early versions. For a
shared hosted deployment, it would need separate architecture, key rotation,
access approval, monitoring, incident response, and restore testing.

### 7. No Server Break-Glass Support

A deployment may decide not to support server-assisted key access at all. In
that model, Proofline stays with contact-wrapped keys, device-wrapped keys,
recovery-wrapped keys, and client-side or contact-side decryption.

This has the best ordinary confidentiality story because the backend cannot
unwrap CEKs. It has weaker availability if the owner device and all configured
contacts or recovery holders are unavailable. It remains a valid policy choice
for deployments that prefer fail-closed confidentiality over emergency
operator recovery.

## Dead-Man Switch Policy Design

Dead-man switch logic should be explicit and conservative.

Possible trigger inputs:

- missed user check-in
- device offline beyond a configured grace period
- active incident started with urgent escalation enabled
- account-owner preconfigured timer
- trusted-contact review request after an access grant
- optional future operator-approved break-glass request

Required design decisions before implementation:

- check-in interval and grace period
- cancellation behavior and cancellation deadline
- whether network loss pauses, extends, or triggers escalation
- whether the user can configure different policies for emergency incidents, interaction records, safety checks, and evidence notes
- which trusted contacts receive alerts
- what data contacts see before decryption
- whether contacts can request more access
- how false positives and false negatives are audited
- whether escalation unlocks only metadata, encrypted bundles, wrapped keys, raw keys, or plaintext exports

A missed check-in should not automatically mean emergency services were
contacted. Proofline does not currently contact emergency services. Trusted
contacts should review the context and decide whether to call emergency
services unless a future jurisdiction-specific emergency-services integration
is explicitly designed, implemented, and documented.

Suggested trusted-contact wording for a missed check-in:

```text
A Proofline safety check was missed.
Review the incident, try to contact the user, and call emergency services if you believe there is immediate danger.
```

## Policy State Model

A future dead-man-switch or break-glass policy should have an explicit state
machine. Do not infer state from incident labels, timestamps alone, viewer-token
existence, or contact relationships.

Suggested future states:

| State | Meaning | Allowed next states |
|---|---|---|
| `disabled` | No dead-man-switch or break-glass policy is armed for the account, contact, incident, or stream. | `armed` |
| `armed` | The user or policy owner configured escalation, but no trigger has fired. | `pending_grace`, `cancelled`, `expired`, `disabled` |
| `pending_grace` | A trigger candidate fired and the user is inside the cancellation or grace window. | `cancelled`, `contact_review`, `expired`, `failed_closed` |
| `contact_review` | Eligible trusted contacts may review safe metadata and attempt to reach the user. | `wrapped_key_release_authorized`, `cancelled`, `expired`, `failed_closed` |
| `wrapped_key_release_authorized` | Policy allows delivery of eligible wrapped-key material to authorized contacts or clients. | `completed`, `revoked`, `failed_closed` |
| `cancelled` | The user or another authorized actor cancelled before key release or escalation. | terminal for that trigger occurrence |
| `revoked` | A previously authorized release is withdrawn for future access. | terminal for that release occurrence |
| `expired` | The policy or trigger window ended without release. | terminal for that occurrence |
| `failed_closed` | The system could not prove authorization, policy state, contact eligibility, or safe delivery. | terminal until a new trigger occurrence |
| `completed` | The release or review flow finished according to policy. | terminal for that occurrence |

State transitions should be monotonic for one trigger occurrence. Retrying a
failed notification or key-delivery attempt should not rewind policy state or
create a broader grant than the user configured.

## Trigger Sources, Grace Periods, And Cancellation

Trigger sources should be narrow, explicit, and auditable. Plausible future
sources include:

- a missed user check-in after the configured interval
- device offline status after a configured threshold
- an urgent incident start with a user-selected escalation delay
- an account-owner timer configured before the incident
- a trusted-contact review request that still requires policy approval
- an operator-assisted request in a deployment where that mode is explicitly enabled

Each trigger must define:

- the actor or system component allowed to create it
- the policy version used for the decision
- the grace period and exact cancellation deadline
- who can cancel during the grace period
- whether cancellation requires the original device, any trusted device, an
  account session, a second factor, or another recovery channel
- whether repeated missed check-ins create one escalation occurrence or many
- whether network loss extends the grace period, starts a longer offline
  timer, or fails closed
- what contacts can see before wrapped-key release
- how long released access remains valid

Cancellation must be designed for hostile and unreliable conditions. If a user
is coerced, a too-easy cancellation path may suppress legitimate escalation. If
connectivity is poor, a too-strict cancellation path may create false
positives. A future implementation should let users configure conservative
grace windows and should record only safe cancellation metadata.

## Contact Review Boundary

Trusted-contact review is not the same as key release. Before release,
contacts should see only the minimum information needed to interpret the
escalation and attempt to reach the user. Future review screens should avoid
showing full GPS, narrative safety details, raw tokens, stored paths, object
keys, uploaded bytes, plaintext, or wrapped-key ciphertext unless a separate
authorized evidence-view path allows it.

Contact review should answer:

- which trusted contacts are eligible for this policy
- whether contacts must be accepted, active, and bound to active recipient keys
- whether one contact can review alone or multiple contacts must agree
- whether contacts can request escalation without receiving key material
- whether contact review can be cancelled by the user during the grace window
- whether review creates a durable audit event

Trusted contacts remain responsible for deciding whether to contact emergency
services. Proofline should not say help is on the way or that authorities were
notified unless a future emergency-services integration is separately designed
and actually implemented.

## False Positives, False Negatives, And Offline Devices

Dead-man-switch policy is safety-sensitive because both early and late
escalation can cause harm.

False-positive cases include:

- the user forgets to check in
- the phone battery dies
- the phone loses data service
- the app is killed by the OS
- a clock or timezone changes unexpectedly
- a server, relay, or notification path is delayed
- a trusted contact interprets limited metadata incorrectly

False-negative cases include:

- a coercer cancels the policy
- the phone is destroyed before an incident or wrapped-key upload completes
- the trigger source never reaches the server
- contacts are unreachable or have lost private keys
- the policy expires too soon
- the deployment is down or restored from stale backup

Offline-device behavior should be explicit. A device that is merely offline
should not by itself prove danger. A device that is offline past a user-chosen
threshold may move the policy to `pending_grace` or `contact_review`, but any
wrapped-key release should require the policy to say that offline status is a
valid release condition. If the server cannot tell the difference between
offline, app failure, intentional cancellation, or network partition, the
default should be conservative and auditable rather than pretending certainty.

## Access Policy Requirements

Before implementing break-glass or dead-man-switch key access, define:

- who can configure the policy
- who can trigger the policy
- who can cancel the policy
- who can review an escalation
- which second-factor or recovery checks are required for policy changes
- what contacts or operators can see before decryption
- what evidence can be decrypted
- whether raw keys are ever exposed
- whether plaintext exports are created
- how long access lasts
- how access can be revoked
- how audit records are retained
- how policy versions are migrated when product behavior changes
- how backup restore handles armed, pending, cancelled, and completed states

The design must distinguish account-owner access, trusted-contact access, bearer-link access, admin/operator access, and optional server escrow access.

For the first implementation, the policy should authorize only wrapped-key
release. A wrapped-key release flow can deliver encrypted CEK metadata already
stored for an eligible recipient. It must not require the server to unwrap,
decrypt, transcode, or export plaintext.

## Audit And Logging Requirements

Audit logs should be useful for review without becoming a second copy of sensitive evidence.

Safe audit fields may include:

- incident ID
- stream ID, if a policy is stream-scoped
- policy ID and policy version
- trigger ID
- actor or trusted-contact ID
- actor type, such as account owner, trusted contact, operator, or system
- action type
- timestamp
- grace-period deadline
- state transition
- decision or outcome
- non-sensitive reason category
- safe error category
- release scope, such as metadata-only, bundle access, or wrapped-key release

Audit logs must not include:

- raw viewer tokens or incident tokens
- raw keys or key shares
- raw server escrow material
- wrapped-key ciphertext
- plaintext media or transcripts
- uploaded bytes
- request bodies
- Authorization headers
- private deployment details
- stored paths or object keys
- unnecessary user safety data

Audit fields should use controlled reason codes rather than free-form notes
where practical. Free-form operator notes are risky because they tend to collect
private deployment details, safety narratives, token fragments, or support
transcripts. If free-form notes are ever allowed, they need separate privacy,
retention, access-control, and redaction review.

## Deployment Requirements

Break-glass and server escrow modes require stronger deployment controls than the current ciphertext-only backend:

- TLS at the edge
- reviewed main `/v1` access boundaries
- app-level authorization before public control-plane exposure
- rate limiting and abuse controls
- restricted operator access
- secret storage with backup and restore procedures
- key rotation and revocation policy
- deployment-specific retention/deletion policy for any plaintext outputs
- tested restore and emergency procedures
- warning text during policy enablement
- post-event review and revocation procedures

Self-hosted deployments may intentionally accept stronger operator trust. Public or shared deployments need stricter separation, policy, audit, and warning text.

Deployment documentation must say plainly that enabling server escrow or
server-side decryption changes the trust model. It makes the deployment
operator, secret store, backups, monitoring pipeline, and incident-response
process part of the evidence-confidentiality boundary. A deployment that
cannot protect those systems should not enable server-assisted key access.

## First Implementation Direction

The first implementation should be wrapped-key release only.

Allowed first implementation shape:

- policy state records for account, incident, stream, or grant scope
- trigger and cancellation records with controlled reason codes
- trusted-contact review state
- authorization to deliver already stored wrapped-key records to eligible
  active recipients
- audit records using only safe fields
- user and operator warnings that the feature is not emergency dispatch

Disallowed in the first implementation:

- server unwrapping of CEKs or media keys
- server-held raw media keys
- server escrow private-key storage
- backend decryption
- plaintext media export or transcoding
- emergency-services integration
- public admin dashboards or public operator routes
- provider-specific SMS, push, Messenger, or email notification delivery
  unless separately scoped by a notification-boundary issue

Server escrow, raw server-side key access, or server-side decryption would
require a separate security-sensitive design and review before implementation.
That review must update the security model, threat model, key-custody docs,
encryption docs, deployment guidance, API docs, tests, runbooks, and release
warnings. It must also define the escrow secret store, operator authorization,
rate limits, audit review, backup/restore impact, plaintext handling,
revocation, incident response, and user-facing consent model.

## Future Work

Likely implementation phases:

1. Keep the current backend ciphertext-only.
2. Build mode-driven policy on top of the optional incident-mode, capture-profile, escalation-policy, and sharing-state metadata fields.
3. Prototype contact-wrapped keys without server decryption.
4. Prototype trusted-contact client-side decryption.
5. Design dead-man switch triggers, cancellation, notification, and audit policy.
6. Implement wrapped-key release under an accepted policy, if the maintainer
   accepts that scope.
7. Only then consider optional server escrow or server-side decryption.

Each phase must update [security-model.md](security-model.md), [threat-model.md](threat-model.md), [key-custody.md](key-custody.md), [encryption.md](encryption.md), and operational guidance where relevant.

## Open Questions

- Should break-glass be available for interaction records, or only emergency incidents and safety checks?
- How should false missed-check-in alerts be cancelled and audited?
- Should contacts receive metadata before key access is granted?
- Can trusted contacts request escalation, or only receive it?
- Should server escrow exist at all in a first production release?
- What deployment secret store is acceptable for self-hosted versus shared deployments?
- What plaintext export formats, if any, are acceptable later?
- How should retention and deletion apply to decrypted outputs?
- What exact warning text should users see when enabling higher-trust server access?
