# Break-Glass And Dead-Man-Switch Key Access

This document designs possible future server-assisted key access for Proofline. It is a design document only. It does not implement server-side decryption, key escrow, API routes, database schema changes, background jobs, notifications, or dead-man-switch logic.

## Summary

Break-glass key access is an explicit mode where a server, operator, deployment secret store, or policy-controlled process can help recover or use media keys when normal client-side or trusted-contact access is unavailable. A dead-man switch is a related policy that grants or escalates access after configured conditions are met, such as missed checkins or a device being offline too long.

This may be needed because Proofline is meant to preserve incident evidence when a phone is lost, damaged, powered off, taken, destroyed, or otherwise unavailable. Contact-wrapped keys and browser/client-side decryption should be the default future path, but some deployments may want an additional recovery path for cases where trusted contacts cannot decrypt quickly enough.

This is not implemented today. The current backend remains ciphertext-only: it stores encrypted chunk bytes, validates hashes over ciphertext, and produces encrypted ZIP bundles. It does not store raw media keys or decrypt media.

Any future break-glass mode would increase backend, operator, and deployment trust requirements. It must be explicit, separately configured, auditable, tested, threat-modeled, and documented with clear deployment warnings. It must never appear as an incidental side effect of unrelated key custody, viewer, simulator, or incident-mode work.

## Incident Mode Boundary

Break-glass and dead-man-switch behavior should be policy attached to an incident or account, not an automatic property of recording.

| Incident mode | Break-glass implication |
|---|---|
| Emergency incident | May justify urgent trusted-contact access or explicit break-glass policy if configured. |
| Interaction record | Should not trigger emergency escalation by default. Sharing, export, or decryption should remain deliberate. |
| Safety check | May trigger trusted-contact access after a missed check-in, but requires careful grace periods, cancellation, and false-alarm handling. |
| Evidence note | Usually private by default. Break-glass access is unlikely unless the user explicitly configured it. |

Do not use labels such as `police mode` as access-control policy. Future clients may allow user-selected tags, but tags must not silently change key custody or escalation behavior.

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

A missed check-in should not automatically mean emergency services were contacted. Proofline does not currently contact emergency services. Trusted contacts should review the context and decide whether to call emergency services unless a future jurisdiction-specific emergency-services integration is explicitly designed, implemented, and documented.

Suggested trusted-contact wording for a missed check-in:

```text
A Proofline safety check was missed.
Review the incident, try to contact the user, and call emergency services if you believe there is immediate danger.
```

## Access Policy Requirements

Before implementing break-glass or dead-man-switch key access, define:

- who can configure the policy
- who can trigger the policy
- who can cancel the policy
- who can review an escalation
- what contacts or operators can see before decryption
- what evidence can be decrypted
- whether raw keys are ever exposed
- whether plaintext exports are created
- how long access lasts
- how access can be revoked
- how audit records are retained

The design must distinguish account-owner access, trusted-contact access, bearer-link access, admin/operator access, and optional server escrow access.

## Audit And Logging Requirements

Audit logs should be useful for review without becoming a second copy of sensitive evidence.

Safe audit fields may include:

- incident ID
- actor or trusted-contact ID
- action type
- policy name or version
- timestamp
- decision or outcome
- non-sensitive reason category

Audit logs must not include:

- raw viewer tokens or incident tokens
- raw keys or key shares
- plaintext media or transcripts
- uploaded bytes
- request bodies
- Authorization headers
- private deployment details
- unnecessary user safety data

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

Self-hosted deployments may intentionally accept stronger operator trust. Public or shared deployments need stricter separation, policy, audit, and warning text.

## Future Work

Likely implementation phases:

1. Keep the current backend ciphertext-only.
2. Build mode-driven policy on top of the optional incident-mode, capture-profile, escalation-policy, and sharing-state metadata fields.
3. Prototype contact-wrapped keys without server decryption.
4. Prototype trusted-contact client-side decryption.
5. Design dead-man switch triggers, cancellation, notification, and audit policy.
6. Only then consider optional server escrow or server-side decryption.

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
