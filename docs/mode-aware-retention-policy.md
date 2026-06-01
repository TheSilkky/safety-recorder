# Mode-Aware Retention Policy Design

This document defines the planning boundary for future retention policy that
depends on incident mode, capture profile, escalation policy, sharing state, or
safety-check state.

Current Proofline behavior is generic. Optional `incident_mode`,
`capture_profile`, `escalation_policy`, and `sharing_state` fields are stored as
metadata only. They do not delete evidence, grant access, send notifications,
release wrapped keys, change key custody, change public viewer behavior, or
imply emergency response.

## Goals

- Preserve the current evidence-preserving default until a future policy is
  explicitly configured and tested.
- Define future retention inputs for emergency incidents, interaction records,
  safety checks, and evidence notes.
- Keep account-owner choices, admin/operator overrides, backup expiry, deletion
  tombstones, public links, trusted-contact grants, wrapped keys, and bundle
  manifests in one policy model.
- Require dry-run and private review before mode-aware retention creates
  deletion decisions.
- Keep public viewer routes read-only and fail-closed.
- Avoid introducing notifications, key custody, browser decryption, backend
  decryption, public account workflows, or public `/v1` exposure.

## Non-Goals

- No implementation of mode-specific retention behavior in this issue.
- No change to generic incident deletion or `SAFE_CLOSED_INCIDENT_RETENTION`.
- No trusted-contact accounts, notification delivery, missed-check-in timers,
  public product API routes, public admin dashboard, browser decryption, backend
  decryption, key escrow, or raw server-held media keys.
- No automatic deletion, access grant, wrapped-key release, public-link
  creation, or emergency-services contact from a mode label alone.
- No deletion of backups, browser downloads, reverse-proxy caches, endpoint
  copies, or persisted derived exports outside backend control.

## Current Baseline

Implemented today:

- generic incident retention by default
- explicit private owner-scoped and admin-global incident deletion requests
- optional closed-incident retention through `SAFE_CLOSED_INCIDENT_RETENTION`
- optional expired/revoked viewer-token metadata pruning
- optional completed tombstone pruning
- local read-only retention preview and deletion status operator commands
- public viewer fail-closed behavior for deleting and deleted incidents

Not implemented today:

- mode-specific retention windows
- safety-check timer state
- trusted-contact grants
- wrapped-key retention policy
- legal/export lifecycle state beyond optional metadata labels
- public product API or public account workflows

## Policy Inputs

Future mode-aware retention should use explicit policy inputs, not labels alone:

| Input | Example values | Retention use |
|---|---|---|
| Incident mode | emergency, interaction record, safety check, evidence note | Select the default policy class after owner review. |
| Capture profile | audio/video/location, location check-in, note or attachment | Decide whether failed/open streams and attachments inherit incident retention. |
| Escalation policy | none, trusted contacts on start, trusted contacts on missed check-in | Delay retention decisions while escalation or review is active. |
| Sharing state | private, trusted-contact access, public link, legal export, revoked/expired | Preserve grant, token, export, and audit metadata long enough for review. |
| Safety-check state | active, completed, canceled, missed, false alarm | Select completion and missed-check handling without adding notifications here. |
| Owner retention choice | default, shorter, longer, hold | Let the account owner choose visible policy when product UX exists. |
| Admin/operator hold | support hold, legal hold, abuse response hold | Prevent deletion until a private audited hold is removed. |
| Backup policy | backup generation and expiry window | Coordinate live deletion with restore expectations and backup expiry. |
| Key custody state | no wrapped keys, wrapped keys available, wrapped-key access revoked | Keep ciphertext and wrapped-key metadata lifecycles aligned. |

Every policy input must be explicit, validated, and auditable. Missing fields
should fall back to the generic evidence-preserving behavior.

## Default Direction By Mode

These are design defaults, not implemented behavior.

| Future mode | Retention direction |
|---|---|
| Emergency incident | Preserve evidence by default, avoid automatic deletion while urgent escalation or trusted-contact review is active, and require clear owner/admin review before shortening retention. |
| Interaction record | Keep private by default, support shorter owner-visible retention where legally and personally appropriate, and treat sharing/export as a separate explicit action. |
| Safety check | Keep active and missed checks until the safety state is resolved. Completed or canceled checks may use shorter retention only after grace periods, false-alarm review, and backup policy are clear. |
| Evidence note | Allow shorter owner-visible retention for lightweight notes or attachments, but retain encrypted media and metadata consistently until explicit deletion or configured policy applies. |

Generic legacy incidents with no mode should not be inferred into one of these
classes. They should keep generic retention until an account-owner-controlled
classification flow exists.

## Safety-Check State

Safety checks need state beyond `incident_mode` before mode-aware retention can
be safe:

- active check
- completed check
- canceled check
- missed check-in
- false alarm or resolved missed check
- escalation review active
- escalation review resolved

Retention must not delete an active safety check, a missed check awaiting
review, or evidence needed to understand a false alarm. This design does not add
notifications, timers, trusted-contact workflows, or emergency-services
integration. It only states that future retention must wait for explicit
safety-check state before applying shorter windows.

## Sharing, Export, Grants, And Links

Retention policy must distinguish capture from sharing and export:

- Public links are bearer-token access grants. Token rows store hashes, not raw
  tokens, and must keep expiry/revocation behavior independent from incident
  mode labels.
- Trusted-contact grants, when implemented, should have their own expiry,
  revocation, audit, and key-access semantics.
- Legal/export state should record that an owner deliberately exported or
  shared evidence. It must not make the backend promise legal admissibility or
  retain plaintext exports that the backend does not create.
- Downloaded bundles, browser caches, reverse-proxy caches, endpoint copies,
  and legal exports outside backend control need separate user/operator
  guidance. Backend deletion cannot erase them.

Mode-aware retention should not delete grant audit metadata earlier than the
deployment's reviewed audit window. It also should not keep raw tokens,
Authorization headers, request bodies, uploaded bytes, plaintext, raw keys,
stored paths, object keys, private deployment details, original filenames,
location values, or incident notes in public logs or public issue text.

## Wrapped Keys And Key Custody

Future wrapped-key metadata affects retention because ciphertext may be useless
to a trusted contact if the relevant wrapped keys are deleted too early. The
default direction is:

- Keep ciphertext, bundle manifests, and wrapped-key metadata aligned for the
  same incident/stream retention window.
- Removing a trusted contact should stop future access and future wrapping, but
  older wrapped keys need a separate revocation and retention design.
- Deleting or withholding wrapped keys must be an explicit policy decision, not
  an incidental side effect of a mode label.
- Backend decryption, browser decryption, key escrow, and break-glass access
  remain separate security-sensitive designs.

If future policy deletes wrapped-key metadata while retaining ciphertext, docs
must warn that trusted-contact access may become unavailable even though
encrypted blobs remain.

## Owner Choices And Operator Overrides

Future account-owner UX should make retention choices visible:

- current policy class
- live retention window, if configured
- backup expiry caveat
- public-link and trusted-contact grant implications
- deletion pending/deleted state
- whether a legal/export or operator hold blocks deletion

Admin/operator overrides should be narrow and audited. A private operator may
need to hold evidence for support, legal process, abuse response, restore
reconciliation, or safety review, but those holds must not expose plaintext,
raw keys, raw tokens, or user safety data casually. Private network placement is
not a substitute for authentication and authorization.

## Dry-Run Requirements

Mode-aware retention must have a dry-run path before live deletion:

- show safe counts grouped by policy class and mode
- include incident IDs and timestamps only when the command is local/private
- show whether public links, trusted-contact grants, wrapped keys, deletion
  decisions, or backup caveats affect eligibility
- avoid stored paths, object keys, raw tokens, request bodies, uploaded bytes,
  plaintext, raw keys, original filenames, location values, notes, private
  deployment details, and user safety data
- refuse to create deletion decisions when policy inputs are missing or
  inconsistent

The existing closed-incident retention preview is the model: it reports safe
counts and IDs without mutating state.

## Deletion, Tombstones, And Backup Interaction

Mode-aware retention should create ordinary deletion decisions through the same
durable deletion queue as current incident deletion. It should not delete blobs
or rows through a separate path.

Policy rules:

- Open incidents are not eligible for automatic retention deletion.
- Active safety checks and unresolved missed checks are not eligible for
  automatic retention deletion.
- Failed streams stay with their parent incident unless a later issue designs
  stream-level deletion semantics.
- Minimal tombstones should retain only non-sensitive fields needed for
  idempotency, audit, and restore reconciliation.
- Tombstone pruning must wait until deletion is complete and retry state is
  gone.
- Backup expiry is a separate lifecycle. Live deletion does not delete older
  SQLite backups, PostgreSQL backups, S3 object versions, snapshots, downloaded
  bundles, endpoint copies, or derived exports.

Restore drills must account for mode-aware policy. Restoring an older backup may
resurrect incidents that were deleted later unless backup expiry, restore
reconciliation, or storage-key retirement prevents it.

## Public Viewer Behavior

Public incident viewer routes remain read-only. They must not expose retention
policy controls, deletion controls, grant controls, admin/operator state,
trusted-contact account workflows, wrapped-key management, or reassignment
state.

When an incident is deleting, deleted, expired by policy, or otherwise
unavailable, public viewer routes should fail closed with the same generic
public behavior used for invalid, expired, or revoked viewer tokens. Mode-aware
retention must not reveal whether an incident was an emergency, an interaction
record, a safety check, an evidence note, or subject to an operator hold.

## Implementation Requirements

A future implementation issue should include:

- explicit configuration for mode-aware policy classes, disabled by default
- SQLite and PostgreSQL parity for policy state and deletion decisions
- dry-run coverage before live deletion
- tests for each mode and safety-check state
- tests for public viewer fail-closed behavior
- tests for owner/admin authorization and operator holds
- tests for token metadata, tombstone pruning, and backup/restore docs
- security-model, threat-model, API, deployment, and retention docs updates
- clear changelog entry and operational warnings

Until that implementation exists, the current backend remains generic and
evidence-preserving by default.
