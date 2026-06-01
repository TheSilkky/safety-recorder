# AGENTS.md

## Project rules

- Keep the backend small, boring, and testable.
- Prefer Go standard library where practical.
- This repository is the Proofline Go server backend only. In the current organisation layout it is `open-proofline/server`.
- Do not add web-client, iOS-client, Android-client, or shared-protocol implementation to this repository unless the maintainer explicitly changes the repository strategy.
- Do not add React, Node, npm, Docker Compose, Kubernetes, OAuth, JWT, user accounts, SMS, Messenger, push notifications, cloud services, or public admin dashboards unless explicitly requested.
- Treat uploaded chunks as immutable.
- Never overwrite stored chunks or evidence bundle contents.
- Never log raw viewer tokens, incident tokens, request bodies, uploaded file bytes, Authorization headers, plaintext, raw keys, or future token-like values.
- Keep the main API/viewer route tree and the private `/admin` dashboard route tree on separate listener groups and separate muxes.
- Do not route private write/admin routes from public incident viewer edges.
- Public incident viewer routes must remain read-only.
- ZIP bundle download routes must not expose filesystem paths or accept client-provided stored paths.
- Generated ZIP entry names must be controlled by the server.
- Completed evidence bundles are encrypted chunk bundles, not decrypted/playable media exports.
- Use stable, documented crypto libraries only. Do not implement cryptographic primitives. Do not create custom AEAD, block modes, padding, MAC, KDF, or random generator logic.
- Preserve the current backend ciphertext-only implementation unless a task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour incidentally.
- Key custody/decryption changes must be explicit security-sensitive work and update the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.
- Preserve the current deployment model: main `/v1` behind the reviewed localhost/LAN/WireGuard/firewall boundary, private `/admin` behind its own private listener, and only read-only incident viewer paths behind HTTPS/reverse proxy when exposed.
- Separate bind addresses are a deployment boundary, not a complete security model.
- Treat Codex prompts as scoped change requests, not open-ended permission to expand the project.
- Do not implement newly discovered future work during an unrelated task; document it as an issue/backlog item instead.
- For larger changes, start from a clean working tree or an explicit checkpoint commit.
- Backlog scanning should create draft Markdown files first, not GitHub issues directly.
- Do not create public GitHub issues from backlog drafts until the maintainer has reviewed them.
- Never put raw tokens, secrets, private deployment details, exploit details, or user safety data into public issue drafts.

## Current project shape

- Product documentation uses the name Proofline.
- This repository is the Go server backend component only.
- Current organisation: `open-proofline`.
- Current server repository: `open-proofline/server`.
- Planned future companion repositories: `open-proofline/web-client`, `open-proofline/ios-client`, `open-proofline/android-client`, and `open-proofline/protocol`.
- The Go module path is `github.com/open-proofline/server` at the repository root, release binaries use `proofline-server-*` names, and the published GHCR image is `ghcr.io/open-proofline/server`.
- Some compatibility identifiers, including the v1 simulator encryption envelope and default SQLite filename, may still use earlier `safety-recorder` names until separate protocol or data-layout migrations are explicitly performed.
- SQLite metadata by default.
- Optional PostgreSQL metadata when explicitly configured.
- Local disk blob storage by default.
- Optional S3-compatible encrypted blob storage for committed chunks.
- No coordination backend by default.
- Optional Valkey/Redis-compatible coordination when explicitly configured.
- Main API/viewer listener group for authenticated `/v1` routes, canonical `/i/{token}` viewer routes, legacy `/e/{token}` compatibility aliases, and token-neutral `/static/...` viewer assets.
- Private admin-dashboard listener group for `/admin` routes and token-neutral `/admin/static/...` assets.
- Uploaded chunks may be grouped into media streams.
- Media streams can be marked `open`, `complete`, or `failed`.
- Completed streams and incidents can be downloaded as encrypted ZIP evidence bundles.
- Simulator CLI exists for incident upload/check-in/encryption test flows.
- The current simulator encryption envelope is development/test oriented.
- Future product scope includes emergency incidents, non-emergency interaction records, timed safety checks, and evidence notes.
- The current backend implements local username/password accounts, main `/v1` account/session authentication, admin account management routes, and owner/admin incident authorization.
- The current backend implements optional incident mode, capture profile, escalation policy, and sharing state metadata fields on private incident create/read routes, but these fields do not grant access, send notifications, change retention, change key custody, expose trusted-contact workflows, or change public viewer and bundle behavior.
- The current backend implements private owner-scoped and admin-global incident deletion routes, deletion tombstones, retryable blob deletion, and optional closed-incident retention through a background worker.
- The current backend does not yet implement mode-driven access, trusted-contact accounts, dead-man switch notifications, public account workflows, or public `/v1` product authentication.
- Planned production-cluster scope may add cluster-safe idempotent upload semantics and operation-level use of coordination. These additions must not remove SQLite, optional PostgreSQL metadata, local filesystem support, the optional S3-compatible blob backend, or the optional Valkey/Redis-compatible coordination backend.
- Regional stream-ingress relay work is planning-only unless explicitly scoped for implementation; any future relay must stay upload-only, temporary, ciphertext-only, and subordinate to the core API for authorization, idempotency, durable blob commits, and metadata.
- Future encryption direction should be a hybrid key custody model.
- Docker and GitHub Actions/GHCR publishing exist, but deployment expansion should not be added unless explicitly requested.

## Commands

From the repository root:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
```

Use `go vet ./...` when reviewing larger changes:

```bash
go vet ./...
```

## Review expectations

Before accepting Codex changes, check:

- tests pass
- generated code stays in scope
- private/public route separation is preserved
- raw tokens are not logged
- plaintext and raw keys are not logged
- ZIP downloads use safe headers and controlled paths
- documentation still matches `README.md`
- future web, iOS, Android, or protocol work was not accidentally added to this server repository
- key custody/decryption changes are explicit and security-reviewed
- no public-production readiness is implied unless deployment hardening has actually been implemented
