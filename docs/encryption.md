# Encryption

Proofline currently stores opaque encrypted chunk bytes. This document describes the first client-side chunk encryption envelope used by the Go simulator and tests.

This milestone does not add backend decryption. The server still validates SHA-256 over uploaded ciphertext bytes, stores those bytes in the configured blob backend, and emits encrypted ZIP evidence bundles.

## Naming

The current compatibility envelope uses Proofline-named identifiers. Earlier
experimental `safety-recorder` envelope identifiers are not accepted by the
current parser except in explicit negative tests that prove fail-closed
behavior.

## Threat Model

The v1 envelope protects chunk plaintext from the backend, SQLite, configured blob storage, and evidence bundle readers who do not have the client-held key. It does not protect metadata that is already sent to the backend, such as incident ID, stream ID, media type, chunk index, timestamps, byte size, and ciphertext hashes.

The simulator key handling in this repository is for development and test use
only. In future-design terminology, the current simulator key is a development
CEK rather than a long-term private-key identity for an incident. Future
production client key storage, sharing, recovery, trusted-contact access,
account-owner access, and incident-mode sharing are out of scope for the current
implementation and are designed separately in [key-custody.md](key-custody.md),
[incident-modes.md](incident-modes.md), and
[v1-access-control.md](v1-access-control.md).

## Scheme v1

| Field | Value |
|---|---|
| Scheme | `proofline-chunk-encryption-v1` |
| Algorithm | `AES-256-GCM` |
| Key size | 32 bytes |
| Nonce size | 12 bytes |
| Tag | Included in the AES-GCM ciphertext |
| Associated data | Deterministic UTF-8 metadata string |

The Go implementation uses the standard library packages `crypto/aes`, `crypto/cipher`, `crypto/rand`, `encoding/base64`, `encoding/binary`, and `encoding/json`.

Generate a fresh random nonce for every encrypted chunk. Never reuse a nonce with the same key. A single key must not be used for more than `2^32` chunks/messages. By default, the simulator generates a fresh ephemeral key per run unless `--key-file` is supplied.

## Associated Data

The AEAD associated data is an exact UTF-8 string:

```text
ProoflineChunk:v1
incident_id=<incident_id>
stream_id=<stream_id>
media_type=<media_type>
chunk_index=<chunk_index>
```

There is a trailing newline after the `chunk_index` line. Example:

```text
ProoflineChunk:v1
incident_id=inc_abc
stream_id=str_def
media_type=audio
chunk_index=1
```

Encryption and decryption must use identical associated data. IDs and media type must not contain newlines, and `chunk_index` must be positive. This matches streamed upload semantics; legacy unstreamed `chunk_index = 0` chunks cannot use this v1 associated data. Decryption fails when incident ID, stream ID, media type, or chunk index differs from the original metadata.

The v1 associated data does not include capture stream group IDs, variant roles,
source timeline identity, supersession state, or encrypted GPS/context
bindings. Those fields are future planning work in
[capture-stream-variants.md](capture-stream-variants.md) and require explicit
protocol and envelope review before they are implemented.

## Chunk Envelope

Each uploaded `.enc` file contains:

```text
magic bytes
uint32 big-endian header length
UTF-8 JSON header
AES-GCM ciphertext including authentication tag
```

Magic bytes are exactly:

```text
PLCHNK1
```

The JSON header is non-secret:

```json
{
  "version": 1,
  "scheme": "proofline-chunk-encryption-v1",
  "algorithm": "AES-256-GCM",
  "key_id": "kid_...",
  "nonce_b64": "base64url-no-padding-12-byte-nonce",
  "aad": "ProoflineChunk:v1\nincident_id=inc_...\nstream_id=str_...\nmedia_type=audio\nchunk_index=1\n"
}
```

The implementation rejects malformed magic, truncated envelopes, oversized headers, unknown versions, unknown algorithms, missing fields, wrong nonce lengths, and associated-data mismatches. Header length is capped at 16 KiB. The header must not contain plaintext, secret keys, or server filesystem paths.

Nonce and key values use URL-safe base64 without padding.

## Simulator Key File

The simulator can load or create a local development key file:

```json
{
  "version": 1,
  "scheme": "proofline-chunk-encryption-v1",
  "algorithm": "AES-256-GCM",
  "key_id": "kid_...",
  "key_b64": "base64url-no-padding-32-byte-key"
}
```

`key_id` is non-secret. `key_b64` is secret. Do not upload this file, add it to evidence bundles, commit it to git, or paste it into logs. The simulator writes key files with `0600` permissions where practical.

## Simulator Usage

Encryption is enabled by default:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

Expected output includes the non-secret key ID, encrypted chunk uploads, bundle
download, and local decrypt verification. The simulator does not print raw
keys, plaintext, key-file paths, or token-bearing viewer URLs.

To persist a simulator key locally:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle --key-file /tmp/proofline-sim.key.json
```

Run it again with the same path to load the existing key:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 2 --interval 1s --download-bundle --key-file /tmp/proofline-sim.key.json
```

Older local examples may have used `/tmp/safety-recorder-sim.key.json`; that
name is historical and is not part of the current protocol or default
simulator artifact layout.

To preserve the old raw fake chunk behavior for development compatibility:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --encrypt=false
```

Bundle decrypt verification defaults on when `--download-bundle` and `--encrypt` are both enabled. It can be disabled with:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --download-bundle --verify-bundle-decryption=false
```

## What The Backend Sees

The backend sees opaque uploaded bytes and client-provided metadata. It stores
ciphertext and validates SHA-256 over the ciphertext envelope. Private
owner-authenticated routes can store grant-bound wrapped-key records as
encrypted metadata, but the backend does not parse raw CEKs or media keys,
store raw keys in SQLite, upload raw keys, decrypt chunks, or expose public
decryption endpoints.

Evidence bundles remain ZIP files containing encrypted `.enc` chunk files and JSON manifests. Bundle manifests include a non-secret hint that client-side encryption is expected and that the server does not decrypt.

## Incident Modes And Encryption

Future incident modes do not change the backend ciphertext-only posture by themselves. Emergency incidents, interaction records, safety checks, and evidence notes may have different capture, sharing, or escalation policies, but uploaded media should still be encrypted before upload and treated as opaque ciphertext by the backend unless an explicit future decryption/key-custody design says otherwise.

## Future Work

The intended Apple-side equivalent is CryptoKit or Swift Crypto AES-GCM. This repository does not include iOS or Swift code yet.

Future work includes production client key storage, Keychain integration, trusted-contact key access, key sharing, browser/client-side decryption, account-based access, incident-mode sharing, and playable export. The intended production key custody direction is a hybrid trusted-contact model documented in [key-custody.md](key-custody.md), with future access boundaries in [v1-access-control.md](v1-access-control.md), browser decryption constraints in [browser-decryption.md](browser-decryption.md), and optional break-glass design in [break-glass-key-access.md](break-glass-key-access.md). Password-derived keys, passphrases, production public-key wrapping, key escrow, backend decryption, and browser decryption are not implemented in this milestone.

Future capture stream variant and supersession work may need new encrypted
context bindings or source-timeline metadata. That must not be inferred from
the current envelope; it is a separate design tracked in
[capture-stream-variants.md](capture-stream-variants.md).

The required v1 preview pure post-quantum envelope profile is designed
separately in [post-quantum-envelope.md](post-quantum-envelope.md). That design
accepts `proofline-pq-mlkem768-hkdfsha384-aes256gcm-v1`, using
`ML-KEM-768 + HKDF-SHA384 + AES-256-GCM` for recipient-key wrapping of CEKs and
payload encryption. It defines the future wrapped-key API field values,
metadata shape, canonical encoding, downgrade rejection, fail-closed behavior,
and conformance-vector requirements. The isolated `internal/envelope/pq`
package now exercises that profile in local tests and vectors only. It does not
change current runtime behavior, viewer behavior, storage behavior, bundle
behavior, key custody, simulator defaults, or server defaults. The profile must
still be fully implemented, documented, tested, and made default before server
or web-client v1 preview use with real sensitive evidence.

The simulator-only contact-wrapped key metadata prototype is implemented
separately in
[contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md).
That prototype can model contact public keys, non-secret key IDs, and wrapped
stream CEKs in local development artifacts, but it does not change the current
v1 envelope, make the backend store raw keys, or make the backend decrypt
media. Server-side wrapped-key records remain encrypted metadata behind
authenticated owner routes.
