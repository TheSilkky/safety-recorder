# Pure Post-Quantum Encryption Envelope

Status: design and implementation plan only. This document does not change the
current runtime encryption envelope, backend storage behavior, viewer behavior, or
trusted-contact access model.

This document defines the intended future pure post-quantum encryption envelope
for Proofline evidence media and wrapped media-key metadata. The proposed suite is:

```text
ML-KEM-768 + HKDF-SHA384 + AES-256-GCM
```

The envelope is intended for long-lived encrypted evidence where
harvest-now/decrypt-later risk matters. It is pure post-quantum in the key
establishment layer: it does not depend on X25519, P-256, RSA, or another
classical public-key algorithm for confidentiality.

## Relationship To The Current v1 Envelope

The current simulator and backend reference flow use the documented v1
AES-256-GCM chunk envelope. That envelope encrypts chunks with a client-held
symmetric key and binds incident ID, stream ID, media type, and chunk index as
associated data. The backend stores opaque encrypted bytes and validates hashes
over ciphertext rather than decrypting media.

This future envelope keeps the same backend posture:

- clients encrypt before upload
- the backend stores ciphertext and non-secret metadata
- the backend may store wrapped content-encryption keys or media keys
- the backend must not store raw content-encryption keys, ML-KEM shared secrets,
  ML-KEM decapsulation keys, or plaintext in the default path
- bundle and viewer flows must remain explicit about whether key-wrapping
  metadata is present and who can use it

The new design is not a transparent in-place replacement for
`safety-recorder-chunk-encryption-v1`. It should be introduced as an explicit
new scheme with compatibility tests and migration notes.

## Goals

- Provide a post-quantum key establishment and wrapping design for long-lived
  incident media.
- Keep media encryption client-side and backend ciphertext-only by default.
- Support one or more authorised recipients per incident or stream.
- Support trusted-contact and future client-side viewer decryption without
  leaking raw media keys to the backend.
- Bind non-secret envelope metadata into AEAD authentication so metadata swaps
  are detected.
- Keep the format versioned and algorithm-agile without allowing downgrade
  behavior.
- Make implementation constraints explicit before adding runtime code.

## Non-Goals

- No server-side decryption.
- No server escrow or break-glass key access.
- No browser decryption implementation.
- No trusted-contact account implementation.
- No production mobile-client implementation.
- No hybrid classical/post-quantum mode in this document.
- No attempt to hide server-visible operational metadata such as incident ID,
  stream ID, timestamps, byte counts, ciphertext hashes, stream state, or route
  access logs.

## Standards And Primitive Selection

### ML-KEM-768

Use ML-KEM-768 as the default post-quantum key encapsulation mechanism. ML-KEM
is standardized by NIST FIPS 203. It is a KEM, not a bulk encryption algorithm:
a recipient has a decapsulation key and publishes an encapsulation key; the
sender encapsulates to the recipient's public key and receives a shared secret
plus a KEM ciphertext; the recipient decapsulates the KEM ciphertext to recover
the same shared secret.

ML-KEM-768 is selected as the default balance between security margin, key size,
ciphertext size, and implementation availability. ML-KEM-1024 may be added later
as a high-security profile, but this document keeps the first profile narrow.
ML-KEM-512 is not the default because Proofline evidence may be long-lived and
storage overhead is less important than conservative confidentiality.

Expected ML-KEM-768 sizes, using Go's `crypto/mlkem` names:

| Value | Size |
|---|---:|
| Encapsulation key | 1184 bytes |
| KEM ciphertext | 1088 bytes |
| Shared secret | 32 bytes |

### HKDF-SHA384

Use HKDF with SHA-384 as the extract-and-expand KDF. The ML-KEM shared secret is
input keying material, not a direct AES key. HKDF derives domain-separated keys
for CEK wrapping and future subkeys.

The implementation must use both extract and expand:

```text
prk = HKDF-Extract(salt, mlkem_shared_secret)
okm = HKDF-Expand(prk, info, length)
```

Use a per-envelope random salt. The salt is non-secret and is stored in the
recipient wrapping record. Use structured `info` strings that include the suite,
scheme version, envelope ID, recipient key ID, purpose, and any future context
needed for domain separation.

### AES-256-GCM

Use AES-256-GCM for payload encryption and for wrapping the content-encryption
key (CEK) unless a future standard key-wrap profile is deliberately selected.
AES-GCM provides authenticated encryption with associated data. Nonces must be
unique for each key. Implementations must generate 96-bit random nonces for each
AES-GCM operation unless a future deterministic nonce construction is separately
specified and reviewed.

Payload encryption uses a fresh random 256-bit CEK per envelope scope. For long
or streaming media, the CEK should be scoped to a stream or chunk group rather
than to all user evidence.

## Terminology

| Term | Meaning |
|---|---|
| Envelope | The serialized metadata and ciphertext container for encrypted media or key material. |
| CEK | Content-encryption key used with AES-256-GCM for payload media. |
| KEK | Key-encryption key derived from ML-KEM shared secret through HKDF-SHA384. |
| Encapsulation key | ML-KEM public key published by a recipient. |
| Decapsulation key | ML-KEM private key held by the recipient and never uploaded to the backend. |
| KEM ciphertext | ML-KEM output stored with the recipient wrapping record. |
| AAD | Additional authenticated data passed to AES-GCM. |
| Suite ID | Stable identifier for algorithm choices and serialization rules. |

## Proposed Suite Identifiers

Use explicit identifiers. Do not infer behavior from partial algorithm names.

```text
Scheme: proofline-pq-envelope-v1
Suite: proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1
KEM: ML-KEM-768
KDF: HKDF-SHA384
AEAD: AES-256-GCM
```

The implementation must reject unknown mandatory schemes, unknown suite IDs,
unknown KEM identifiers, unknown KDF identifiers, and unknown AEAD identifiers.
It must not silently fall back to older algorithms.

## Key Hierarchy

Recommended production hierarchy:

```text
recipient ML-KEM decapsulation key
└── ML-KEM shared secret per recipient wrapping record
    └── HKDF-SHA384 KEK per recipient wrapping record
        └── wrapped CEK
            └── AES-256-GCM payload encryption
```

A single payload ciphertext may be shared with multiple recipients by wrapping
the same CEK separately for each recipient. Each recipient record has its own
ML-KEM ciphertext, HKDF salt, KEK derivation context, AES-GCM wrapping nonce,
and wrapped CEK ciphertext.

For streaming media, prefer one CEK per stream or per bounded chunk group. Avoid
using one CEK for every incident forever. This limits blast radius if one key or
nonce domain is compromised.

## Envelope Structure

A future serialized envelope should contain a compact binary framing or a
canonical JSON/CBOR header followed by ciphertext. The exact serialization can be
chosen later, but the logical model should be stable.

```text
Envelope
├── magic/version
├── scheme
├── suite_id
├── envelope_id
├── created_at
├── payload_context
│   ├── incident_id
│   ├── stream_id
│   ├── media_type
│   ├── chunk_index or chunk_range
│   └── payload_type
├── payload_aead
│   ├── algorithm: AES-256-GCM
│   ├── nonce
│   ├── ciphertext
│   └── tag
└── recipient_wrapping_records[]
    ├── recipient_key_id
    ├── recipient_role
    ├── kem: ML-KEM-768
    ├── kem_ciphertext
    ├── kdf: HKDF-SHA384
    ├── hkdf_salt
    ├── hkdf_info_id or canonical info fields
    ├── cek_wrap_aead: AES-256-GCM
    ├── cek_wrap_nonce
    └── wrapped_cek_ciphertext_and_tag
```

The recipient wrapping records may be stored inside the envelope, stored as
separate authenticated metadata rows, or delivered through grant-scoped API
responses. The storage choice must not change the cryptographic boundary: the
backend may store wrapped CEKs but not raw CEKs or recipient private keys.

## Associated Data

Every AES-GCM operation must authenticate the relevant non-secret metadata.

Payload AEAD AAD must include a canonical encoding of:

- scheme
- suite ID
- envelope ID
- incident ID
- stream ID
- media type
- chunk index or chunk range
- payload type
- recipient wrapping manifest digest, if wrapping records are embedded or bound
  to the ciphertext

CEK-wrap AEAD AAD must include a canonical encoding of:

- scheme
- suite ID
- envelope ID
- recipient key ID
- recipient role
- KEM algorithm
- KDF algorithm
- wrapping purpose, for example `proofline-cek-wrap-v1`
- digest of the payload header or payload context

AAD must be derived from canonical structured fields, not from ad hoc string
concatenation in new formats. If JSON is used, use a strict canonical JSON
profile. If CBOR is used, use deterministic encoding. The current v1 string AAD
is acceptable only for the compatibility envelope it already defines.

## Derivation Rules

The ML-KEM shared secret must never be used directly as an AES key. Derive a KEK
as follows:

```text
salt = random(48 bytes)
info = canonical(
  "proofline-cek-wrap-v1",
  suite_id,
  envelope_id,
  recipient_key_id,
  kem_ciphertext_digest,
  payload_header_digest
)

kek = HKDF-SHA384(
  salt = salt,
  ikm = mlkem_shared_secret,
  info = info,
  length = 32 bytes
)
```

The `kem_ciphertext_digest` prevents accidental context confusion if the same
recipient has multiple wrapping records. The `payload_header_digest` binds the
KEK to this envelope's payload context.

Future subkeys must use different `info` purposes. Do not reuse the CEK-wrap KEK
for payload encryption, signing, token derivation, logging identifiers, or any
non-wrap purpose.

## Encryption Flow

For each payload envelope:

1. Generate a fresh 256-bit CEK using a cryptographically secure random source.
2. Generate a fresh 96-bit AES-GCM payload nonce.
3. Build the canonical payload header and payload AAD.
4. Encrypt the payload with AES-256-GCM using the CEK and payload AAD.
5. For each authorised recipient:
   1. Load and validate the recipient ML-KEM-768 encapsulation key.
   2. Encapsulate to obtain `mlkem_shared_secret` and `kem_ciphertext`.
   3. Generate a fresh HKDF salt.
   4. Derive a 256-bit KEK with HKDF-SHA384 and domain-separated info.
   5. Generate a fresh 96-bit CEK-wrap AES-GCM nonce.
   6. Encrypt the CEK with AES-256-GCM using the KEK and CEK-wrap AAD.
   7. Zero or allow the runtime to release temporary shared-secret and KEK bytes
      as soon as practical.
6. Serialize the envelope or store the payload and wrapping records according to
   the accepted storage design.

## Decryption Flow

For an authorised recipient:

1. Parse the envelope and reject unknown mandatory fields or algorithms.
2. Locate a recipient wrapping record matching the recipient key ID.
3. Validate ML-KEM ciphertext size and recipient key type.
4. Decapsulate the KEM ciphertext with the recipient ML-KEM decapsulation key.
5. Recompute HKDF salt and info from the wrapping record and canonical payload
   context.
6. Derive the KEK with HKDF-SHA384.
7. Decrypt the wrapped CEK with AES-256-GCM and CEK-wrap AAD.
8. Decrypt the payload with AES-256-GCM and payload AAD.
9. Treat all parse, decapsulation, unwrap, and payload authentication failures as
   decryption failure without exposing secret-dependent detail to untrusted
   callers.

## Error Handling

Implementation errors should distinguish enough detail for local development and
conformance tests, but public API and viewer errors must not leak useful oracle
information. External errors should collapse into categories such as:

- unsupported envelope version
- unsupported suite
- recipient key not found
- decryption failed
- malformed envelope

Do not expose whether a specific ML-KEM decapsulation failed, CEK unwrap failed,
or payload tag failed to unauthorised callers.

## Server Storage And API Boundary

The server may store:

- envelope scheme and suite identifiers
- recipient key IDs
- recipient role labels
- ML-KEM encapsulation public-key metadata
- ML-KEM KEM ciphertexts
- HKDF salts
- AES-GCM nonces
- wrapped CEK ciphertexts and tags
- payload ciphertext hashes and byte counts

The server must not store in the default path:

- raw CEKs
- ML-KEM decapsulation keys
- ML-KEM shared secrets
- derived KEKs
- plaintext media
- decrypted export caches
- browser-submitted private keys

Any future endpoint that accepts or returns wrapped-key metadata must stay behind
owner or grant-scoped authentication and must be reviewed against access-control,
logging, and bundle-manifest behavior.

## Serialization Requirements

Before implementation, choose and document a concrete wire format. It must define:

- magic bytes
- version number
- byte order for length fields
- maximum header size
- maximum recipient count
- maximum wrapping-record size
- canonical metadata encoding
- binary/base64 encoding rules
- strict duplicate-field rejection
- unknown-field behavior
- required versus optional fields
- conformance test vectors

Prefer binary fields for keys, nonces, salts, ciphertexts, and tags. If JSON is
used for early simulator work, base64url without padding should match the current
repository convention.

## Go Implementation Notes

The server repository currently targets Go and already uses standard-library
AES-GCM for the v1 compatibility envelope. A future implementation should prefer
standard-library packages where available:

- `crypto/mlkem` for ML-KEM-768
- `crypto/hkdf` or the accepted Go version's HKDF API for HKDF-SHA384
- `crypto/aes` and `crypto/cipher` for AES-256-GCM
- `crypto/rand` for CEKs, salts, and nonces
- `crypto/sha256` or `crypto/sha512` for non-secret header digests, depending on
  the final digest choice

Do not implement ML-KEM, HKDF, AES-GCM, random generation, canonical encoding, or
secret sharing manually.

Implementation should live in a new package or subpackage rather than mutating the
v1 compatibility envelope in place, for example:

```text
internal/envelope/pq
```

Suggested package boundaries:

```text
internal/envelope/pq
├── envelope.go        # logical types and suite constants
├── encode.go          # serialization and canonical AAD encoding
├── encrypt.go         # payload encrypt and recipient wrap flow
├── decrypt.go         # recipient unwrap and payload decrypt flow
├── keys.go            # key parsing and key ID helpers
└── envelope_test.go   # round trips, tampering, vectors, limits
```

## Test Requirements

Minimum tests before runtime use:

- ML-KEM-768 round-trip recipient wrapping.
- AES-GCM payload round-trip with authenticated metadata.
- CEK unwrap failure when recipient key ID, KEM ciphertext, HKDF salt, HKDF info,
  wrapping nonce, or wrapping AAD is changed.
- Payload decrypt failure when incident ID, stream ID, media type, chunk index,
  payload nonce, payload AAD, ciphertext, or tag is changed.
- Multiple-recipient envelope where each recipient can decrypt the same payload.
- Recipient isolation: one recipient cannot use another recipient's wrapping
  record.
- Unknown suite rejection.
- Unknown mandatory field rejection.
- Header size and recipient count limits.
- Malformed base64/binary field rejection.
- Golden test vectors for at least one single-recipient and one multi-recipient
  envelope.

Use deterministic test hooks only in tests. Production encapsulation, CEK
creation, salts, and nonces must come from secure randomness.

## Compatibility And Migration

The future PQ envelope must be additive:

- keep current v1 fixtures and simulator behavior working until an explicit
  migration is accepted
- advertise the new scheme through manifests or metadata only when the payload
  actually uses it
- reject mismatched scheme and envelope bytes
- keep evidence bundles explicit about which envelope scheme protects each chunk
- do not reinterpret old `SRCENC1` envelopes as PQ envelopes
- avoid automatic downgrade from PQ envelope to v1 envelope

A migration may support both schemes during a transition window, but decrypting
clients must decide from authenticated envelope metadata and magic bytes, not from
caller-supplied flags alone.

## Security Considerations

### Backend compromise

A passive backend, database, or blob-storage compromise should expose ciphertext,
non-secret envelope metadata, and wrapped CEKs, but not plaintext or raw CEKs.
Confidentiality then depends on the recipient ML-KEM decapsulation keys staying
private and the cryptographic implementation being correct.

### Active server or malicious viewer

This envelope does not by itself solve malicious-server browser decryption. If a
future browser viewer performs decryption, a compromised backend could still serve
malicious JavaScript that steals keys or plaintext. Browser decryption requires a
separate trust model, strict CSP, static asset controls, and possibly signed or
pinned viewer assets.

### Recipient compromise

A compromised recipient decapsulation key can unwrap any CEK that was wrapped to
that key. Revocation prevents future wrapping but cannot make already-distributed
or already-accessible wrapped records safe unless the affected payload is
re-encrypted under new CEKs.

### Nonce safety

AES-GCM nonce reuse under the same key is catastrophic. Payload CEKs and wrapping
KEKs must have separate nonce domains. Generate fresh random 96-bit nonces for
each AES-GCM operation and reject attempts to reuse caller-supplied nonces in
production APIs.

### Metadata privacy

The envelope authenticates metadata; it does not make all metadata secret.
Incident IDs, stream IDs, byte counts, timestamps, sharing state, and access logs
may remain visible to the backend. Sensitive GPS or dashboard metadata may need a
separate encrypted metadata design.

### Algorithm agility

Algorithm agility must not become downgrade agility. New suites may be added only
with explicit identifiers, compatibility tests, migration notes, and rejection of
unknown mandatory algorithms.

## Open Questions

- Should PQ envelopes be chunk-scoped, stream-scoped, or both?
- Should wrapping records be embedded in bundle manifests, delivered only through
  authenticated API responses, or both depending on grant type?
- Should payload AAD bind a digest of recipient wrapping records, or should
  wrapping records bind only to the payload header digest?
- Should high-security users be offered ML-KEM-1024 in a separate suite?
- What exact canonical encoding should be used: deterministic CBOR, canonical
  JSON, or a custom binary frame?
- What maximum recipient count is acceptable for emergency incident sharing?
- How should recipient public-key verification be represented in metadata?
- How should lost or rotated recipient keys affect old wrapping records?

## Proposed Implementation Phases

Phase 1: documentation and review.

- Add this design document.
- Link it from encryption and key-custody documentation.
- Keep existing runtime behavior unchanged.

Phase 2: test-only prototype.

- Add a new package for the PQ envelope.
- Implement in-memory round trips and tamper tests only.
- Do not expose new API routes or bundle behavior yet.

Phase 3: simulator prototype.

- Add opt-in simulator support for PQ envelopes.
- Generate local ML-KEM recipient keys for development only.
- Produce local test vectors and encrypted bundles.

Phase 4: API and storage design.

- Decide where wrapped CEK records live.
- Update access-control and bundle-manifest docs.
- Add schema/API changes only after access boundaries are accepted.

Phase 5: production client planning.

- Define mobile/client key storage.
- Define trusted-contact key enrollment, verification, rotation, and revocation.
- Define browser/native decryption trust boundaries before any user-facing decrypt
  flow ships.
