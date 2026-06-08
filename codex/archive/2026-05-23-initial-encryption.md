# Codex Prompt: Add Client-Side Encryption Envelope and Simulator Encryption

This prompt is historical/reference-only. Do not re-run it without checking it
against the current `README.md`, `AGENTS.md`, `SECURITY.md`, docs, and reusable
prompts.

Add a documented client-side encryption envelope and implement encryption/decryption support in the Go simulator.

This is a crypto-sensitive task.

Do **not** implement cryptographic primitives from scratch.
Do **not** invent a custom cipher, MAC, AEAD mode, padding scheme, KDF, random number generator, or password-based encryption scheme.
Do **not** add backend decryption.
Do **not** store encryption keys in SQLite.
Do **not** upload encryption keys to the backend.
Do **not** add browser decryption, iOS code, Swift code, React, Node, npm, OAuth, JWT, user accounts, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features.

## Goal

The backend already treats uploaded chunks as encrypted blobs.

Add the next milestone:

1. document the encryption envelope format in `docs/encryption.md`
2. implement a stable Go encryption/decryption helper for simulator/test use
3. update `cmd/simclient` so simulated uploaded chunks are encrypted by default
4. allow downloaded evidence bundles to be decrypted/verified locally by the simulator
5. keep the backend ignorant of plaintext and keys

The backend should continue storing opaque encrypted bytes and verified ciphertext hashes.

## Source of truth

Use the current repository files as source of truth, especially:

- `README.md`
- `CHANGELOG.md`
- `AGENTS.md`
- `SECURITY.md`
- `docs/api.md`
- `docs/security-model.md` or `docs/threat-model.md`
- `docs/code-map.md`
- `server/cmd/simclient`

Preserve current project facts:

- Go backend only
- private `/v1` write/admin API listener group
- public read-only emergency viewer listener group
- SQLite metadata
- local disk encrypted chunk storage
- immutable chunk uploads
- media streams that can be marked `open`, `complete`, or `failed`
- completed encrypted stream and incident ZIP evidence bundle downloads
- emergency viewer tokens
- simulator CLI
- Docker image build
- GitHub Actions / GHCR publishing
- AGPL-3.0-only license
- repository security policy

Evidence bundles are ZIP files containing encrypted chunks plus JSON manifests.

Evidence bundles are not decrypted, playable, or merged media exports.

## Crypto library requirements

Use stable, documented crypto libraries only.

For Go simulator/test implementation:

- use Go standard library `crypto/aes`
- use Go standard library `crypto/cipher`
- use Go standard library `crypto/rand`
- use Go standard library `encoding/base64`
- use Go standard library `encoding/binary`
- use Go standard library `encoding/json`
- use Go standard library `crypto/sha256` only for non-secret identifiers or verification where appropriate

Use AES-256-GCM via `cipher.NewGCM`.

Do not use CBC, CFB, OFB, ECB, custom CTR, custom MACs, or unauthenticated encryption.

For future iOS implementation, document that the intended Apple-side equivalent is CryptoKit / Swift Crypto AES-GCM, but do not implement iOS code in this task.

## Encryption scheme v1

Implement and document:

```text
scheme: safety-recorder-chunk-encryption-v1
algorithm: AES-256-GCM
key size: 32 bytes
nonce size: 12 bytes, generated randomly per chunk using crypto/rand
tag: provided by AES-GCM
associated data: deterministic UTF-8 string built from incident/stream/chunk metadata
```

Nonce requirement:

- generate a fresh random nonce for every encrypted chunk
- never reuse a nonce with the same key
- document that a key must not be used for more than 2^32 chunks/messages
- default simulator behaviour should generate a fresh key per simulated incident unless a key file is explicitly provided

## Key model for this task

This is simulator/development key handling only.

Add a simple simulator key file format:

```json
{
  "version": 1,
  "scheme": "safety-recorder-chunk-encryption-v1",
  "algorithm": "AES-256-GCM",
  "key_id": "kid_...",
  "key_b64": "base64url-no-padding-32-byte-key"
}
```

Rules:

- `key_id` is non-secret and randomly generated.
- `key_b64` is secret.
- Do not upload this file.
- Do not add it to evidence bundles.
- Do not log `key_b64`.
- If a key file is written, create it with restrictive permissions where practical, e.g. `0600`.
- Add `.gitignore` entries for local simulator key files if needed.

Future key sharing is out of scope.

Do not implement:

- password-derived keys
- passphrases
- public-key wrapping
- emergency contact key sharing
- browser decryption
- server-side decryption
- key escrow

## Associated data

Create a helper that builds exact AEAD associated data bytes.

Use a deterministic UTF-8 string with this exact shape:

```text
SafetyRecorderChunk:v1
incident_id=<incident_id>
stream_id=<stream_id>
media_type=<media_type>
chunk_index=<chunk_index>
```

Include a trailing newline after the `chunk_index` line.

Example:

```text
SafetyRecorderChunk:v1
incident_id=inc_abc
stream_id=str_def
media_type=audio
chunk_index=1
```

Rules:

- associated data must be identical for encryption and decryption
- validate that IDs and media type do not contain newlines
- validate chunk index is positive
- decryption must fail if associated data differs
- document this exact format in `docs/encryption.md`

## Chunk envelope format

Implement a binary envelope that wraps each encrypted chunk.

The uploaded `.enc` file should contain:

```text
magic bytes
uint32 big-endian header length
UTF-8 JSON header
AES-GCM ciphertext including authentication tag
```

Use this exact magic:

```text
SRCENC1\n
```

That is 8 bytes.

Header JSON:

```json
{
  "version": 1,
  "scheme": "safety-recorder-chunk-encryption-v1",
  "algorithm": "AES-256-GCM",
  "key_id": "kid_...",
  "nonce_b64": "base64url-no-padding-12-byte-nonce",
  "aad": "SafetyRecorderChunk:v1\nincident_id=inc_...\nstream_id=str_...\nmedia_type=audio\nchunk_index=1\n"
}
```

Header rules:

- JSON must be UTF-8
- use URL-safe base64 without padding for nonce/key values
- set a reasonable maximum header length, for example 16 KiB
- reject malformed headers
- reject unknown versions
- reject unknown algorithms
- reject wrong nonce length
- reject missing fields
- reject AAD mismatch if caller supplies expected metadata
- do not put plaintext in the header
- do not put secret keys in the header
- do not put server filesystem paths in the header

Ciphertext rules:

- ciphertext is the output of `AEAD.Seal`
- ciphertext includes the GCM authentication tag
- no separate MAC should be added

## Suggested package

Create a small package for the envelope implementation.

Suggested location:

```text
server/internal/envelope/
```

Suggested public functions inside the package:

```go
type ChunkContext struct {
    IncidentID string
    StreamID   string
    MediaType  string
    ChunkIndex int
}

type Key struct {
    Version   int
    Scheme    string
    Algorithm string
    KeyID     string
    Key       []byte
}

func GenerateKey() (Key, error)
func LoadKeyFile(path string) (Key, error)
func SaveKeyFile(path string, key Key) error

func BuildAssociatedData(ctx ChunkContext) ([]byte, error)

func EncryptChunk(key Key, ctx ChunkContext, plaintext []byte) ([]byte, error)
func DecryptChunk(key Key, ctx ChunkContext, envelopeBytes []byte) ([]byte, error)

func ParseHeader(envelopeBytes []byte) (Header, error)
```

These names are suggestions, not sacred scripture. Keep the API small and clear.

The backend handlers should not call decrypt functions.

## Simulator changes

Update `cmd/simclient` so it encrypts fake chunk plaintext before upload.

Add flags:

```text
--encrypt
```

Default: `true`

```text
--key-file
```

Optional path.

Behaviour:

- if `--encrypt=true` and `--key-file` is not set:
  - generate an ephemeral key for this simulator run
  - print the non-secret `key_id`
  - do not print the raw key
- if `--encrypt=true` and `--key-file` exists:
  - load it
- if `--encrypt=true` and `--key-file` is set but does not exist:
  - generate a new key
  - save it to that path with restrictive permissions where practical
- if `--encrypt=false`:
  - preserve old behaviour and upload raw fake chunk bytes
  - print a warning that this is only for development compatibility

Add:

```text
--verify-bundle-decryption
```

Default: `true` when both `--download-bundle` and `--encrypt` are true.

When `--download-bundle` is enabled:

- download the completed stream bundle
- locate encrypted chunk files in the ZIP
- decrypt each chunk using the same key and expected metadata
- report success/failure
- do not print plaintext bytes

Simulator output should be clear:

```text
Encryption: enabled
Key ID: kid_...
Uploading encrypted audio chunk 1/5...
Downloaded bundle.
Verified decrypt of 5 encrypted chunks.
```

If using `--encrypt=false`:

```text
Encryption: disabled. Uploading raw fake chunk bytes for development compatibility only.
```

## Backend behaviour

Do not add backend decryption.

The backend may receive ciphertext with the envelope format, but it should continue treating uploaded files as opaque bytes.

The backend still validates SHA-256 of the uploaded ciphertext bytes.

The backend should not parse encryption headers unless there is a compelling reason. If you do parse headers for validation, do not store secrets and do not reject valid old ciphertext-free legacy chunks unless explicitly requested.

Prefer keeping backend storage unchanged.

## Evidence bundle behaviour

Evidence bundles should continue containing `.enc` chunk files and JSON manifests.

Do not decrypt bundles on the server.

Do not turn bundles into playable media.

If practical, update bundle manifests to include a non-secret indication that chunks appear to use encryption scheme v1, but do not require the backend to inspect or validate encryption envelopes.

Acceptable manifest hint:

```json
{
  "encryption": {
    "expected": "client-side",
    "scheme": "safety-recorder-chunk-encryption-v1",
    "server_decrypts": false
  }
}
```

Do not include keys.

## Documentation

Add:

```text
docs/encryption.md
```

Document:

- threat model for this milestone
- scheme version
- algorithm
- key size
- nonce size
- nonce uniqueness warning
- associated data format
- envelope binary format
- key file format for simulator only
- what the backend sees
- what is out of scope
- how to run simulator with encryption
- how to run simulator with a key file
- how to verify downloaded bundle decryption
- future iOS implementation notes using CryptoKit / Swift Crypto
- future work: key sharing, Keychain storage, emergency contact access, browser/client-side decryption, playable export

Update:

- `README.md`, briefly
- `docs/README.md`, if present
- `docs/api.md`, only if needed
- `docs/security-model.md` or `docs/threat-model.md`
- `docs/simulator.md`, if present
- `CHANGELOG.md`

Do not make the README huge.

## Tests

Add tests for the envelope package:

- generate key produces 32-byte key
- key file save/load roundtrip
- encrypt/decrypt roundtrip
- wrong key fails
- wrong associated data fails
- changed incident ID fails
- changed stream ID fails
- changed media type fails
- changed chunk index fails
- malformed magic fails
- truncated envelope fails
- oversized header fails
- invalid nonce length fails
- unknown algorithm fails
- newline in IDs/media type is rejected
- non-positive chunk index is rejected

Add simulator-related tests where practical:

- encrypted chunk upload path still works
- downloaded stream bundle can be decrypted/verified
- `--encrypt=false` preserves development compatibility if implemented

Existing backend tests must continue to pass.

## Security requirements

- do not implement crypto primitives manually
- do not use unauthenticated encryption
- do not use insecure randomness
- do not log raw keys
- do not log plaintext chunk bytes
- do not log decrypted plaintext
- do not upload keys
- do not store keys in SQLite
- do not add keys to ZIP bundles
- do not add server-side decryption
- do not add public decryption endpoints
- do not claim production-ready cryptography
- document limitations clearly

## Validation

Run:

```bash
gofmt -w .
go test ./...
```

Manual smoke test:

```bash
go run ./cmd/api
```

In another terminal:

```bash
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

Expected:

- simulator uses encryption by default
- chunks upload successfully
- stream completes
- bundle downloads
- simulator verifies it can decrypt downloaded encrypted chunks
- no plaintext is printed
- no key is uploaded

Test with a key file:

```bash
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle --key-file /tmp/safety-recorder-sim.key.json
```

Then run again with the same key file and confirm loading works:

```bash
go run ./cmd/simclient --chunks 2 --interval 1s --download-bundle --key-file /tmp/safety-recorder-sim.key.json
```

## Output after implementation

Summarize:

1. files changed
2. encryption scheme implemented
3. libraries used
4. simulator flags added
5. tests added
6. docs added/updated
7. what remains out of scope
8. whether all tests pass
