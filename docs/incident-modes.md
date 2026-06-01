# Incident Capture Modes

Proofline is intended to be broader than an emergency-only recorder. The long-term product direction is private, encrypted incident capture for moments where a user wants a durable record, with emergency escalation available only when the user chooses it or a configured safety check is missed.

This is a planning and schema-design document for mode-driven behavior. The
current backend implements optional nullable `incident_mode`, `capture_profile`,
`escalation_policy`, and `sharing_state` metadata fields on the existing private
incident create/read routes. Those fields do not add mobile clients, public
account workflows, public `/v1` exposure, push notifications, emergency-services
integration, key custody, browser decryption, trusted-contact access, retention
behavior, or new backend routes. Future account-owner, trusted-contact,
public-link, admin/operator, and optional escrow role boundaries are documented
in [v1-access-control.md](v1-access-control.md).

## Product Framing

Proofline should preserve encrypted evidence and context while keeping escalation separate from capture.

Core principles:

- capture can be emergency or non-emergency
- upload should preserve already-captured evidence if the device is lost, damaged, powered off, or taken
- emergency-services contact should remain a user or trusted-contact action, not an automatic backend action
- sharing, export, publication, and legal submission should be deliberate user-controlled steps
- recording and sharing laws vary by jurisdiction, so future clients should include clear user-facing guidance without giving legal advice

## Planned Incident Types

Future clients may expose incident modes such as:

| Incident mode | Purpose | Typical capture profile | Default escalation |
|---|---|---|---|
| Emergency incident | Active safety risk where the user wants recording, upload, and urgent trusted-contact access. | Audio/video/location where available. | Trusted contacts alerted immediately or after a short configured delay. |
| Interaction record | Non-emergency record of an important interaction, such as with police, security, landlords, employers, service providers, or other authorities. | Audio/video/location and notes where user-selected. | No automatic escalation by default. |
| Safety check | Timed check-in flow for walking home, meeting someone, travel, fieldwork, or other elevated-risk situations. | Location/check-in status, with optional media. | Trusted contacts alerted if the user misses the check-in. |
| Evidence note | Quick photo, audio, location, or note bundle for damage, harassment, threats, or disputes. | Note or attachment-oriented capture, with optional media. | No automatic escalation by default. |

Avoid product labels such as `police mode`. Use neutral language like `Interaction record` and optional user-selected tags.

## Design Vocabulary

Future implementation should keep these concepts separate:

- `incident_mode` is the user-visible reason for capture, such as emergency incident, interaction record, safety check, or evidence note.
- `capture_profile` describes what the client intends to capture, such as audio/video/location, audio/location, location check-ins, notes, attachments, or a custom combination.
- `escalation_policy` describes if and when trusted contacts or future emergency-access workflows are triggered.
- `sharing_state` describes what access has actually been granted or exported.
- User tags and notes are context metadata. They must not silently change access, key custody, notification, retention, or legal/export behavior.

The exact public protocol field names may differ, but the distinction between mode, capture, escalation, and sharing should remain.

## Interaction Records

Interaction records should help users preserve what happened during important encounters without treating every recording as an emergency.

Future client behavior should support:

- starting an interaction record quickly
- recording audio, video, location, notes, and important moments where platform permissions allow
- uploading encrypted chunks continuously when network access exists
- continuing local encrypted staging when upload is unavailable
- adding optional tags such as `police`, `security`, `workplace`, `landlord`, `medical`, or `other`
- adding post-incident notes, reference numbers, involved agency names, badge or vehicle numbers, and user-visible timeline markers
- keeping the incident private unless the user explicitly grants access or exports a bundle

The app should not imply that it reports alleged criminal activity, contacts law enforcement, guarantees admissibility, or provides legal advice.

## Future Data Model Direction

The current backend has generic incidents, streams, chunks, checkins, incident
tokens, and optional nullable incident-mode metadata fields. Missing mode fields
mean the incident remains a generic legacy incident. Stored mode fields are
server-visible metadata only; they do not grant access, create public links,
send notifications, change retention, change key custody, release wrapped keys,
expose plaintext, or change public viewer and bundle behavior.

Future protocol work may add a durable incident record shaped around these concepts:

```text
incident_mode:
  emergency
  interaction_record
  safety_check
  evidence_note

capture_profile:
  audio_video_location
  audio_location
  location_checkin
  note_or_attachment
  custom

escalation_policy:
  kind:
    none
    trusted_contacts_on_start
    trusted_contacts_on_missed_checkin
    urgent_trusted_contact_alert
  delay_seconds: optional non-negative integer
  contact_set_id: optional role-scoped identifier
  grant_scope:
    metadata_only
    ciphertext_bundle
    wrapped_keys
  cancel_until: optional timestamp for delayed or missed-check-in policies

sharing_state:
  private
  trusted_contact_access
  public_link_created
  legal_export_created
  revoked_or_expired
```

The schema should prefer explicit state over implied behavior. For example, `incident_mode: emergency` does not by itself notify anyone, return wrapped keys, expose plaintext, or create a public link. Those actions require an explicit escalation policy, sharing grant, and key-custody decision.

## Escalation Semantics

Emergency escalation should be a policy attached to an incident, not a property of all recording.

Expected policy behavior:

| Policy | Behavior | Default fit |
|---|---|---|
| `none` | Keep the incident private unless the account owner shares or exports it. | Interaction records and evidence notes. |
| `trusted_contacts_on_start` | Notify selected trusted contacts when the incident starts or after an accepted short delay. | Emergency incidents. |
| `trusted_contacts_on_missed_checkin` | Notify selected trusted contacts only if a configured check-in deadline plus grace period is missed. | Safety checks. |
| `urgent_trusted_contact_alert` | Notify selected trusted contacts urgently and provide emergency review guidance. | Explicit high-risk emergency mode. |

Safety-check escalation needs additional state before implementation, such as check-in due time, grace period, cancellation rules, missed-check-in timestamp, and whether network loss pauses, extends, or triggers escalation. False positives and false negatives are product and safety risks, not only timer bugs.

Dead-man switch handling should rely on trusted contacts to interpret the context and decide whether to contact emergency services. Proofline should not claim that help is on the way or that emergency services have been notified unless a future jurisdiction-specific integration explicitly implements and documents that behavior.

Suggested trusted-contact guidance:

```text
A Proofline safety check was missed.
Review the incident, try to contact the user, and call emergency services if you believe there is immediate danger.
```

## Server Schema Versus Client Metadata

Future server schema may need fields when the backend must enforce policy, return consistent summaries, or coordinate grants:

- incident mode and capture profile identifiers
- escalation policy kind, state, and relevant timestamps
- owner, device, trusted-contact, public-link, admin/operator, and optional escrow grant references from the access-control model
- non-secret sharing state derived from grants, exports, and revocations
- retention policy class or explicit retention override, after retention enforcement is designed
- safe audit fields such as actor ID, action type, incident ID, policy version, and non-sensitive outcome

Future client or protocol metadata should hold values that do not need server enforcement or should remain encrypted where practical:

- user-facing tags, local labels, timeline markers, and detailed notes
- platform permission choices and local recording UI state
- jurisdiction-specific guidance text, which must avoid legal advice
- plaintext note content, transcripts, media descriptions, and sensitive context
- local notification wording and device-only state until server delivery is explicitly designed

Fields that affect access, wrapped-key release, token creation, notification, retention, deletion, or public viewer wording belong in a reviewed protocol/server design. Fields that are only display or evidence context should be minimized, encrypted where practical, and omitted from public summaries unless deliberately shared.

## Access And Sharing Direction

Future account-enabled clients should distinguish:

- account-owner access to their own incident data
- capture-device upload authority for one account and incident
- trusted-contact access granted by the account owner or by an explicit escalation policy
- public-link access for a single incident
- administrative/operator access, which should not casually expose user safety data
- optional escrow or break-glass access, only if separately configured and audited
- legal/export workflows controlled by the account owner

Incident labels must not silently grant access. Emergency incidents, interaction records, safety checks, and evidence notes need explicit sharing, escalation, and grant policy before implementation.

The current token-scoped incident viewer is a temporary read-only access model.
A future web client may replace it after public account workflows,
authorization, key custody, and trusted-contact access are designed. The future
`/v1` role, grant, and route-exposure direction is documented in
[v1-access-control.md](v1-access-control.md).

## Migration From Generic Incidents

The migration path from the current backend should remain additive and
conservative:

1. Keep existing incidents readable as generic legacy incidents with no incident-mode value.
2. Do not backfill old incidents as emergencies, interaction records, safety checks, or evidence notes without an account-owner-controlled classification flow.
3. Keep the initial server fields nullable and optional in SQLite, PostgreSQL,
   and main `/v1` API responses.
4. Keep old clients working against generic incident creation until an explicit API version or compatibility plan replaces it.
5. Keep current public viewer and bundle behavior tolerant of missing mode,
   capture-profile, escalation, and sharing fields.
6. Do not infer key access, trusted-contact grants, public links, or retention windows from a legacy generic incident.

The current authenticated `POST /v1/incidents` route accepts the initial optional mode
metadata fields documented in [API](api.md). Future fields or mode-driven
behavior still require an explicit protocol/API compatibility decision.

## API Compatibility And Viewer Wording

Future API changes should keep current behavior clear:

- Current `/v1/incidents` creates generic incidents by default and accepts
  optional mode metadata.
- Future public product API routes must wait for implemented authentication and authorization.
- Future private/admin routes must remain on private listener groups and still require authentication after the future admin API exists.
- Public incident viewer routes must stay read-only and must not become write, grant-management, admin, escrow, or decryption routes.
- Bundle manifests may eventually include non-secret incident-mode summaries, but they must not include raw tokens, raw keys, plaintext, private deployment details, server paths, object keys, or unreviewed sensitive context.

Viewer wording should become mode-aware only after mode-driven viewer behavior is
explicitly implemented. Interaction records and evidence notes should not use
emergency-only copy. Safety-check wording should explain the missed-check-in
context without implying emergency services were contacted. Emergency incidents
can use urgent trusted-contact guidance only when the escalation policy actually
grants urgent access.

## Retention And Deletion Implications

Incident modes may influence retention defaults, but they do not override evidence-preservation and deletion controls by label alone.

Future design should decide:

- whether emergency incidents, interaction records, safety checks, and evidence notes need different default retention windows
- whether safety-check retention changes after the check is completed, canceled, or missed
- how legal/export state interacts with deletion, tombstones, backups, and revocation
- how retention applies to wrapped keys, public links, trusted-contact grants, and bundle manifests
- what audit fields can be retained without leaking raw tokens, raw keys, plaintext, request bodies, uploaded bytes, or private deployment details

The current backend implements generic incident deletion and optional
closed-incident retention, but it does not implement mode-specific retention or
mode-specific deletion behavior. Future work should align with
[mode-aware retention policy](mode-aware-retention-policy.md),
[retention-backup-deletion.md](retention-backup-deletion.md) and
[incident-deletion-retention-enforcement.md](incident-deletion-retention-enforcement.md).

## Dependencies Before Implementation

The optional metadata fields are not enough to implement mode-specific product
behavior. Mode-driven access, escalation, retention, sharing, viewer, or
key-custody behavior depends on other designs:

- [Future `/v1` access control](v1-access-control.md) for account-owner, capture-device, trusted-contact, public-link, admin/operator, and optional escrow roles
- [Key custody and emergency access](key-custody.md) for contact-wrapped keys, wrapped-key delivery, and phone-unavailable assumptions
- [Browser-side decryption](browser-decryption.md) before any web viewer decrypts evidence
- [Break-glass key access](break-glass-key-access.md) before any server-assisted emergency key access exists
- client and protocol repository planning before mobile or shared protocol behavior is implemented outside this server repository
- notification delivery design before push, SMS, Messenger, email, or other trusted-contact delivery is added

Any implementation that changes key custody, wrapped-key delivery, browser decryption, server-side decryption, or break-glass access is separate security-sensitive work and must update the security model, threat model, encryption docs, operational guidance, tests, and deployment warnings before or alongside code.

## Current Implementation Status

Implemented today:

- generic incidents
- optional nullable incident-mode, capture-profile, escalation-policy, and
  sharing-state metadata on main incident create/read routes
- media streams
- encrypted chunk upload and immutable storage
- checkins
- incident tokens
- read-only incident viewer
- encrypted ZIP evidence bundles
- simulator upload and decrypt-verification flows

Not implemented today:

- mode-driven access, capture, escalation, sharing, retention, viewer, or
  key-custody behavior
- public account workflows
- public `/v1` product authentication
- trusted-contact accounts
- mobile clients
- non-emergency interaction UX
- dead-man switch notifications
- push/SMS/Messenger integrations
- production key custody
- browser/client-side decryption
- legal export workflow

## Documentation And Review Rules

When future implementation touches incident modes, update the relevant source-of-truth docs together:

- [README](../README.md), to keep the top-level implemented/future scope clear
- [Architecture](architecture.md), to show any new data flow, listener, or repository boundary
- [API](api.md), to document any accepted route, request, response, viewer, or bundle-manifest field
- [iOS local recorder prototype](ios-local-recorder-prototype.md)
- [/v1 access control](v1-access-control.md)
- [Security model](security-model.md), to preserve storage, logging, listener, access, and ciphertext-only assumptions
- [Threat model](threat-model.md), to cover mode-specific sharing, escalation, false-positive, and access risks
- [Key custody](key-custody.md), if sharing, wrapped-key delivery, or decryption behavior changes
- [Mode-aware retention policy](mode-aware-retention-policy.md), [retention, backup, and deletion](retention-backup-deletion.md), and [incident deletion and retention enforcement](incident-deletion-retention-enforcement.md), if mode-specific retention behavior changes
- [Browser-side decryption](browser-decryption.md) and [break-glass key access](break-glass-key-access.md), if a mode affects decryption or emergency key access

New ideas discovered while documenting incident modes should become backlog items unless they are required for the scoped task.
