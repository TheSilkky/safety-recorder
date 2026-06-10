# Simulator

The simulator CLI lives at `cmd/simclient`. It exercises the current Proofline
ingest flow that a future recording client is expected to use. It logs in to
the main `/v1` API with a local account session, then encrypts generated test
bytes, local pre-recorded files, or optional ffmpeg test segments with the
accepted post-quantum envelope before upload. Each intended chunk upload
includes a stable `Idempotency-Key`, and the simulator verifies one equivalent
replay without printing raw key material.

The simulator covers generic incidents only. It does not set optional
incident-mode metadata for emergency incidents, interaction records, safety
checks, or evidence notes.

The current simulator uploads complete encrypted chunks directly to the main
API. It does not issue relay sessions, upload through `cmd/stream-ingress`,
subscribe to relay fanout, or verify relay readiness; relay smoke checks are
documented separately in [deployment](deployment.md) and
[regional-stream-ingress-relay.md](regional-stream-ingress-relay.md).

## Desktop Recorder Simulator

The desktop recorder simulator is an expanded `simclient` mode for backend
reference testing. It is not a production desktop app, desktop app package, or a
replacement for planned mobile clients.

It uses the current complete encrypted chunk upload contract: create an
incident and media stream through the main `/v1` API, capture or read short
local test intervals, encrypt each completed interval, write encrypted chunks
and immutable upload metadata to local staging, retry failed uploads by
resending complete chunks, and complete the stream only after the local staged
queue is fully uploaded. With `--fail-incomplete-stream`, it can mark the
stream failed when local state proves the staged queue cannot be fully uploaded.

Poor-network controls are adjustable rather than one fixed failure mode. The
simulator can inject latency, jitter, request timeouts, bandwidth ceilings,
intermittent offline failures, upload failure rates, and process restart or
resume drills. These controls exercise local staging, retry scheduling, and
stream completion behavior without requiring partially uploaded bytes to become
server-visible evidence.

The desktop simulator continues using account-aware local sessions without
turning this repository into a production desktop app. Simulator credentials
are local development credentials only. The simulator does not add OAuth, JWT,
broad public `/v1` exposure, browser decryption, mobile client behavior, a public
account portal, resumable uploads, partial-upload lease sessions, or
server-visible queue summary routes.

Supported desktop recorder sources:

- `generated`: random local test bytes, encrypted and staged before upload.
- `files`: one or more local pre-recorded files supplied with repeated
  `--input-file` flags. Each file becomes one encrypted staged chunk.
- `ffmpeg`: invokes a local `ffmpeg` binary to record or encode short video
  segments, then encrypts, stages, and uploads each completed segment as a
  complete chunk while capture is running. This is local encoding plus
  complete-chunk upload, not public live playback or server-visible partial
  streaming.

The simulator may also prototype contact-wrapped key metadata in local
development artifacts. That design is documented in
[contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md)
and must keep raw media keys, contact private keys, plaintext, and decryption
capabilities out of server storage, logs, and bundle manifests unless a later
explicit production key-custody task changes the boundary.

Do not add resumable uploads, partial-upload lease sessions, or server-visible
queue summary routes just to support that simulator. The resumable-upload
decision is planned separately in
[resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md).

## Basic Flow

Start the backend first:

For repeatable local configuration, set the bootstrap secret through a private
secret file referenced by TOML:

```toml
[auth]
bootstrap_secret_file = "/path/to/local-bootstrap-secret"
```

Then run:

```bash
go run ./cmd/api --config /path/to/proofline.toml
```

For a one-off local shell, an environment override remains supported:

```bash
SAFE_AUTH_BOOTSTRAP_SECRET='replace-with-local-bootstrap-secret' \
go run ./cmd/api
```

For a new local database, create an admin account through the private
`/admin` bootstrap screen or `POST /admin/bootstrap`, then remove the
bootstrap secret from TOML, the environment, or the secret mount and restart
the server. See
[deployment](deployment.md) for the bootstrap flow.

Then run:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 12 --interval 5s
```

The simulator creates a read-only incident viewer token for the flow but omits
token-bearing viewer URLs from output.

## Duplicate Reconciliation Drill

To verify the private duplicate chunk reconciliation path after an accepted
streamed chunk:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --chunks 3 \
  --interval 1s \
  --reconcile-duplicate
```

The drill uploads chunk 1, verifies the normal idempotent replay path, calls
`POST /v1/incidents/{incident_id}/chunks/reconcile` with locally known
ciphertext metadata, and then sends a deliberate metadata mismatch for the same
chunk identity to confirm the safe `409 duplicate_chunk_conflict` response. It
does not re-upload ciphertext during reconciliation and does not print raw
session tokens, idempotency keys, request bodies, uploaded bytes, plaintext,
raw keys, local staging paths, stored paths, object keys, or token-bearing
viewer URLs.

Recorder clients should use reconciliation after a `409 duplicate_chunk` or an
uncertain restart state when local durable metadata can identify the intended
stream ID, chunk index, media type, time range, ciphertext byte size,
ciphertext SHA-256, and normalized original filename. A matched reconciliation
confirms that the server already accepted the expected complete encrypted
chunk. A conflict means the local queue and server evidence metadata disagree;
the client must not overwrite server evidence and should preserve local
diagnostics for operator review.

## Desktop Generated Staging Flow

To stage generated encrypted chunks durably before upload:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --desktop-recorder \
  --stage-dir /tmp/proofline-desktop-stage \
  --chunks 5 \
  --download-bundle
```

The stage directory contains a manifest and encrypted staged chunks. When
`--key-file` is omitted, the simulator creates a local key file in the stage
directory for restart recovery. The manifest stores incident and stream
identity, safe chunk metadata, retry status, ciphertext byte sizes, and
ciphertext SHA-256 values. It does not store raw viewer tokens, request bodies,
plaintext, raw media keys, server stored paths, or absolute local file paths.
On resume, the manifest must describe a contiguous stream chunk sequence.

To rehearse restart recovery, first stage without uploading:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --desktop-recorder \
  --stage-dir /tmp/proofline-desktop-stage \
  --chunks 5 \
  --stage-only
```

Then resume the staged queue after a process restart:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --desktop-recorder \
  --stage-dir /tmp/proofline-desktop-stage \
  --resume-staged \
  --download-bundle \
  --verify-bundle-decryption=false
```

PQ bundle decryption verification needs the local wrapping records produced
during the same simulator process. After a process restart, staged PQ chunks can
still be uploaded and bundled, but local decrypt verification should be disabled
unless a future simulator task persists local wrapping records. The explicit
`--envelope v1` compatibility path can still use offline key-file verification.

## Desktop File Input Flow

To upload local pre-recorded files as complete encrypted chunks:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --desktop-recorder \
  --stage-dir /tmp/proofline-file-stage \
  --desktop-source files \
  --media-type video \
  --input-file /tmp/chunk-001.mp4 \
  --input-file /tmp/chunk-002.mp4 \
  --download-bundle
```

Each input file becomes one encrypted staged chunk. Use short pre-segmented
files so whole-chunk retry stays realistic for the current complete-upload API.

## Desktop ffmpeg Flow

`ffmpeg` is optional and must already be installed on the local machine. The
simulator does not vendor or package ffmpeg.

The default ffmpeg source uses a generated video test pattern:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --desktop-recorder \
  --stage-dir /tmp/proofline-ffmpeg-stage \
  --desktop-source ffmpeg \
  --media-type video \
  --ffmpeg-duration 15s \
  --ffmpeg-segment-time 5s \
  --download-bundle
```

For desktop capture, configure ffmpeg for the local platform. For example, an
X11 test run can use:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --desktop-recorder \
  --stage-dir /tmp/proofline-ffmpeg-stage \
  --desktop-source ffmpeg \
  --media-type video \
  --ffmpeg-input-format x11grab \
  --ffmpeg-input :0.0 \
  --ffmpeg-video-codec mpeg4 \
  --ffmpeg-duration 15s \
  --ffmpeg-segment-time 5s \
  --download-bundle
```

The ffmpeg path writes temporary encoded segments under the local staging area,
encrypts completed segments into durable staged chunks, uploads staged chunks
while capture is running, and removes the temporary encoded segment files after
staging. Uploaded chunks still use
`POST /v1/incidents/{incident_id}/chunks` with complete encrypted payloads. If
an upload fails after a stream has been created and `--fail-incomplete-stream`
is set, the simulator attempts to mark that stream failed instead of leaving the
failure decision implicit.

## Bundle Download Flow

To test encrypted bundle download through the incident viewer:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

This creates a media stream, uploads encrypted chunks with `stream_id`, completes
the stream, downloads the completed encrypted ZIP bundle through the incident
viewer, and verifies local decryption.

To also keep a local copy of the encrypted ZIP bundle:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --chunks 5 \
  --interval 1s \
  --download-bundle \
  --key-file /tmp/proofline-sim.key.json \
  --bundle-output /tmp/proofline-stream-bundle.zip
```

`--bundle-output` writes only the encrypted ZIP bundle that the server returned.
It requires encrypted uploads, refuses to overwrite an existing output file, and
does not write decrypted chunks or playable media.

Offline verification remains available for explicit v1 compatibility bundles.
To verify an existing v1 encrypted stream bundle without uploading anything:

```bash
go run ./cmd/simclient \
  --envelope v1 \
  --verify-bundle /tmp/proofline-stream-bundle.zip \
  --key-file /tmp/proofline-sim.key.json
```

For desktop-recorder bundles, `--verify-bundle` may use `--stage-dir` instead of
`--key-file`; it then reads the simulator key from that stage directory. Offline
verification checks the bundle manifest, encrypted chunk hashes where present,
and local decryption with the simulator key. It does not export plaintext. PQ
bundle ZIPs intentionally do not include wrapped-key records, so PQ bundle
decryption verification is same-run only while the simulator still has local
wrapping records.

## Contact-Wrapped Key Metadata

The simulator can write a local contact-wrapped media-key artifact for
development testing:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --chunks 5 \
  --interval 1s \
  --download-bundle \
  --envelope v1 \
  --wrapped-key-output /tmp/proofline-sim-wrapped-keys.json
```

When `--wrapped-key-output` is set, the simulator creates or loads a local
development trusted-contact key file. If `--contact-key-file` is omitted, the
default file is `proofline-sim-contact.key.json` next to the wrapped-key
artifact. The contact private key file is local simulator state and is written
with restrictive permissions where practical.

The wrapped-key artifact is a companion development file. It records the
incident ID, stream ID, simulator media key ID, contact ID, contact key ID,
wrapping algorithm, and wrapped-key ciphertext. It does not include raw media
keys, contact private keys, unwrapped secrets, plaintext, viewer tokens,
incident tokens, filesystem paths, object keys, or secret-bearing URLs.

The wrapping profile is `age-v1-x25519` through the maintained
`filippo.io/age` library. The simulator reads the written artifact and unwraps
the media key through the local development contact key before bundle decrypt
verification. This remains simulator-only: it does not add production key
custody, backend decryption, browser decryption, key escrow, trusted-contact
accounts, public product authentication, or bundle manifest key records. The
server now has private authenticated metadata routes for contact public keys,
sharing grants, and grant-bound wrapped-key records, but this simulator option
does not call them or add raw key custody behavior. It requires
`--envelope v1`; the runtime default PQ path uses the accepted wrapped-key API
profile instead of this age-based local artifact.

## Encryption

Encryption is enabled by default:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

The simulator prints a non-secret PQ recipient key ID by default, but it never
prints the raw key, decapsulation seed, or decrypted plaintext. With
`--envelope v1`, it prints the non-secret v1 `key_id`.

To reuse a simulator key across runs:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle --key-file /tmp/proofline-sim.key.json
```

If the key file exists, the simulator loads it. If it does not exist, the simulator creates it with restrictive permissions where practical. Do not upload or commit simulator key files.

Older local examples may have used `/tmp/safety-recorder-sim.key.json`; that
name is historical and is not part of the current protocol or default
simulator artifact layout.

To preserve the old raw fake chunk behavior:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --encrypt=false
```

To use the old v1 AES-GCM compatibility envelope explicitly:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --envelope v1 --chunks 5 --interval 1s --download-bundle
```

This is only for development compatibility. See [encryption.md](encryption.md) for the envelope and key file format.

## Failure And Retry Flow

To test hash failure and retry behavior:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 12 --interval 2s --simulate-failure-every 4
```

Every fourth chunk intentionally fails SHA-256 verification before being
retried. Hash-mismatch attempts do not reserve idempotency state because the
server has not accepted the immutable fingerprint. The first successfully
uploaded chunk is then resent with the same `Idempotency-Key` to verify
equivalent retry success.

For desktop-recorder retry flows, an upload that was accepted by the server but
lost its response can be retried as the same complete encrypted staged chunk
with the same `Idempotency-Key`. If the server returns `200 OK` with
`Idempotency-Replayed: true`, the simulator treats the staged chunk as uploaded
without printing the raw idempotency key, uploaded bytes, local staging path, or
session token.

The standard simulator can call the duplicate chunk reconciliation route with
`--reconcile-duplicate` to compare a local expected chunk fingerprint with
accepted server metadata without re-uploading ciphertext. Broader desktop
recorder ambiguous-network and process-restart reconciliation drills remain
future work planned in
[cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md).

## Poor-Network Desktop Controls

Desktop recorder mode supports poor-network controls on the existing HTTP
client path:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient \
  --desktop-recorder \
  --stage-dir /tmp/proofline-network-stage \
  --chunks 5 \
  --network-latency 200ms \
  --network-jitter 100ms \
  --network-timeout 5s \
  --network-bandwidth 256KiB \
  --network-offline-every 3 \
  --network-offline-for 2s \
  --network-failure-rate 0.2 \
  --desktop-max-attempts 8 \
  --desktop-retry-delay 1s \
  --download-bundle
```

Retries resend the same complete encrypted staged chunk with the same
`Idempotency-Key`. The server still sees only complete encrypted chunk upload
attempts and durable metadata for accepted chunks.

## Useful Flags

| Flag | Purpose |
|---|---|
| `--api` | Main API base URL. Defaults to `http://localhost:8080`. |
| `--viewer` | Incident viewer base URL. Defaults to `http://localhost:8080` because the viewer is mounted on the main listener. |
| `--username` | Proofline account username. Defaults to `PROOFLINE_SIM_USERNAME`. |
| `--password` | Proofline account password. Defaults to `PROOFLINE_SIM_PASSWORD`. |
| `--chunks` | Number of chunks to upload. |
| `--interval` | Delay between chunk uploads. |
| `--chunk-size` | Size of each fake plaintext chunk before optional encryption. |
| `--media-type` | Media type to upload. |
| `--complete-stream` | Mark the uploaded media stream complete. |
| `--download-bundle` | Download the completed stream bundle through the incident viewer. |
| `--bundle-output` | Write the downloaded encrypted stream bundle ZIP to a new local file. |
| `--verify-bundle` | Verify an existing encrypted stream bundle ZIP without uploading. |
| `--encrypt` | Encrypt simulated chunk bytes before upload. Defaults to `true`. |
| `--envelope` | Encrypted chunk envelope profile. Defaults to `pq`; use `v1` only for compatibility. |
| `--key-file` | Optional local simulator key file. |
| `--wrapped-key-output` | Write a simulator-only contact-wrapped key metadata artifact. |
| `--contact-key-file` | Optional local simulator trusted-contact private key file for wrapped-key metadata. |
| `--wrapped-key-contact-id` | Local simulator trusted-contact ID for wrapped-key metadata. |
| `--verify-bundle-decryption` | Locally decrypt downloaded bundles when encryption is enabled. |
| `--simulate-failure-every` | Intentionally fail every Nth chunk hash before retrying. |
| `--reconcile-duplicate` | Reconcile accepted chunk 1 metadata and verify a safe duplicate-conflict response in the standard simulator flow. |
| `--close` | Close the incident when complete. |
| `--desktop-recorder` | Enable durable desktop recorder simulator mode. |
| `--stage-dir` | Local durable staging directory for desktop recorder mode. |
| `--resume-staged` | Resume uploading an existing staged desktop recorder queue. |
| `--stage-only` | Create local encrypted staging without uploading. |
| `--fail-incomplete-stream` | Mark the stream failed if the staged queue cannot fully upload. |
| `--desktop-source` | Desktop source: `generated`, `files`, or `ffmpeg`. |
| `--input-file` | Local pre-recorded input file for `--desktop-source=files`; may be repeated. |
| `--desktop-max-attempts` | Maximum upload attempts per staged desktop recorder chunk. |
| `--desktop-retry-delay` | Delay before retrying a failed staged chunk upload. |
| `--network-latency` | Simulated latency before each HTTP request. |
| `--network-jitter` | Additional random simulated latency before each HTTP request. |
| `--network-timeout` | HTTP client request timeout. |
| `--network-bandwidth` | Simulated upload bandwidth ceiling. |
| `--network-offline-every` | Fail every Nth request before sending to simulate an offline window. |
| `--network-offline-for` | Delay used with simulated offline windows. |
| `--network-failure-rate` | Random request failure rate from `0` to `1`. |
| `--network-seed` | Seed for deterministic poor-network simulation. |
| `--ffmpeg-bin` | ffmpeg executable name or path for `--desktop-source=ffmpeg`. |
| `--ffmpeg-input-format` | ffmpeg input format, such as `lavfi` or `x11grab`. |
| `--ffmpeg-input` | ffmpeg input source. |
| `--ffmpeg-video-codec` | ffmpeg video codec for segmented desktop capture. Defaults to `mpeg4`. |
| `--ffmpeg-duration` | ffmpeg capture duration. |
| `--ffmpeg-segment-time` | ffmpeg segment duration for complete chunk uploads. |
