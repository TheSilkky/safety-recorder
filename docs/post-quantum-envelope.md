# Pure Post-Quantum Encryption Envelope

Status: v1 preview runtime default for server-side upload validation,
wrapped-key profile validation, bundle manifest profile hints, and the Go
simulator reference flow. The implementation keeps the backend ciphertext-only:
API routes validate public envelope metadata and accepted frame shapes but do
not decrypt evidence, store raw CEKs, store ML-KEM shared secrets, store
recipient decapsulation keys, or expose browser/backend decryption.

The first runtime implementation lives in `internal/envelope/pq`. API upload
routes validate payload frame headers from staged bytes before committing a
chunk. Wrapped-key creation validates the accepted public metadata and
wrapped-key frame transport before storing a record. Bundle manifests identify
the PQ profile while remaining key-free. The simulator defaults to PQ encrypted
uploads and retains the older v1 AES-GCM envelope only behind explicit
development compatibility flags.

This document defines the required v1 preview pure post-quantum encryption
envelope for Proofline evidence media and wrapped media-key metadata. The
accepted first production profile uses:

```text
ML-KEM-768 + HKDF-SHA384 + AES-256-GCM
```

Recipient lifecycle and trusted-contact enrollment are designed separately in
[contacts, key model, and viewer replacement](contacts-and-viewer-replacement.md).
That design treats account, device, and trusted-contact public-key records as
durable recipient keys. Incidents, streams, and bounded chunk groups own CEKs;
wrapped-key records connect those CEKs to recipient key versions. End users
should not manually manage ML-KEM keys, choose algorithms, or paste public keys
as the primary trusted-contact flow.

The envelope is intended for long-lived encrypted evidence where
harvest-now/decrypt-later risk matters. Proofline evidence may be extremely
sensitive, may involve victims, and may later be used in real legal settings.
The v1 preview default must therefore protect users from the earliest possible
date that real evidence may be uploaded. It is pure post-quantum in the key
establishment layer: it does not depend on X25519, P-256, RSA, or another
classical public-key algorithm for confidentiality.

## Relationship To The Compatibility v1 Envelope

The older documented v1 AES-256-GCM chunk envelope encrypts chunks with a
client-held symmetric key and binds incident ID, stream ID, media type, and
chunk index as associated data. It remains available only for explicit
simulator/test compatibility, for example with `--envelope v1`. It is not the
v1 preview runtime default.

This envelope keeps the same backend posture:

- clients encrypt before upload
- the backend stores ciphertext and non-secret metadata
- the backend may store wrapped content-encryption keys or media keys
- the backend must not store raw content-encryption keys, ML-KEM shared secrets,
  ML-KEM decapsulation keys, or plaintext in the default path
- bundle and viewer flows must remain explicit about whether key-wrapping
  metadata is present and who can use it

The post-quantum envelope is an explicit new scheme rather than a transparent
in-place replacement for `proofline-chunk-encryption-v1`. The older
`safety-recorder-chunk-encryption-v1` identifier is a pre-reset legacy value and
should remain only in explicit fail-closed tests or historical documentation.
Compatibility or simulator envelopes may remain for development, migration, or
test compatibility, but they must not be the real-user v1 preview default.

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
- Become the fully implemented, documented, and tested default envelope before
  v1 preview server and web-client use.

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
| Decapsulation seed form | 64 bytes |

### HKDF-SHA384

Use HKDF with SHA-384 as the extract-and-expand KDF. The ML-KEM shared secret is
input keying material, not a direct AES key. HKDF derives domain-separated keys
for CEK wrapping and future subkeys.

The implementation must use both extract and expand:

```text
prk = HKDF-Extract(salt, mlkem_shared_secret)
okm = HKDF-Expand(prk, info, length)
```

Use a fresh random HKDF salt per recipient wrapping record. The salt is
non-secret and is stored in that recipient wrapping record. Use structured `info`
bytes that include the suite, scheme version, envelope ID, recipient key ID,
purpose, SHA-384 digest identifiers, and any future context needed for domain
separation.

All non-secret digests in this envelope profile use SHA-384 unless a future suite
explicitly specifies otherwise.

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

## Accepted Production Profile

This is the accepted first production wrapping profile for v1 preview
runtime-default implementation. Runtime code must not land a different
post-quantum wrapping profile unless this document, compatibility tests, and
migration notes are updated first.

Use explicit identifiers. Do not infer behavior from partial algorithm names or
from caller-provided compatibility flags.

```text
Scheme: proofline-pq-envelope-v1
Suite: proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1
Wrapping algorithm: proofline-pq-mlkem768-hkdfsha384-aes256gcm
Wrapping algorithm version: 1
KEM: ML-KEM-768
KDF: HKDF-SHA384
AEAD: AES-256-GCM
Digest: SHA-384
Payload envelope magic: PLPQENC1
Wrapped-key ciphertext magic: PLPQWK1
Maximum recipients per CEK scope: 16
```

The implementation must reject unknown mandatory schemes, unknown suite IDs,
unknown KEM identifiers, unknown KDF identifiers, unknown AEAD identifiers, and
unknown digest identifiers. It must not silently fall back to older algorithms.

Server wrapped-key metadata creation validates these accepted values in the
existing fields:

```text
wrapping_algorithm = proofline-pq-mlkem768-hkdfsha384-aes256gcm
wrapping_algorithm_version = 1
```

The older `age-v1-x25519` value remains a simulator-development artifact format
only and is not accepted by the wrapped-key API runtime default.

## Key Encoding And Key IDs

The envelope needs stable key encodings before runtime use. For the initial Go
implementation profile:

- ML-KEM-768 encapsulation keys are encoded as the 1184-byte byte string returned
  by `crypto/mlkem.EncapsulationKey768.Bytes()`.
- ML-KEM-768 decapsulation keys are secret. When a test or local development
  format needs an exportable form, use the 64-byte seed returned by
  `crypto/mlkem.DecapsulationKey768.Bytes()` and imported with
  `crypto/mlkem.NewDecapsulationKey768(seed)`. Production clients may keep
  decapsulation keys non-exportable in platform key storage; the server format
  must not require decapsulation-key export.
- KEM ciphertexts are encoded as 1088-byte ML-KEM-768 ciphertexts.
- Shared secrets are 32 bytes and must never be serialized, logged, stored, or
  included in manifests.

If JSON is used for early simulator or test fixtures, binary fields must be
encoded with URL-safe base64 without padding, matching the current repository
convention. Decoders must reject non-canonical encodings, wrong lengths, and
unknown key types.

Recipient key IDs are non-secret identifiers derived only from the canonical
recipient public-key record, never from a decapsulation key or shared secret.
The accepted initial key ID format is:

```text
recipient_key_id = "pqk1_" || base64url_no_padding(SHA384(canonical_public_key_record))
```

The canonical public-key record used for this digest must include:

```text
scheme = proofline-pq-envelope-v1
kem = ML-KEM-768
digest = SHA-384
encoded_encapsulation_key = <1184-byte ML-KEM-768 public key>
key_version = <monotonic recipient-key version or creation identifier>
```

The canonical public-key record uses the field order shown above. Each field is
encoded as `uint16 field_name_length`, UTF-8 field name bytes,
`uint32 value_length`, and value bytes. String values are UTF-8. Binary values
are raw bytes, not base64, inside canonical profile bytes.

The key ID is a lookup and audit identifier. It does not grant access and must not
be treated as proof that a recipient controls the corresponding private key.
Public-key verification, contact enrollment, replacement, and revocation remain
part of the trusted-contact model, not the envelope primitive itself. Recipient
private keys belong to accounts, devices, and contacts; the envelope must not
model an incident as its own long-term private-key identity.

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

## Envelope And Wrapped-Key Record Shape

The accepted profile uses fixed-order binary encodings for cryptographic inputs
and binary ciphertext containers. JSON API fields are transport wrappers around
those profile values; implementations must not authenticate ad hoc JSON
stringification.

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
    ├── digest: SHA-384
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

Initial v1 preview server placement is:

- payload chunks use the new PQ payload envelope only after the runtime-default
  implementation lands
- server-stored wrapped-key records use authenticated owner or future
  grant-scoped `/v1` API responses
- current evidence bundle manifests remain key-free until a separate
  grant-scoped bundle-manifest design is accepted
- public token-only viewer responses must not deliver trusted-contact wrapped
  keys by default

The payload envelope container uses:

```text
magic = "PLPQENC1\n"
version = 1
header_length = uint32 big-endian
payload_header = canonical profile header bytes
payload_ciphertext_and_tag = AES-256-GCM output
```

The header length limit is 16 KiB. Header bytes are authenticated as payload
AEAD AAD and are also digested with SHA-384 for CEK wrapping records.

The `wrapped_key_ciphertext` API field is URL-safe base64 without padding over
this binary profile frame:

```text
magic = "PLPQWK1\n"
version = 1
kem_ciphertext_length = uint16 big-endian, value 1088
kem_ciphertext = 1088-byte ML-KEM-768 ciphertext
cek_wrap_nonce_length = uint8, value 12
cek_wrap_nonce = 96-bit AES-GCM nonce
wrapped_cek_length = uint16 big-endian
wrapped_cek_ciphertext_and_tag = AES-256-GCM output over the CEK
```

For the first profile the wrapped CEK plaintext is exactly 32 bytes, so the
AES-GCM wrapped CEK ciphertext and tag is expected to be 48 bytes. Decoders must
still parse the explicit length and reject unsupported sizes.

The `public_wrapping_metadata` API field is a JSON object used for routing,
validation, auditing, and re-creating the canonical AAD. For the first profile
it must contain only these top-level fields unless a future extension rule is
accepted:

```json
{
  "profile": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1",
  "scheme": "proofline-pq-envelope-v1",
  "suite_id": "proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1",
  "mandatory": [
    "profile",
    "scheme",
    "suite_id",
    "recipient_key_id",
    "recipient_key_version",
    "recipient_role",
    "media_key_id",
    "envelope_id",
    "payload_header_digest_b64u",
    "kem_ciphertext_digest_b64u",
    "hkdf_salt_b64u",
    "hkdf_info_id",
    "cek_wrap_aad_digest_b64u"
  ],
  "recipient_key_id": "pqk1_...",
  "recipient_key_version": 1,
  "recipient_role": "trusted_contact",
  "media_key_id": "media-key-...",
  "envelope_id": "env_...",
  "payload_header_digest_b64u": "base64url-no-padding-sha384",
  "kem_ciphertext_digest_b64u": "base64url-no-padding-sha384",
  "hkdf_salt_b64u": "base64url-no-padding-48-byte-salt",
  "hkdf_info_id": "proofline-cek-wrap-v1",
  "cek_wrap_aad_digest_b64u": "base64url-no-padding-sha384"
}
```

The metadata object must not contain raw CEKs, raw media keys, recipient private
keys, unwrapped shared secrets, derived KEKs, plaintext, browser fragment
secrets, server escrow material, request bodies, uploaded bytes, stored paths,
object keys, private deployment details, or user safety data.

## Associated Data

Every AES-GCM operation must authenticate the relevant non-secret metadata.

Payload AEAD AAD must include a canonical encoding of:

- scheme
- suite ID
- digest algorithm
- envelope ID
- incident ID
- stream ID
- media type
- chunk index or chunk range
- payload type

Payload AEAD AAD must not bind recipient wrapping records in the initial suite.
Recipient wrapping records may need to be added, removed, withheld, or delivered
through grant-scoped API responses after the payload has already been encrypted.
Binding recipient records into payload AAD would make late trusted-contact
enrollment and separate wrapped-key delivery harder. If a future immutable export
format needs to bind an embedded recipient manifest into payload AAD, it should
use a new suite or a separately versioned container rule.

CEK-wrap AEAD AAD must include a canonical encoding of:

- scheme
- suite ID
- digest algorithm
- wrapping algorithm
- wrapping algorithm version
- envelope ID
- recipient key ID
- recipient role
- `media_key_id`
- grant ID when the wrapped-key record is grant-bound
- KEM algorithm
- KDF algorithm
- wrapping purpose, for example `proofline-cek-wrap-v1`
- SHA-384 digest of the canonical payload header or payload context

AAD must be derived from canonical structured fields, not from ad hoc string
concatenation in new formats. The accepted PQ profile uses a fixed field order,
UTF-8 strings with explicit byte lengths, big-endian integer lengths, and raw
binary values for digests, nonces, salts, KEM ciphertexts, and ciphertext bytes.
For each canonical AAD structure, encode fields in the order listed above as
`uint16 field_name_length`, UTF-8 field name bytes, `uint32 value_length`, and
value bytes. String values are UTF-8. Integers use their shortest decimal ASCII
form unless a binary length field is explicitly specified. Binary values are raw
bytes, not base64, before hashing or AEAD use.
If a future JSON or CBOR transport is added, it must map into these canonical
profile bytes before hashing or AEAD use. The current v1 string AAD is
acceptable only for the compatibility envelope it already defines.

## Derivation Rules

The ML-KEM shared secret must never be used directly as an AES key. Derive a KEK
for each recipient wrapping record as follows:

```text
salt = random(48 bytes)  # per recipient wrapping record
payload_header_digest = SHA384(canonical_payload_header)
kem_ciphertext_digest = SHA384(kem_ciphertext)

info = canonical(
  "proofline-cek-wrap-v1",
  suite_id,
  envelope_id,
  recipient_key_id,
  digest = "SHA-384",
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
KEK to this envelope's payload context while allowing recipient wrapping records
to be delivered or rotated separately from the payload ciphertext.

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
   3. Generate a fresh 48-byte HKDF salt for this wrapping record.
   4. Compute SHA-384 digests for the KEM ciphertext and payload header.
   5. Derive a 256-bit KEK with HKDF-SHA384 and domain-separated info.
   6. Generate a fresh 96-bit CEK-wrap AES-GCM nonce.
   7. Encrypt the CEK with AES-256-GCM using the KEK and CEK-wrap AAD.
   8. Zero or allow the runtime to release temporary shared-secret and KEK bytes
      as soon as practical.
6. Serialize the envelope or store the payload and wrapping records according to
   the accepted storage design.

## Decryption Flow

For an authorised recipient:

1. Parse the envelope and reject unknown mandatory fields or algorithms.
2. Locate a recipient wrapping record matching the recipient key ID.
3. Validate ML-KEM ciphertext size and recipient key type.
4. Decapsulate the KEM ciphertext with the recipient ML-KEM decapsulation key.
5. Recompute SHA-384 digests, HKDF salt, and HKDF info from the wrapping record
   and canonical payload context.
6. Derive the KEK with HKDF-SHA384.
7. Decrypt the wrapped CEK with AES-256-GCM and CEK-wrap AAD.
8. Decrypt the payload with AES-256-GCM and payload AAD.
9. Treat all parse, decapsulation, unwrap, and payload authentication failures as
   decryption failure without exposing secret-dependent detail to untrusted
   callers.

## Error Handling

Implementation errors should distinguish enough detail for local development and
conformance tests, but public API and viewer errors must not leak useful oracle
information. External untrusted errors should collapse into categories such as:

- unsupported envelope version
- unsupported suite
- malformed envelope
- decryption failed

`recipient key not found`, ML-KEM decapsulation failure, CEK unwrap failure, and
payload tag failure may be useful internal test or authenticated owner-tooling
errors, but they must not be exposed to unauthorised callers as distinct public
viewer or API responses.

## Validation, Limits, And Fail-Closed Rules

Runtime-default implementation must fail closed for this profile:

- reject unknown schemes, suites, KEMs, KDFs, AEADs, digests, wrapping
  algorithms, and wrapping algorithm versions
- reject unknown top-level public metadata fields unless a later extension rule
  explicitly allows them
- reject any field listed in `mandatory` that the implementation does not
  understand
- reject missing required fields, duplicate fields, malformed JSON metadata,
  malformed binary frames, non-canonical base64url, wrong fixed lengths, empty
  salts or nonces, and oversized headers or records
- reject more than 16 recipient wrapping records for one CEK scope
- reject public wrapping metadata larger than 4096 bytes and wrapped-key
  ciphertext transport strings larger than 16384 bytes unless a schema and API
  migration deliberately changes those limits
- reject payload envelopes whose magic, scheme, suite, or authenticated header
  does not match the advertised metadata
- reject `PLCHNK1` compatibility envelopes, legacy `SRCENC1` envelopes,
  `age-v1-x25519` simulator artifacts, or any classical-only wrapping profile
  as a v1 preview default
- reject automatic downgrade from the PQ suite to the compatibility v1 envelope;
  migration windows must be explicit and authenticated by envelope metadata
- treat unsupported algorithms, malformed metadata, missing recipient keys,
  lost contact private keys, contact-key revocation, CEK unwrap failure, and
  payload authentication failure as no-decrypt outcomes

If a trusted contact loses the matching private key, the backend must not
decrypt, rewrap, or synthesize a replacement wrapped key. Future clients may
wrap future CEKs to a replacement key and may rewrap older CEKs only if an
authorized client or separately approved escrow mode still has access to the raw
CEK. The default server path remains ciphertext-only.

## Server Storage And API Boundary

The server may store:

- envelope scheme and suite identifiers
- recipient key IDs
- recipient role labels
- ML-KEM encapsulation public-key metadata
- SHA-384 public-key record digests
- ML-KEM KEM ciphertexts
- SHA-384 KEM ciphertext digests
- SHA-384 payload header digests
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

Before implementation, code and tests must enforce the concrete profile
constraints defined above:

- magic bytes: `PLPQENC1\n` for payload envelopes and `PLPQWK1\n` for wrapped
  CEK ciphertext frames
- version number: `1`
- byte order for length fields: big-endian
- maximum header size: 16 KiB
- maximum recipient count: 16 wrapping records per CEK scope
- maximum public wrapping metadata size: 4096 bytes
- maximum wrapped-key ciphertext transport string size: 16384 bytes
- canonical metadata encoding: fixed-order profile fields with explicit byte
  lengths, not ad hoc JSON string concatenation
- binary/base64 encoding: URL-safe base64 without padding at JSON/API
  boundaries, raw bytes inside canonical profile frames
- strict duplicate-field rejection for any JSON transport metadata
- unknown-field behavior: reject unknown top-level fields in v1 metadata, and
  reject unknown fields named in `mandatory`
- required versus optional fields: no optional top-level fields in the first
  production profile except future extension containers accepted by a new suite
- SHA-384 digest encoding: raw 48-byte digest in canonical bytes, base64url
  without padding in JSON/API fields
- conformance test vectors: single-recipient, multi-recipient, and negative
  tamper cases before runtime use

Prefer binary fields for keys, nonces, salts, ciphertexts, tags, and digests
inside the profile. If JSON is used for fixtures or API transport, base64url
without padding must match the current repository convention.

## Implementation Compatibility Requirements

### Go

The server repository currently targets Go 1.26.4 and already uses
standard-library AES-GCM for the v1 compatibility envelope. Runtime
implementation should prefer:

- `crypto/mlkem` for ML-KEM-768
- `crypto/hkdf` for HKDF-SHA384
- `crypto/aes` and `crypto/cipher` for AES-256-GCM
- `crypto/rand` for CEKs, salts, and nonces
- `crypto/sha512` with `sha512.New384` for SHA-384 digests and HKDF hash input

Do not implement ML-KEM, HKDF, AES-GCM, random generation, or secret sharing
manually. Implement the profile encoder and parser as ordinary, heavily tested
serialization code that follows this document exactly; do not create alternate
undocumented encodings.

Use `crypto/mlkem/mlkemtest` or equivalent deterministic hooks only for tests.
Production encapsulation, CEK creation, salts, and nonces must use secure
randomness.

### Browser And Web Client

Browser support must not assume that Web Crypto provides ML-KEM. A web-client
implementation must either use a maintained, reviewed ML-KEM implementation or a
standard browser/platform API once one is available and reviewed. It may use Web
Crypto for AES-GCM and HKDF only when key extractability, import/export,
encoding, worker, memory, and failure behavior match this profile.

Browser decrypt code remains out of this server issue. Before any preview claim,
the web-client trust model must still address same-origin malicious JavaScript,
static asset integrity, CSP, plaintext lifetime, service-worker/cache behavior,
debug output, analytics, extension risk, and user guidance.

### Native Mobile And Trusted-Contact Clients

Native clients should use platform cryptography or maintained libraries for
ML-KEM, HKDF-SHA384, AES-256-GCM, SHA-384, and secure randomness. The profile
must not require exporting long-term decapsulation keys from platform key
storage. A client may publish the 1184-byte encapsulation key and retain a
non-exportable private key when the platform supports that model.

Trusted-contact clients must treat the recipient private key, ML-KEM shared
secret, derived KEK, unwrapped CEK, and plaintext as local secrets. They must
not place those values in API requests, logs, crash reports, URLs, local
plaintext caches, or public issue/test artifacts.

## Go Package Boundary

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
├── keys.go            # key encoding and key ID helpers
└── envelope_test.go   # round trips, tampering, vectors, limits
```

## Conformance And Test Vector Requirements

Minimum tests before runtime use:

- ML-KEM-768 round-trip recipient wrapping.
- AES-GCM payload round-trip with authenticated metadata.
- ML-KEM-768 public-key encoding and decapsulation-seed import/export tests for
  development fixtures.
- Recipient key ID derivation from canonical public-key records.
- SHA-384 digest fixtures for canonical public-key records, KEM ciphertexts, and
  payload headers.
- CEK unwrap failure when recipient key ID, KEM ciphertext, HKDF salt, HKDF info,
  wrapping nonce, wrapping AAD, or SHA-384 digest field is changed.
- Payload decrypt failure when incident ID, stream ID, media type, chunk index,
  payload nonce, payload AAD, ciphertext, or tag is changed.
- Multiple-recipient envelope where each recipient can decrypt the same payload.
- Recipient isolation: one recipient cannot use another recipient's wrapping
  record.
- Late-recipient metadata behavior: adding a wrapping record must not require
  payload ciphertext changes in the initial suite.
- Unknown suite rejection.
- Unknown mandatory field rejection.
- Header size and recipient count limits.
- Malformed base64/binary field rejection.
- Golden test vectors for at least one single-recipient and one multi-recipient
  envelope.
- Negative vectors for wrong magic, unknown suite, unknown mandatory field,
  malformed public metadata, downgrade attempt, missing recipient key, wrong
  recipient key, lost/revoked contact key state, wrong `media_key_id`, and
  modified `wrapped_key_ciphertext`.
- Fixture checks proving raw production keys, raw production CEKs, plaintext,
  ML-KEM shared secrets, derived KEKs, recipient private keys, request bodies,
  uploaded bytes, stored paths, object keys, and private deployment details are
  absent from committed vectors, logs, API examples, bundle manifests, and public
  documentation.

Use deterministic test hooks only in tests. Production encapsulation, CEK
creation, salts, and nonces must come from secure randomness.

Conformance vectors may include deterministic test-only decapsulation seeds,
test CEKs, and expected plaintext only when the fixture path, metadata, and
comments mark them as non-production conformance material. They must not be
reused as simulator defaults, sample user keys, deployment secrets, or runtime
fallback keys.

Existing simulator `age-v1-x25519` artifacts are prototype vectors for the local
development companion flow. They are not conformance vectors for this production
PQ profile. Future PQ simulator vectors must use the accepted identifiers and
must remain clearly separate from later runtime-default implementation fixtures.

## Compatibility And Migration

The future PQ envelope must be additive:

- keep current v1 fixtures and simulator behavior working until an explicit
  migration is accepted
- migrate or regenerate prototype/test fixtures that assumed per-incident
  private-key identities to durable recipient public-key records plus scoped
  CEKs when those fixtures are touched
- advertise the new scheme through manifests or metadata only when the payload
  actually uses it
- reject mismatched scheme and envelope bytes
- keep evidence bundles explicit about which envelope scheme protects each chunk
- do not reinterpret `PLCHNK1` compatibility envelopes or old `SRCENC1`
  envelopes as PQ envelopes
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

## References And Tracking

Implementation work must verify current references before code is merged:

- NIST FIPS 203, Module-Lattice-Based Key-Encapsulation Mechanism Standard:
  <https://csrc.nist.gov/pubs/fips/203/final>
- RFC 5869, HKDF extract-and-expand construction:
  <https://www.rfc-editor.org/rfc/rfc5869>
- Go `crypto/mlkem` package documentation:
  <https://pkg.go.dev/crypto/mlkem>
- Go `crypto/hkdf` package documentation:
  <https://pkg.go.dev/crypto/hkdf>

As of 2026-06-10, the NIST FIPS 203 page includes a 2025-11-17 planning note
for future errata. Before implementation, check the current FIPS 203 page, the
errata spreadsheet, and Go release notes for revisions, package API changes,
size constants, and test-vector guidance.

## Open Questions

- Should a future suite add stream-scoped or bounded chunk-group CEKs beyond the
  current chunk-scoped runtime payload frames?
- Should a separate immutable export format bind an embedded wrapping-record
  manifest into payload AAD, or should all wrapping records remain bound only to
  the payload header digest?
- Should high-security users be offered ML-KEM-1024 in a separate suite?
- How should recipient public-key verification be represented in metadata?
- How should lost or rotated recipient keys affect product UX and old wrapping
  record retention after the fail-closed cryptographic behavior is enforced?

## Proposed Implementation Phases

Phase 1: accepted production profile. Complete.

- Define the accepted production wrapping profile in this document.
- Link it from encryption, key-custody, contact-sharing, and simulator
  documentation.

Phase 2: implementation package. Complete for the accepted v1 preview runtime
default.

- Add a package for the PQ envelope.
- Implement in-memory round trips, canonical encoding, limits, local vectors,
  tamper tests, public payload-header validation, and public wrapped-key
  metadata validation.

Phase 3: simulator reference flow. Complete for default encrypted uploads and
same-run bundle verification.

- Make simulator uploads use PQ envelopes by default.
- Generate local ML-KEM recipient keys for development only.
- Keep the v1 AES-GCM envelope behind explicit compatibility flags.

Phase 4: API and storage integration. Complete for upload validation,
wrapped-key profile validation, and bundle manifest hints.

- Map PQ wrapped-key records onto the existing authenticated wrapped-key
  metadata create route.
- Keep bundle manifests key-free unless a separate grant-scoped manifest design
  is accepted.
- Avoid schema changes because the accepted profile fits the existing wrapped-key
  fields and limits.

Remaining future work:

- Production client key storage and trusted-contact UX.
- Browser/client-side decryption in separate client repositories.
- Cross-repository protocol conformance tests.
- Any future grant-scoped wrapping-record manifest design.

Phase 5: production client planning.

- Define mobile/client key storage.
- Define trusted-contact key enrollment, verification, rotation, and revocation.
- Define browser/native decryption trust boundaries before any user-facing decrypt
  flow ships.
