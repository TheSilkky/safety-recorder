# Codex Prompt: Simulator Maintenance

Update or review the simulator CLI for the current backend.

Do **not** add unrelated product features.
Do **not** implement iOS code.
Do **not** add backend decryption.
Do **not** change key custody model unless explicitly requested.

## Source of truth

Before making changes, read current source-of-truth files as relevant:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
- relevant files in `docs/`
- relevant source files
- relevant tests
- relevant issue or PR, if this is issue/PR work

Do not rely on stale assumptions from this prompt if the repository has changed.
## Global constraints

- Keep changes scoped to the task.
- Do not add unrelated features.
- Do not weaken security warnings.
- Do not claim production readiness.
- Do not expose `/v1` publicly.
- Do not log raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, or future token-like values.
- Do not add React, Node, npm, OAuth, JWT, user accounts, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features unless explicitly requested.
- Prefer Go standard library where practical.
- Preserve private/public listener separation.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour as an incidental implementation detail.
- Any key custody/decryption change must be an explicit security-sensitive task that updates the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.

## Current simulator purpose

The simulator acts as a local backend reference client for current and future
recorder flows.

The current `cmd/simclient` uploads fake chunks through the private API and can
verify encrypted bundle downloads locally. A future local desktop recorder
simulator client may extend this simulator boundary, but it should remain a
simulator, not a production desktop app or replacement for planned mobile
clients.

The future desktop simulator should support adjustable poor-network simulation
for latency, jitter, request timeouts, bandwidth ceilings, intermittent offline
windows, upload failure rates, and process restart or resume drills.

It should use the current local account/session flow for private `/v1`
authentication. Simulator maintenance must not add OAuth, JWT, public `/v1`
exposure, trusted-contact accounts, public account workflows, or extra
account-management routes incidentally.

It should exercise:

- incident creation
- incident token creation
- media stream creation
- chunk uploads with `stream_id`
- stream chunk indexes starting at `1`
- SHA-256 validated upload flow
- client-side encryption envelope for simulated chunks
- local simulator key-file loading/saving, when configured
- checkins
- stream completion
- optional failure/retry behaviour
- adjustable poor-network simulation for future desktop recorder flows, when
  that simulator exists
- optional encrypted bundle download verification
- local decrypt verification of downloaded bundles when encryption is enabled

## Review or update focus

Check that the simulator:

- uses the private API base URL for `/v1` routes
- uses the public viewer base URL for printed incident viewer links
- creates a media stream before uploading chunks
- uploads chunks with `stream_id`
- keeps streamed chunk indexes unique per `(incident_id, stream_id, chunk_index)`
- starts stream chunk indexes at `1`
- encrypts fake chunks by default
- supports `--encrypt=false` only for development compatibility
- supports `--key-file` without printing the key
- does not print raw keys or plaintext
- sends checkins during simulation
- completes the stream by default unless configured otherwise
- can simulate bad-hash failure and retry
- can simulate poor network conditions through adjustable controls when working
  on the future desktop recorder simulator
- can optionally download a completed encrypted bundle
- can decrypt-verify downloaded encrypted chunks locally
- prints readable progress output
- does not require public auth
- does not assume playable media exists
- keeps near-term account support local or fixture-only until explicit account
  and access-control backend work exists
- does not add resumable uploads, upload leases, or server-visible queue
  summary routes just to support simulator work unless explicitly requested

## Tests

Add tests for reusable simulator helpers where practical.

Do not overbuild tests around terminal output unless simple.

Existing backend tests must continue to pass.

## Validation

Run:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
```

Manual smoke test:

```bash
SAFE_AUTH_BOOTSTRAP_SECRET='replace-with-local-bootstrap-secret' go run ./cmd/api
```

In another terminal, create the first local admin if the test database does not
already have one:

```bash
curl -sS -X POST http://127.0.0.1:8081/admin/bootstrap \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'bootstrap_secret=replace-with-local-bootstrap-secret' \
  --data-urlencode 'username=admin' \
  --data-urlencode 'password=replace-with-a-long-local-password'
```

Then run the simulator with account credentials:

```bash
PROOFLINE_SIM_USERNAME='admin' \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

## Output

Summarize:

1. files changed
2. simulator behaviour updated
3. encryption/key-file impact
4. validation commands run
5. any follow-up work
