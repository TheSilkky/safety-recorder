# Contact Key Sharing, Grants, And Wrapped-Key Metadata

This document designs the contact key-sharing model for Proofline. The current
backend implements the server metadata steps for this model: account owners can
create account-to-account trusted-contact relationship invites, register
trusted-contact public-key metadata, replace or mark contact keys revoked/lost,
create or revoke incident/stream-scoped sharing grants, and store or revoke
grant-bound wrapped-key records through authenticated private `/v1` routes.
Signed-in accepted trusted contacts can read wrapped-key records only when an
active relationship, recipient-bound active contact key, active unexpired
ciphertext grant, and active wrapped-key record all authorize the request. It
does not add bundle wrapped-key fields, browser decryption, backend decryption,
server escrow, public account workflows, notifications, client code, or
production key custody behavior.

The design connects the long-term key custody direction in
[key-custody.md](key-custody.md), the role and grant boundaries in
[v1-access-control.md](v1-access-control.md), and the simulator-only wrapped-key
metadata prototype in
[contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md).

## Security Boundary

The backend remains ciphertext-only by default:

- clients encrypt media before upload
- the server stores encrypted chunks and metadata
- the server may store recipient public-key records and wrapped CEK ciphertext
- the server must not store raw CEKs, raw media keys, recipient private keys,
  plaintext, unwrapped shared secrets, browser fragment secrets, or
  server-decryptable key material in the default contact-sharing path
- server escrow, break-glass key access, browser decryption, and backend
  decryption remain separate security-sensitive designs

Access to an incident is not the same as decryption capability. A grant can
authorize metadata, ciphertext, wrapped-key delivery, or some combination of
those data classes. Decryption still requires the matching contact private key,
client key, or another explicit future decryption capability.

## Model

The future model should keep these concepts separate:

| Concept | Purpose | Security treatment |
|---|---|---|
| Account owner | Owns incidents, contacts, sharing policy, and revocation decisions. | Authenticated product actor; can create and revoke grants for owned incidents. |
| Trusted contact | A person authorized by the account owner or escalation policy. | Receives only grant-scoped metadata, ciphertext, and wrapped keys. |
| Recipient public-key record | Versioned public key material for an account, device, trusted contact, or future escrow target. | Server-visible metadata, but still privacy-sensitive because it links recipients and sharing history. |
| Recipient private key | Secret key controlled by the account, device, trusted contact, or future escrow holder. | Never stored, logged, backed up, or handled by the server in the default contact-sharing model. |
| CEK | Content-encryption key for an incident, stream, or bounded chunk group. | Never stored raw by the server in the default model. Existing `media_key_id` fields identify this key material until a protocol/API naming migration is accepted. |
| Access grant | Authorization record for an actor, incident or stream, data classes, expiry, and state. | Does not itself contain decryption material. |
| Viewer token | Bearer public-link capability for the read-only incident viewer. | Separate from trusted-contact identity and grants; not a general `/v1` credential. |
| Wrapped-key record | Encrypted copy of a CEK for a recipient public-key record. | Access-enabling encrypted metadata; deliver only under explicit policy. |

Do not model an incident as its own long-term private-key identity. Long-term
private keys belong to accounts, devices, and trusted contacts. Incidents,
streams, and bounded chunk groups own CEKs, and wrapped-key records connect
authorized recipient key versions to those CEKs.

Prototype data or test fixtures that assumed per-incident private keys should
be migrated or regenerated against this model when they are touched. Do not add
unnecessary dual-stack compatibility just to preserve that prototype shape; use
an explicit migration issue if old local data or fixtures need support.

## Contact Public-Key Lifecycle

Contact key registration should be account-owner controlled and deny by
default.

Registration requirements:

- record a stable `contact_id` for the trusted contact relationship
- record a `contact_public_key_id` and monotonically increasing version for
  each public key
- record the wrapping profile, public key material, creation time, verification
  state, and non-sensitive display metadata
- require explicit account-owner approval before a key can receive wrapped
  CEKs
- provide an out-of-band verification step, such as comparing a short
  fingerprint or safety number, before marking a key trusted

The server must not verify a contact key by trusting user-supplied names,
phone numbers, email addresses, or unverified profile metadata alone. Any
future notification or invitation channel is a contact-discovery convenience,
not proof that the public key belongs to the intended person.

Suggested contact key states:

- `pending_verification`: registered but not trusted for wrapping
- `active`: verified and eligible for new wrapping
- `replaced`: superseded by a newer key version
- `revoked`: no longer eligible for new grants or new wrapping
- `lost`: contact reports private-key loss; not eligible for new wrapping

The implemented `/v1/contact-public-keys/{public_key_id}/replace` route creates
a successor version for the same contact and marks prior nonterminal versions
`replaced`. `/lost` records contact private-key or device loss. Replacement,
revocation, and lost-key states are terminal for future grants and wrapped-key
records. They do not mutate old wrapped-key records; old records remain bound
to the original contact public-key ID and version, and current delivery filters
omit them once the referenced key is no longer active. Rewrapping older CEKs is
possible only when an authorized client or reviewed future service still has
access to the raw CEK; the server must not invent a rewrap path by decrypting
existing wrapped-key ciphertext.

## Grants

Grants authorize access. Wrapped keys enable decryption. They should be
separate records with separate state transitions.

Grant fields should include:

- `grant_id`
- account owner ID
- recipient actor type, such as trusted contact, owner device, public link, or
  optional future escrow actor
- recipient ID, such as `contact_id`, device ID, token ID, or escrow policy ID
- incident ID
- optional stream ID
- authorized data classes, such as metadata, ciphertext bundle, wrapped-key
  metadata, or wrapped-key ciphertext delivery
- creation actor and creation time
- expiry time, if any
- grant state, such as active, expired, revoked, superseded, or pending policy
- non-sensitive reason category or policy version

Incident-scoped grants can cover all current and future streams in an incident
only when the account owner or escalation policy explicitly chooses that scope.
Stream-scoped grants should expose only the named stream. Mode labels, capture
profiles, escalation-policy names, or sharing-state summaries must not silently
grant access.

Public viewer tokens remain separate from trusted-contact grants. A public
viewer token may authorize read-only metadata and encrypted bundle download for
one incident, as the current viewer does. It should not deliver trusted-contact
wrapped keys by default. Any future decryption-bearing public link must be a
separate, explicitly reviewed capability because bearer links are easily
forwarded.

## Revocation And Late Contacts

Revocation stops future authorization and future delivery. It cannot erase
plaintext, ciphertext, bundle files, or wrapped keys that an authorized contact
already downloaded.

Rules:

- revoking a grant stops future metadata, ciphertext, and wrapped-key delivery
  through that grant
- revoking a contact stops new grants and new wrapped-key records for that
  contact
- revoking or replacing a contact key stops new wrapping to that key version
- existing wrapped-key records should be marked revoked, superseded, or
  retained for audit, according to policy
- deleting an incident removes or tombstones its grants and wrapped-key records
  according to the deletion and backup policy

Late-added contacts should not automatically receive old incident keys. The
account owner must explicitly choose whether the new contact receives access to
existing incidents or only future incidents. If the owner's client no longer
has the raw CEKs and no explicit escrow mode exists, the backend cannot
produce new wrapped keys for old evidence.

CEK rotation should use new `media_key_id` values until a protocol/API naming
migration introduces a CEK-named field. A stream can have one CEK, or later
designs may use key generations for long-running streams.
Wrapped-key records must identify the exact CEK or generation they wrap.
Revoking a contact does not rotate already uploaded ciphertext; future clients
may rotate CEKs after revocation to limit future exposure.

## Wrapped-Key Records

A wrapped-key record is encrypted key material plus public wrapping metadata.
It is not raw key material, but it can enable access when combined with the
matching private key and ciphertext evidence. Treat it as sensitive metadata.

Server-stored fields should include:

- `wrapped_key_id`
- incident ID
- optional stream ID, or incident scope
- `media_key_id` and optional CEK generation
- recipient type
- `grant_id` or recipient/grant binding identifier
- contact ID, when recipient type is trusted contact
- contact public key ID and version
- wrapping algorithm and version
- wrapped-key ciphertext
- required public wrapping metadata, such as an ephemeral public key, key
  encapsulation value, salt, or associated-data profile
- creation actor and creation time
- rotation, supersession, revocation, and deletion state

Server-stored wrapped-key records must not include:

- raw CEKs
- raw media keys
- recipient private keys
- plaintext
- unwrapped shared secrets
- browser fragment secrets
- server-held key-encryption keys for the default contact-sharing model
- request bodies, uploaded bytes, stored paths, staging paths, object keys, or
  private deployment details

The first production wrapping format for v1 preview is the accepted
post-quantum profile in [post-quantum-envelope.md](post-quantum-envelope.md).
Future runtime code should store profile records through the existing fields
with:

```text
wrapping_algorithm = proofline-pq-mlkem768-hkdfsha384-aes256gcm
wrapping_algorithm_version = 1
```

The current routes remain generic metadata storage and delivery routes until
runtime implementation adds profile validation. They must not be described as
cryptographic validators merely because they can already store those strings.

The wrapping format must use stable, documented cryptographic libraries or
platform APIs. Do not implement custom public-key encryption, KDF, AEAD,
padding, MAC, random generator, or secret-sharing primitives in this repository.
Do not invent alternate undocumented encodings for wrapped-key records. Any
future suite that replaces the accepted PQ profile needs explicit identifiers,
compatibility tests, migration notes, and future mobile or trusted-contact
client review.

## Delivery

Wrapped-key delivery should be policy-driven and explicit.

Current implementation:

- authenticated owner API responses deliver only active records for the
  authenticated account's owned incident
- authenticated trusted-contact API responses deliver only active records whose
  owner relationship, recipient-bound contact key, grant, and wrapped-key record
  authorize the signed-in recipient account
- records are omitted when the sharing grant is revoked or expired, the contact
  public key is no longer active, or the wrapped-key record is revoked or
  rotated
- bundle manifests remain key-free
- public-link viewer bundles keep their current ciphertext-only behavior
- request logging must not log wrapped-key ciphertext, raw tokens, raw keys,
  request bodies, uploaded bytes, plaintext, object keys, or private deployment
  details

Including wrapped keys in evidence bundle manifests can make offline transfer
and legal export easier, but it also makes the bundle more access-enabling.
Keeping wrapped keys behind authenticated API responses gives the server a
clearer revocation point for future requests, but it is less useful for
offline handoff. The current backend chooses authenticated API delivery and
keeps bundle manifests unchanged. A future implementation may support both,
provided each response is scoped to an authorized grant and tests prove
unauthorized grants cannot see wrapped-key records.

## Server Metadata Versus Encrypted Client Metadata

Server metadata may store only the information needed to authorize, deliver,
audit, back up, restore, and delete wrapped-key records.

Appropriate server metadata:

- non-secret IDs
- contact public keys and verification state
- grant scope and state
- wrapping algorithm identifiers
- wrapped-key ciphertext
- public wrapping metadata
- safe audit categories

Appropriate encrypted client metadata:

- sensitive contact labels, relationship context, or safety narrative
- user-facing explanation of why a contact is trusted
- private escalation instructions
- notes that could endanger the user if exposed through server metadata

Server-visible display labels should be optional, minimal, and treated as
personal data. Public issue drafts, logs, metrics, support tickets, and
operator dashboards must not include user safety narratives or private contact
context.

## Retention, Backup, Restore, And Deletion

Wrapped-key records and grants must follow the incident lifecycle, but they
should not weaken evidence preservation or deletion behavior.

Rules:

- backups must include grant and wrapped-key metadata together with incident
  metadata so restored systems do not lose authorized decryption paths
- restore drills must check that grants, contact public-key versions, and
  wrapped-key records remain internally consistent
- incident deletion should remove or tombstone associated grants and
  wrapped-key records according to the deletion policy
- grant revocation should stop future delivery but may retain minimal audit
  metadata
- contact deletion should stop future wrapping and future contact-grant
  delivery, while preserving only the minimum audit data needed by policy
- expired grants and stale wrapped-key records should be pruned only after a
  reviewed retention window

Wrapped-key ciphertext should not be logged even though it is encrypted. It is
access-enabling metadata and belongs in the metadata backend, backups, or
authorized responses only.

## Audit

The current backend stores private repository-level sharing audit events for
contact public-key registration, replacement, revocation, and lost-key marking;
sharing-grant creation and revocation; wrapped-key record creation and
revocation; and incident deletion-pruning of sharing and wrapped-key metadata.
No public route exposes these records.

Implemented audit fields:

- timestamp
- actor account ID, or the owner account ID for owner-scoped automatic
  pruning decisions that have no explicit account actor
- action type
- incident ID
- optional stream ID
- grant ID
- contact ID or contact public key ID
- wrapped-key record ID
- deletion decision ID
- decision or outcome

Audit records must not include raw viewer tokens, raw incident tokens, raw
session tokens, Authorization headers, request bodies, uploaded bytes,
plaintext, raw keys, contact private keys, unwrapped shared secrets, wrapped
key ciphertext, public wrapping metadata, stored paths, staging paths, object
keys, private deployment details, or user safety narratives.

## Implementation Sequence

Implementation should stay split into narrow issues:

1. Implemented: add metadata schema and repository coverage for contact public keys and
   sharing grants behind the existing reviewed `/v1` boundary.
2. Implemented: add authenticated owner routes for contact key registration,
   verification, replacement, revocation, lost-key marking, and grant
   management behind the existing reviewed `/v1` boundary.
3. Implemented: add wrapped-key metadata schema and repository behavior without
   exposing raw CEKs, raw media keys, or recipient private keys.
4. Implemented: add owner-authenticated wrapped-key delivery through private
   API responses, with bundle manifests remaining key-free and tests proving
   unauthorized actors do not receive wrapped-key records.
5. Implemented: add account-to-account trusted-contact relationship lifecycle
   metadata for owner invite/revoke/replace and recipient accept/decline
   without granting key delivery.
6. Implemented: add signed-in trusted-contact grant-scoped wrapped-key read
   routes without changing token-only viewer routes, bundle manifests, or
   decryption behavior.
7. Future: add trusted-contact incident metadata, ciphertext bundle, and full
   viewer read routes only after the public product API exposure model is
   explicitly reviewed.
8. Future: optionally add grant-scoped bundle manifests only after a separate
   design and tests prove unauthorized actors do not receive wrapped-key records.
9. Update simulator/client tooling to generate production-shaped wrapped-key
   records using a reviewed wrapping profile.
10. Update deployment, security, threat-model, API, and retention docs before
   any public-authenticated contact route is exposed.

Each implementation issue must include tests for:

- deny-by-default authorization
- grant scope by account owner, incident, stream, data class, and recipient
- contact key replacement and revocation
- late-contact behavior
- CEK rotation or generation matching
- no raw CEKs, raw media keys, recipient private keys, plaintext, request
  bodies, uploaded bytes, raw tokens, stored paths, object keys, or private
  deployment details in logs, manifests, API errors, tests, or public
  documentation
- unchanged ciphertext-only backend behavior outside explicit future
  break-glass work

## Out Of Scope

- UI, trusted-contact accounts, and public product authentication.
- Creating web, iOS, Android, or protocol repository code.
- Backend decryption, browser decryption, server escrow, break-glass access,
  raw server-held media keys, playable media export, push notifications, SMS,
  Messenger, or emergency-services integration.
- Changing public incident viewer behavior or private/public listener
  boundaries.
