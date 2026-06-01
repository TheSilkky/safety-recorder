# Simulator

The simulator CLI lives at `cmd/simclient`. It exercises the current Proofline ingest flow that a future recording client is expected to use. It logs in to the main `/v1` API with a local account session, then encrypts generated test bytes, local pre-recorded files, or optional ffmpeg test segments with the v1 client-side envelope before upload. Each intended chunk upload includes a stable `Idempotency-Key`, and the simulator verifies one equivalent replay without printing the raw key.

The simulator covers generic incidents only. It does not set optional
incident-mode metadata for emergency incidents, interaction records, safety
checks, or evidence notes.

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
public `/v1` exposure, browser decryption, mobile client behavior, a public
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

```bash
SAFE_AUTH_BOOTSTRAP_SECRET='replace-with-local-bootstrap-secret' \
go run ./cmd/api
```

For a new local database, create an admin account through the private
`/admin` bootstrap screen or `POST /admin/bootstrap`, then remove
`SAFE_AUTH_BOOTSTRAP_SECRET` and restart the server. See
[deployment](deployment.md) for the bootstrap flow.

Then run:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 12 --interval 5s
```

The simulator creates a read-only incident viewer token for the flow but omits
token-bearing viewer URLs from output.

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
  --download-bundle
```

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

To verify an existing encrypted stream bundle without uploading anything:

```bash
go run ./cmd/simclient \
  --verify-bundle /tmp/proofline-stream-bundle.zip \
  --key-file /tmp/proofline-sim.key.json
```

For desktop-recorder bundles, `--verify-bundle` may use `--stage-dir` instead of
`--key-file`; it then reads the simulator key from that stage directory. Offline
verification checks the bundle manifest, encrypted chunk hashes where present,
and local decryption with the simulator key. It does not export plaintext.

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
does not call them or add raw key custody behavior.

## Encryption

Encryption is enabled by default:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

The simulator prints a non-secret `key_id`, but it never prints the raw key or decrypted plaintext.

To reuse a simulator key across runs:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle --key-file /tmp/proofline-sim.key.json
```

If the key file exists, the simulator loads it. If it does not exist, the simulator creates it with restrictive permissions where practical. Do not upload or commit simulator key files.

Older examples may use `/tmp/safety-recorder-sim.key.json`; the file name is not part of the encryption protocol.

To preserve the old raw fake chunk behavior:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --encrypt=false
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

The simulator does not yet call the duplicate chunk reconciliation route. Future
ambiguous-network and process-restart drills can use
`POST /v1/incidents/{incident_id}/chunks/reconcile` to compare a local expected
chunk fingerprint with accepted server metadata without re-uploading ciphertext.
Broader simulator drills remain future work planned in
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
| `--key-file` | Optional local simulator key file. |
| `--wrapped-key-output` | Write a simulator-only contact-wrapped key metadata artifact. |
| `--contact-key-file` | Optional local simulator trusted-contact private key file for wrapped-key metadata. |
| `--wrapped-key-contact-id` | Local simulator trusted-contact ID for wrapped-key metadata. |
| `--verify-bundle-decryption` | Locally decrypt downloaded bundles when encryption is enabled. |
| `--simulate-failure-every` | Intentionally fail every Nth chunk hash before retrying. |
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
