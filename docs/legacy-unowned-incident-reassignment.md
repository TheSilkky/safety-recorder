# Legacy Unowned Incident Reassignment Plan

This document plans a future private workflow for incidents created before local
account ownership existed. Current Proofline behavior is fail-closed: new
incidents created through private authenticated `/v1` routes are owned by the
creating account, regular users can access only their own incidents, and legacy
incidents with no `owner_account_id` are admin-only.

The current backend does not implement reassignment. This plan defines the
boundary for a future private admin API or local operator command.

## Goals

- Keep legacy unowned incidents admin-only unless an explicit private operator
  action assigns an owner.
- Give operators a safe way to identify unowned incidents without exposing
  evidence contents or sensitive evidence metadata.
- Require admin or local operator authorization for reassignment.
- Record safe audit metadata for each reassignment or quarantine decision.
- Preserve SQLite and PostgreSQL metadata parity.
- Preserve public incident viewer behavior, token hashing, deletion behavior,
  retention behavior, bundle generation, and ciphertext-only storage.

## Non-Goals

- No public account portal or public `/v1` exposure.
- No public admin dashboard.
- No bulk reassignment without per-incident operator review.
- No trusted-contact accounts, notifications, mode-driven access, key custody,
  browser decryption, backend decryption, key escrow, or raw server-held media
  keys.
- No change to public viewer routes, bundle contents, encrypted blobs, stored
  paths, object keys, or current token hashing.

## Current Behavior

The account migration added nullable `owner_account_id` metadata to incidents.
New authenticated incident creation stores the creating account ID. Current
private authorization is owner-or-admin:

- admins can access incidents across accounts
- regular users can access incidents only when `owner_account_id` matches their
  account ID
- incidents with no owner remain admin-only

That default is intentional. It prevents old data from becoming visible to a
regular account merely because local accounts now exist.

## Identifying Unowned Incidents

A future implementation should expose an admin-only or local-operator-only
review command that lists unowned incident candidates with minimal safe fields:

- incident ID
- status
- deletion state
- created and updated timestamps
- stream count
- chunk count
- checkin count
- incident token row count
- whether any viewer tokens are currently unexpired and unrevoked
- optional mode metadata values, if already present

The candidate list must not include raw viewer tokens, raw session tokens,
stored paths, object keys, uploaded bytes, plaintext, raw keys, private
deployment details, `original_filename` values, location coordinates, notes,
request bodies, Authorization headers, or user safety data.

If an operator needs deeper evidence review before selecting an owner, that
review should happen through existing private admin-capable incident access,
inside the private deployment boundary. The reassignment candidate list and
audit output should stay count-oriented.

## Reassignment Workflow

A future reassignment operation should be explicit and one incident at a time:

1. The operator lists unowned candidates from a private admin route or local
   operator command.
2. The operator selects one incident ID and reviews minimal safe metadata.
3. The operator chooses either:
   - assign the incident to a destination account
   - leave it unowned and quarantined for admin-only access
4. The operation verifies that the actor is an admin or trusted local operator.
5. The operation verifies that the destination account exists when assigning an
   owner.
6. The operation verifies that `owner_account_id` is still empty before writing,
   so concurrent reviews cannot silently overwrite an owner.
7. The operation records safe audit metadata and updates only
   `owner_account_id` when assigning.
8. The operator repeats the process for the next incident only after reviewing
   the previous result.

The operation should reject or require a separate explicit operator decision for
incidents in `deletion_pending`, `deleting`, or `deleted` states. Reassignment
must not be used to bypass deletion state, revive deleted incidents, or restart
failed deletion work.

## Quarantine Choice

Leaving an incident unowned is a valid reviewed outcome. A future quarantine
record can document that an admin reviewed the incident and intentionally kept
it admin-only. That record should be metadata-only and should not contain
incident notes, filenames, locations, stored paths, object keys, token values,
plaintext, raw keys, or private deployment details.

Quarantine does not need to change public viewer behavior. Existing valid
viewer tokens remain incident-scoped bearer links until expiry or revocation,
and invalid, expired, revoked, deleting, and deleted incidents keep the same
public failure behavior.

## Safe Audit Metadata

A reassignment or quarantine audit row should use controlled fields such as:

- audit event ID
- incident ID
- previous owner account ID, normally empty for this workflow
- new owner account ID, when assigned
- actor account ID or local operator identifier
- action, such as `assign_owner` or `keep_unowned`
- controlled reason code
- creation timestamp
- completion timestamp
- source, such as `admin_api` or `operator_cli`

Avoid free-form reason text in the first implementation. If a later workflow
needs notes, keep them short, private, and explicitly excluded from public
logs, public issue text, metrics labels, and PR descriptions.

Audit output and logs must not include raw viewer tokens, raw session tokens,
stored paths, object keys, uploaded bytes, plaintext, raw keys, private
deployment details, `original_filename` values, location values, incident notes,
request bodies, Authorization headers, or user safety data.

## Access, Tokens, Deletion, Retention, And Bundles

Reassignment changes private owner-scoped access only:

- After reassignment, the destination account can access the incident through
  normal owner-scoped private routes.
- Admins retain admin-wide access.
- Other regular users remain unauthorized.
- Existing viewer tokens are not regenerated and raw token values are not
  recoverable from stored hashes.
- Token expiry and revocation behavior is unchanged.
- Public viewer routes remain read-only and do not reveal reassignment state.
- Existing completed stream and incident encrypted ZIP bundle generation is
  unchanged.
- Existing deletion decisions and retention decisions are not cleared.
- After reassignment, the new owner can use the existing owner-scoped deletion
  route according to the normal deletion policy.

If a reassignment is needed for an incident that already has a deletion decision
or deletion failure, the operator should review deletion state first. The
implementation should avoid changing owner metadata and deletion state in the
same operation unless a future issue explicitly designs that combined recovery
case.

## SQLite And PostgreSQL Parity

Any implementation should add the same behavior to SQLite and PostgreSQL
repositories behind the metadata boundary. The repository operation should:

- update only incidents whose `owner_account_id` is currently null
- verify the destination account exists
- preserve incident ID, stream IDs, chunk metadata, token hashes, deletion
  state, and upload-operation state
- write safe audit metadata in the same transaction as the reassignment
- return a stable not-found or conflict error when the incident was already
  assigned, deleted, or otherwise not eligible

Tests should cover both backends where PostgreSQL integration is available:

- regular users cannot access legacy unowned incidents
- admins can identify legacy unowned incidents through the private review path
- reassignment requires admin or local operator authority
- reassignment grants access only to the selected owner account
- reassignment refuses to overwrite an existing owner
- reassignment preserves viewer-token behavior and bundle generation
- audit output excludes sensitive fields and raw token-like values

## Documentation Updates For Implementation

An implementation PR should update:

- `docs/api.md` for any new private admin route or local operator command
- `docs/security-model.md` and `docs/threat-model.md` for the reassignment
  trust boundary and audit fields
- `docs/code-map.md` for repository and handler flow
- `docs/retention-backup-deletion.md` if reassignment affects deletion or
  retention operations
- `docs/postgresql-metadata-migration.md` if migration guidance needs to point
  operators to the reassignment workflow
- tests and changelog entries for both SQLite and PostgreSQL behavior

Until that implementation exists, legacy unowned incidents remain admin-only.
