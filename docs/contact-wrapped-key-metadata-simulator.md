# Contact-Wrapped Key Metadata Simulator Prototype

This document scopes a simulator/development prototype for contact-wrapped key
metadata. It does not add backend decryption, server-held raw media keys, key
escrow, browser decryption, public account workflows, production
trusted-contact access, or client implementations. The backend now has
production-shaped private metadata routes for contact public keys, sharing
grants, and grant-bound wrapped-key records, but the simulator artifact remains
a local development companion and is not placed in bundle manifests.

The goal is to let Proofline test the shape of contact-wrapped media-key
metadata before production key custody is implemented. The current backend
continues to receive opaque encrypted chunks, validate SHA-256 over ciphertext,
store encrypted blobs, and generate encrypted ZIP evidence bundles.
The production contact public-key lifecycle, grant model, and server-stored
wrapped-key metadata boundary are designed separately in
[contact-key-sharing-grants.md](contact-key-sharing-grants.md).

## Goals

- Model trusted-contact public keys and non-secret key identifiers in
  development flows.
- Wrap simulator stream media keys for one or more model contacts without
  exposing raw keys to the backend.
- Define safe metadata that can appear in local development manifests or test
  artifacts.
- Separate non-secret key IDs from raw media keys, contact private keys, and
  plaintext.
- Show how wrapped-key metadata relates to stream bundle manifests, stream
  media keys, and future trusted-contact access.
- Evaluate stable, documented cryptographic formats or libraries before any
  production implementation.

## Non-Goals

- No production key custody implementation.
- No server-side decryption, browser decryption, key escrow, or break-glass
  behavior.
- No raw server-held media keys or contact private keys.
- No public `/v1` exposure and no authentication model changes.
- No web, iOS, Android, or shared protocol repository implementation.
- No custom public-key wrapping, KDF, AEAD, padding, MAC, random generator, or
  secret-sharing primitive.
- No changes to current bundle ZIP entry naming or stored blob paths.

## Current Boundary

The current simulator uses the documented v1 AES-256-GCM envelope and one
development key per run or key file. That key is client-side simulator state.
The backend does not parse encryption headers for trust decisions, does not
store media keys, and does not decrypt chunk bytes.

The first contact-wrapped prototype should preserve that boundary:

- encrypted chunks upload through the existing main `/v1` flow
- bundle downloads remain encrypted ZIP bundles
- backend bundle manifests continue to omit keys
- simulator wrapped-key artifacts remain local unless the caller explicitly uses
  the authenticated server metadata routes added for wrapped-key records

## Prototype Model

The simulator can model three local concepts:

| Concept | Prototype treatment |
|---|---|
| Stream media key | Symmetric key used to encrypt chunks for one media stream. The current simulator key can stand in for this at first; a later simulator change can create one key per stream. |
| Contact key pair | Development-only public/private key pair for a model trusted contact. Public metadata can be shared; private key material stays in local simulator files only. |
| Wrapped media key | Encrypted copy of the stream media key for a contact public key. This is encrypted key material, not a raw key, but it should still be treated as sensitive access metadata. |

The simulator should assign non-secret IDs separately from key material:

- `media_key_id`: identifies the stream media key referenced by encrypted
  chunk headers and wrapped-key records
- `contact_id`: identifies a model trusted contact in local simulator state
- `contact_key_id`: identifies the contact public key used for wrapping
- `wrapped_key_id`: identifies one wrapped media-key record

IDs can appear in manifests and test output. Raw media keys, contact private
keys, unwrapped keys, plaintext, and decryption capabilities must not appear in
manifests, logs, command output, or server storage.

## Local Prototype Flow

1. The simulator creates an incident and media stream using the existing private
   API.
2. The simulator creates or loads a stream media key. For the first prototype,
   this may reuse the existing local simulator key file format; a follow-up can
   move to one key per stream.
3. The simulator loads a local development contact registry containing contact
   public keys and local private keys for test unwrapping. Private keys must be
   stored only in local files with restrictive permissions where practical.
4. Before or after uploading encrypted chunks, the simulator wraps the stream
   media key for selected contact public keys.
5. The simulator writes a local development manifest or test artifact that
   references the incident ID, stream ID, media key ID, contact key IDs, wrapping
   algorithm, and wrapped-key ciphertext.
6. Bundle verification can use the local contact private key to unwrap the media
   key, then decrypt the downloaded bundle locally. This remains a simulator
   smoke test, not backend behavior.

The first implementation should prefer a local artifact alongside simulator
output, for example `proofline-sim-wrapped-keys.json`, instead of changing
server bundle manifests. This keeps the prototype useful while avoiding a
server schema or API commitment. Keep these artifacts in ignored local paths or
temporary directories; do not commit them.

The implemented simulator flow is opt-in with `--wrapped-key-output`. It creates
or loads a local development contact key file, wraps the simulator media key
with the maintained `filippo.io/age` library using the `age-v1-x25519` profile,
writes a local companion artifact, and verifies downloaded bundle decryption by
reading the artifact and unwrapping through the development contact key. If
`--contact-key-file` is omitted, the simulator uses
`proofline-sim-contact.key.json` next to the wrapped-key artifact. Both files are
local development state and should stay in ignored paths.

## Development Manifest Shape

A local simulator manifest can use a shape like this:

```json
{
  "version": 1,
  "scope": "simulator-development",
  "incident_id": "inc_...",
  "stream_id": "str_...",
  "media_key_id": "kid_...",
  "created_at": "2026-05-29T00:00:00Z",
  "wrapped_keys": [
    {
      "wrapped_key_id": "wkey_...",
      "recipient_type": "trusted_contact",
      "contact_id": "contact_dev_alex",
      "contact_key_id": "ckid_...",
      "wrapping_algorithm": "age-v1-x25519",
      "wrapped_key_b64": "base64url-wrapped-media-key-ciphertext"
    }
  ]
}
```

The manifest may include encrypted wrapped-key ciphertext and public wrapping
metadata. It must not include:

- raw media keys
- contact private keys
- unwrapped shared secrets
- plaintext chunk data
- viewer or incident tokens
- filesystem paths, object-store keys, or staging paths
- secret-bearing URLs

## Relation To Bundle Manifests

Current stream and incident bundle manifests include a non-secret encryption
hint with `server_decrypts: false`. They do not include keys or wrapped keys.

The simulator prototype treats wrapped-key metadata as a companion artifact.
Server-side wrapped-key records now use authenticated private `/v1` API
responses and refer to the same `incident_id`, optional `stream_id`, and
`media_key_id` used by encrypted chunks. Bundle manifests remain key-free. Any
future bundle-manifest exposure would still need an explicit grant-scoped
design, tests, retention and backup guidance, threat-model updates, and
operational guidance before implementation.

Wrapped-key records must remain distinct from access grants. A viewer token or
future account grant may authorize download of encrypted evidence and wrapped
metadata, but decryption should also require the matching contact private key or
another explicit decryption capability.

## Cryptographic Evaluation

The simulator implementation currently uses the age v1 X25519 recipient format
through `filippo.io/age`. That library owns the public-key wrapping, KDF, AEAD,
message layout, authentication, encoding, and random generation used by the
wrapped-key ciphertext. The simulator records the profile as `age-v1-x25519` and
rejects unsupported algorithms when reading artifacts.

Other candidate directions for future production work remain:

- HPKE using a maintained implementation and a documented ciphersuite.
- An `age`-style recipient stanza for development artifacts if the prototype
  favors a file-oriented format.
- A platform-compatible ECDH plus HKDF plus AEAD or key-wrap profile only if it
  is a reviewed standard profile implemented through stable libraries or
  platform APIs.

The current Go toolchain provides documented primitives such as `crypto/ecdh`,
`crypto/hkdf`, `crypto/rand`, and the existing AES-GCM code path. Those
primitives are not enough by themselves to justify an ad hoc wrapping protocol.
The design should choose or reference a reviewed profile rather than inventing
message layout, key derivation context, or authentication rules locally.

For future browser or mobile compatibility, the prototype should record:

- supported curves or recipient key types
- key ID derivation rules
- wrapped-key associated metadata
- ciphertext encoding
- whether the format can be parsed by Web Crypto, CryptoKit, Swift Crypto, or a
  future native trusted-contact client
- how unsupported algorithms fail closed

## Server Metadata

Server storage of wrapped or encrypted media-key metadata is now limited to
grant-bound records created through authenticated private routes. A
server-side metadata record stores:

- incident ID and stream ID
- media key ID
- recipient contact or grant identifier
- contact public key ID and version
- wrapping algorithm and version
- wrapped-key ciphertext
- required public wrapping metadata, such as an ephemeral public key
- creation time and rotation or revocation status

It must not store raw media keys, contact private keys, plaintext, unwrapped
shared secrets, browser fragment secrets, raw tokens, or server escrow
material. It must not log wrapped-key ciphertext because that metadata is
access-enabling even when it is not a raw key.

## Validation

For the design-only milestone:

- run `git diff --check`
- manually review this document against `docs/key-custody.md`,
  `docs/encryption.md`, `docs/browser-decryption.md`,
  `docs/ios-local-recorder-prototype.md`, `docs/security-model.md`, and
  `docs/threat-model.md`

For simulator implementation:

- add tests that prove raw media keys and contact private keys are not written
  to server manifests, logs, or bundle ZIP entries. Implemented for current
  server bundle entries and manifests.
- add tests for malformed wrapped-key metadata and unsupported algorithms.
  Implemented for local simulator artifacts.
- verify downloaded bundles can be decrypted locally after unwrapping with a
  development contact private key. Implemented for simulator bundle verification
  when `--wrapped-key-output` is set.
- run `gofmt -w ./cmd ./internal ./migrations`
- run `go test ./...`
- run the simulator smoke flow with bundle download and local decryption

## Open Questions

- Should the first simulator code use one media key per stream immediately, or
  keep the current one-key-per-run behavior while modeling `media_key_id`?
- Which wrapping format best balances Go simulator simplicity, future browser
  support, and future mobile support?
- How should contact public keys be verified and rotated in a future account
  model?
- Should late-added contacts receive wrapped keys for already completed streams,
  future streams only, or both?
- Which wrapped-key metadata fields should be included in encrypted evidence
  bundles versus kept behind future account-authorized API responses?
