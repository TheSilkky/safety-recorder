# Proofline Server

[![CI](https://github.com/open-proofline/server/actions/workflows/ci.yml/badge.svg)](https://github.com/open-proofline/server/actions/workflows/ci.yml)
[![Latest Tag](https://img.shields.io/github/v/tag/open-proofline/server?sort=semver)](https://github.com/open-proofline/server/tags)
[![License: AGPL-3.0-only](https://img.shields.io/badge/license-AGPL--3.0--only-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26.4-00ADD8?logo=go&logoColor=white)](go.mod)
[![Status: Experimental](https://img.shields.io/badge/status-experimental-orange.svg)](#security-warning)
[![Security Policy](https://img.shields.io/badge/security-policy-blue.svg)](SECURITY.md)
[![GHCR](https://img.shields.io/static/v1?label=GHCR&message=ghcr.io%2Fopen-proofline%2Fserver&color=blue&logo=github)](https://github.com/orgs/open-proofline/packages/container/package/server)

Proofline Server is the experimental Go server backend for encrypted incident capture. It receives already-encrypted recording chunks through authenticated main `/v1` routes, stores metadata in SQLite by default or optional PostgreSQL, keeps encrypted blobs on local disk by default or in optional S3-compatible object storage, enforces default local staging and account-scoped committed blob quotas, serves a private admin dashboard under `/admin`, uses optional Valkey/Redis-compatible coordination for startup checks, route-class counters, and short-lived complete-upload leases when explicitly configured, supports optional browser cookie sessions for a future web client, supports disabled-by-default configurable account registration with SMTP-backed email verification for open self-hosted registration, supports email challenge, TOTP, and disabled-by-default WebAuthn/FIDO2 passkey or roaming security-key second-factor setup for account gating, supports private-admin assisted second-factor reset for lost-factor recovery, and exposes a token-scoped read-only viewer for incident review.

> Repository role: this repository is the server/backend component only. In the multi-repo layout it is `open-proofline/server`, not the full Proofline product suite.
>
> Artifact note: the Go module path is `github.com/open-proofline/server`, the published GHCR image is `ghcr.io/open-proofline/server`, and release binaries use `proofline-server-*` names. Current runtime protocol and default data-layout identifiers use Proofline names. Historical reports and archived prompts may still mention earlier `safety-recorder` identifiers.

## Security Warning

> This project is not production-ready public infrastructure. The main `/v1` API now requires local account sessions for product routes and uses app-level route-class rate limits, so it is no longer an unauthenticated control plane. Broad public exposure still needs route-by-route deployment review, TLS, edge abuse controls, browser credential review, logging review, proxy hardening, and operational testing. Configurable public self-registration is disabled by default; enabling open registration adds only email-verified account creation and does not by itself approve broad public `/v1` exposure. Optional browser cookie sessions add CSRF checks and configured credentialed CORS for future web-client use, but they also need reviewed deployment rules. Existing `/admin/api/...` JSON routes are mounted only on the private-admin listener and remain authenticated admin-only routes. The private-admin listener also serves the `/admin` dashboard surface and must stay behind localhost, LAN, WireGuard, a firewall, or a strict reverse proxy. Separate bind addresses are a deployment boundary, not a complete security model.
>
> Public web-client deployments must follow the reviewed route, CORS, CSRF,
> cookie, cache, edge, and logging boundary in
> [docs/public-web-client-deployment-boundary.md](docs/public-web-client-deployment-boundary.md).

## What It Is

This repository currently contains the Go server backend only. It does not
contain the web client, iOS client, Android client, protocol repository,
account portal, production recording client, or mobile app code. The simulator
may capture or encode local test segments for backend reference flows only.

The intended long-term Proofline product is broader than emergency-only recording: it should support private encrypted incident capture for emergencies, non-emergency interaction records, timed safety checks, and evidence notes.

Future client repositories are expected to record audio/video and supporting metadata in short chunks, encrypt them locally, and upload them continuously so already-uploaded evidence is retained if a phone is lost, damaged, powered off, or taken.

Evidence bundles are ZIP files containing encrypted chunks and JSON manifests. They are not decrypted, playable, or merged media exports.

The simulator encrypts generated chunks by default with the accepted
post-quantum envelope (`ML-KEM-768 + HKDF-SHA384 + AES-256-GCM`), can stage
local file or ffmpeg test segments in desktop-recorder mode, and verifies
downloaded bundles locally when the run still has the local wrapping records.
Keys remain client-side and are not uploaded to the backend. Future production
key custody is expected to use a hybrid trusted-contact model; see
[docs/key-custody.md](docs/key-custody.md).

Planned production-cluster work is additive. SQLite metadata and local filesystem blob storage remain supported. Optional PostgreSQL metadata, S3-compatible object storage, and Valkey/Redis-compatible coordination are available only when explicitly configured. Complete-upload idempotency is implemented through metadata-backed upload-operation state, and Valkey can hold short-lived complete-upload leases and retry hints when configured. Resumable or partial-upload protocols remain future work. See [docs/production-cluster-scope.md](docs/production-cluster-scope.md).

## Open Proofline Repository Roles

The current organisation is `open-proofline`, with current and planned
responsibilities split across repositories:

| Repository | Responsibility |
|---|---|
| `open-proofline/server` | Go backend, main API, private admin web surface, read-only incident viewer, storage, migrations, deployment docs, and server release workflow. |
| `open-proofline/web-client` | Account portal, authorised incident review, trusted-contact access, and eventual replacement for the current token-only viewer. |
| `open-proofline/ios-client` | iOS incident capture, encrypted staging, upload, local account flows, and platform-specific recording behavior. |
| `open-proofline/android-client` | Android incident capture, encrypted staging, upload, local account flows, and platform-specific recording behavior. |
| `open-proofline/protocol` | Shared API specs, encryption envelope specs, bundle manifests, compatibility matrix, and conformance tests. |

This repository should remain scoped to the server/backend role. Product-level
or client-specific work should be documented here only as planning context until
the relevant separate repository scope is ready.

## Planned Incident Modes

Proofline should separate capture from escalation. A user may want to preserve a private encrypted record without treating every recording as an emergency.

Planned incident categories include:

| Mode | Purpose | Default escalation |
|---|---|---|
| Emergency incident | Active safety risk where trusted contacts may need urgent access. | Trusted-contact alert immediately or after a short configured delay. |
| Interaction record | Non-emergency record of important interactions, such as with police, security, landlords, employers, service providers, or other authorities. | No automatic escalation by default. |
| Safety check | Timed check-in flow for walking home, meeting someone, travel, fieldwork, or other elevated-risk situations. | Trusted contacts alerted if the user misses the check-in. |
| Evidence note | Quick photo, audio, location, or note bundle for damage, harassment, threats, or disputes. | No automatic escalation by default. |

The current backend stores generic incidents by default and can optionally store
`incident_mode`, `capture_profile`, `escalation_policy`, and `sharing_state`
metadata on main incident creation. Those fields are labels only: they do not
grant access, send notifications, change retention, change key custody, expose
trusted-contact workflows, or change public viewer and bundle behavior. See
[docs/incident-modes.md](docs/incident-modes.md).

Authenticated account owners can also invite local accounts into
trusted-contact relationships, register or replace trusted-contact public-key
metadata, mark contact keys lost or revoked, and create or revoke
incident/stream-scoped sharing grants for their own incidents. Those grants can
authorize private API storage and delivery of contact-wrapped CEK/media-key
metadata for owned incidents or streams. Signed-in accepted trusted contacts
can read only grant-scoped wrapped-key records whose owner relationship,
recipient-bound contact key, grant, and wrapped-key record are all active.
Relationship records alone do not add incident reads, browser or backend
decryption, public viewer changes, notifications, raw key storage, or key
escrow.

## What Works Today

- Main authenticated `/v1` API listener group
- Private-admin listener for admin-only JSON routes, `/admin`, and
  `/admin/static/...`
- Read-only incident viewer routes mounted on the main listener
- Local username/password accounts for regular users and admins
- Configurable account registration modes: disabled, admin-created accounts
  only, open self-registration with email verification, and a paid-registration
  placeholder that fails closed until billing is implemented
- Email challenge, TOTP, and disabled-by-default WebAuthn/FIDO2 second-factor
  setup. New admin-created and
  open-registration accounts require setup before main product-route access;
  existing migrated accounts default to `not_required` for preview
  compatibility. Email challenge codes are single-use, expiring, emailed
  through the configured sender, and stored only as hashes. TOTP setup stores
  authenticator-app seeds as credential material and active TOTP factors require
  per-session verification before product-route access. WebAuthn setup is
  disabled until an RP ID and exact allowed origins are configured, stores
  public credential material and single-use expiring challenge sessions, and can
  verify active passkey or roaming security-key factors for bearer and browser
  sessions. Private-admin assisted second-factor recovery can reset enrolled
  email, TOTP, and WebAuthn factors with controlled reason codes, audit
  metadata, setup-required state, and target-session revocation. It is not
  self-service recovery and does not change key custody or decrypt evidence.
- Opaque server-side sessions with expiry and revocation
- Optional main `/v1` browser cookie-session login/logout, session recovery,
  CSRF protection for cookie-authenticated unsafe requests, and credentialed
  CORS for explicitly configured web origins, subject to the public web-client
  deployment boundary in
  [docs/public-web-client-deployment-boundary.md](docs/public-web-client-deployment-boundary.md)
- Private admin-only HTML surface under `/admin` for bootstrap, login, local
  account listing, and password workflows
- SQLite metadata and local disk encrypted blob storage by default
- Optional PostgreSQL metadata backend for new deployments
- Optional S3-compatible encrypted blob storage for committed chunks
- Account-scoped committed encrypted blob quota, defaulting to 10 GB per owner
  account as a preview abuse/cost control
- Local temp-upload staging quota, defaulting to 1 GB for both local and
  S3-compatible blob backends before final chunk commit
- Immutable chunk uploads with SHA-256 verification
- `Idempotency-Key` support for equivalent complete chunk upload retries
- Optional Valkey/Redis-compatible short-lived complete-upload leases and
  `upload_in_progress` retry hints when coordination is explicitly configured
- Separate `cmd/stream-ingress` regional relay with token-neutral
  liveness/readiness routes, safe aggregate readiness categories, configured
  complete encrypted chunk upload, temporary local ciphertext staging, hash
  validation, and forwarding to service-authenticated core relay
  preflight/commit endpoints, plus main-API issuance of configured short-lived
  relay upload and fanout capabilities for authorized open streams, optimistic
  near-live encrypted SSE fanout marked unconfirmed until the core backend
  commits exact ciphertext, and bounded `confirmed`, `rejected`, or
  `terminal_failure` fanout state after the core commit outcome
- Authenticated duplicate chunk reconciliation for comparing accepted metadata with
  an expected chunk fingerprint
- Optional incident-mode, capture-profile, escalation-policy, and sharing-state
  metadata on main incident create/read routes
- Owner-only public-safe incident list/detail metadata reads for future
  web-client use, without exposing notes, chunks, checkins, stored paths, owner
  IDs, wrapped keys, ciphertext, raw keys, plaintext, or user safety narrative
- Private-admin legacy unowned incident review, reassignment, and keep-unowned
  audit APIs with count-oriented candidate metadata and controlled reason codes
- Owner-scoped account/device recipient-key metadata with active, replaced,
  revoked, and lost states for future wrapping eligibility
- Account-to-account trusted-contact relationship invites with accept, decline,
  revoke, and replace states
- Owner-scoped trusted-contact public-key metadata with pending, active,
  replaced, revoked, and lost states, plus sharing-grant records for owned
  incidents or streams
- Owner-scoped wrapped CEK/media-key metadata storage and private API delivery
  for active sharing grants
- Authenticated trusted-contact wrapped-key reads for accepted recipient
  accounts when the relationship, recipient-bound contact key, grant, and
  wrapped-key record are active
- Accepted post-quantum client-side chunk envelope as the runtime upload
  validation and simulator default
- Media streams with `open`, `complete`, and `failed` states
- Completed encrypted stream and incident ZIP evidence bundle downloads
- Scoped viewer tokens with a default 24-hour expiry
- App-level main API route limiting by safe route class, with local in-memory
  counters by default and optional Valkey/Redis-compatible counters when
  coordination is explicitly configured
- App-level public viewer rate limiting by safe route class, with local
  in-memory counters by default and optional Valkey/Redis-compatible counters
  when coordination is explicitly configured
- Validated backend-selection config defaults for SQLite metadata, optional PostgreSQL metadata, local encrypted blobs, optional S3-compatible encrypted blobs, no coordination by default, and optional Valkey/Redis-compatible coordination
- Simulator CLI for direct main-API encrypted upload, check-in, stream
  completion, bundle download/decrypt-verification, and durable
  desktop-recorder staging flows
- Docker image build and GitHub Actions / GHCR publishing

## What It Is Not Yet

- No iOS app
- No Android app
- No implemented web client or account portal
- No protocol repository or shared conformance test suite
- No production recording client implementation
- No mode-driven incident access, notification, retention, key-custody, or
  viewer behavior
- No production client-side encryption implementation
- No implemented capture stream group, stream-variant, or evidence-supersession
  model beyond the current concrete media stream upload lanes
- No implemented resumable or partial upload protocol; current Valkey upload
  leases are short-lived complete-upload hints, not durable evidence truth
- No implemented regional relay replay, metrics endpoint, production relay
  deployment automation, relay Valkey counters, or production service-identity
  rotation beyond the early static relay-to-core token; relay readiness reports
  only safe aggregate categories
- No implemented live or partial stream chunk access before stream completion
- No backend/browser decryption, raw key handling, server escrow, break-glass
  key access, or playable media export
- No self-service lost-factor recovery, password recovery, payment processing,
  subscriptions, checkout sessions, billing webhooks, OAuth, or JWT
- No push notifications, SMS, or Messenger integration
- No public account portal or public admin dashboard
- No built-in TLS, mode-specific retention policy, backup lifecycle enforcement, or production deployment hardening
- No emergency-services integration; users or trusted contacts remain responsible for contacting emergency services

## Architecture

Proofline Server runs separate main and private-admin HTTP listener groups from the same Go binary. The main listener serves authenticated non-admin `/v1` routes and the token-gated, read-only incident viewer. Existing `/admin/api/...` JSON routes stay authenticated and admin-only, but are mounted only on the private-admin listener alongside the `/admin` dashboard route tree.

```mermaid
flowchart LR
    FutureClients["Future clients<br/>separate repos"] --> Main["Main authenticated /v1 API<br/>/i/{token} viewer"]
    Main --> DB[(SQLite or PostgreSQL metadata)]
    Main --> Blobs[(Local or S3 encrypted blobs)]
    Main --> Tokens["Viewer token creation"]
    Contact["Trusted contact"] --> Main
    Main --> Bundles["Encrypted ZIP bundles<br/>completed streams only"]
    Admin["Private admin listener<br/>/admin/api API<br/>/admin dashboard"] --> DB
```

For more diagrams and package-level details, see [docs/architecture.md](docs/architecture.md) and [docs/code-map.md](docs/code-map.md). The planned cluster expansion is documented separately in [docs/production-cluster-scope.md](docs/production-cluster-scope.md).

## Quick Start

Requirements:

- Go 1.26.4
- SQLite via the bundled Go SQLite driver dependency
- TOTP generation and validation through the bundled Go OTP dependency
- WebAuthn/FIDO2 ceremony validation through the bundled go-webauthn dependency
- Local disk storage for encrypted uploaded blobs by default

Run the backend:

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

The repository root includes `proofline.toml`, a safe local-first example
config that matches the built-in defaults. TOML config is the recommended
deployment shape; existing `SAFE_*` environment variables remain supported and
override TOML values. Use `--config /path/to/proofline.toml` or
`SAFE_CONFIG_FILE=/path/to/proofline.toml` for a custom file, and prefer
`*_file` TOML keys or `SAFE_*_FILE` environment variables for secret-bearing
settings.

By default this starts:

| Listener | Address |
|---|---|
| Main API and incident viewer | `127.0.0.1:8080` |
| Private admin dashboard | `127.0.0.1:8081` |

The private admin web surface is available on the private-admin listener at
`http://127.0.0.1:8081/admin`.

In another terminal, create the first admin account:

```bash
curl -sS -X POST http://127.0.0.1:8081/admin/bootstrap \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'bootstrap_secret=replace-with-local-bootstrap-secret' \
  --data-urlencode 'username=admin' \
  --data-urlencode 'password=replace-with-a-long-local-password'
```

Stop the server, remove the bootstrap secret from TOML, the environment, or
the secret mount, and start it again. The bootstrap route is disabled once an
admin account exists.

In another terminal, run the simulator:

```bash
PROOFLINE_SIM_USERNAME=admin \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

The simulator creates an incident, creates a viewer token without printing the
token-bearing viewer URL, encrypts and uploads test chunks into a media stream,
sends checkins, completes the stream, downloads the encrypted bundle, and
verifies local decryption. See [docs/simulator.md](docs/simulator.md) for
encrypted bundle output, offline bundle verification, the durable
desktop-recorder mode, local file input, ffmpeg segment capture, and
poor-network retry controls, and simulator-only contact-wrapped key metadata
artifacts.

## Docker

Build from the repository root:

```bash
docker build -t proofline-server .
```

Run with local-only port publishing and a named data volume:

```bash
docker run --rm \
  -e SAFE_AUTH_BOOTSTRAP_SECRET='replace-with-local-bootstrap-secret' \
  -p 127.0.0.1:8080:8080 \
  -p 127.0.0.1:8081:8081 \
  -v proofline-server-data:/var/lib/proofline \
  proofline-server
```

Use the private `/admin` bootstrap screen, or `POST /admin/bootstrap` with form
fields, to create the first admin account. Then restart the container without
the bootstrap secret in TOML, the environment, or the secret mount.

Container defaults bind to `0.0.0.0` inside the container. Restrict host exposure with port publishing, firewall rules, WireGuard, or a reverse proxy. See [docs/deployment.md](docs/deployment.md).

For custom container configuration, mount a reviewed TOML file over
`/etc/proofline/proofline.toml`; the image entrypoint already passes that path
with `--config`. Keep real secrets in mounted secret files rather than in
committed TOML.

## Documentation

- [Docs index](docs/README.md)
- [v1 preview direction](docs/v1-preview-direction.md)
- [v1 preview readiness checklist](docs/v1-preview-readiness-checklist.md)
- [Getting started](docs/getting-started.md)
- [Architecture](docs/architecture.md)
- [Configuration](docs/configuration.md)
- [Production cluster scope](docs/production-cluster-scope.md)
- [Cluster backup, restore, and failure runbook](docs/cluster-backup-restore-runbook.md)
- [PostgreSQL metadata migration path and SQLite-to-PostgreSQL runbook](docs/postgresql-metadata-migration.md)
- [Cluster-safe upload operation semantics](docs/cluster-safe-upload-semantics.md)
- [Resumable upload and upload lease protocol](docs/resumable-upload-lease-protocol.md)
- [Regional stream ingress relay design](docs/regional-stream-ingress-relay.md)
- [Incident capture modes](docs/incident-modes.md)
- [Mode-aware retention policy](docs/mode-aware-retention-policy.md)
- [/v1 access control](docs/v1-access-control.md)
- [Main API public exposure listener split](docs/public-api-listener-split.md)
- [Legacy unowned incident reassignment](docs/legacy-unowned-incident-reassignment.md)
- [Encryption](docs/encryption.md)
- [iOS local recorder prototype](docs/ios-local-recorder-prototype.md)
- [Key custody and emergency access](docs/key-custody.md)
- [Pure post-quantum encryption envelope](docs/post-quantum-envelope.md)
- [Contact key sharing, grants, and wrapped-key metadata](docs/contact-key-sharing-grants.md)
- [Contact-wrapped key metadata simulator prototype](docs/contact-wrapped-key-metadata-simulator.md)
- [Browser-side decryption design](docs/browser-decryption.md)
- [Live partial stream access boundary](docs/live-partial-stream-access-boundary.md)
- [Break-glass key access design](docs/break-glass-key-access.md)
- [API reference](docs/api.md)
- [Deployment notes](docs/deployment.md), including SQLite WAL operations
- [Retention, backup, and deletion](docs/retention-backup-deletion.md)
- [Incident deletion and retention enforcement](docs/incident-deletion-retention-enforcement.md)
- [Security model](docs/security-model.md)
- [Threat model](docs/threat-model.md)
- [Simulator](docs/simulator.md)
- [Development](docs/development.md)
- [Compose smoke tests](compose/README.md)
- [Code map](docs/code-map.md)
- [Technical review reports](docs/reports/README.md)

## AI-Assisted Development

This project has been developed with substantial assistance from OpenAI Codex.

Codex has been used to draft, refactor, test, document, and review parts of the Go backend and Markdown documentation. All accepted changes are reviewed, tested, and committed by the maintainer.

AI assistance does not replace human responsibility. The maintainer remains responsible for:

- code correctness
- security review
- licensing decisions
- release decisions
- deployment choices
- any real-world use of the software

Use of Codex does not imply endorsement, audit, certification, or maintenance by OpenAI.

## Backlog workflow

Use `80-backlog-scan-issue-drafts.md` to generate reviewed local issue drafts under a branch-scoped directory such as `.backlog-drafts/YYYY-MM-DD/<branch-slug>/`.

Review those drafts manually before creating GitHub issues. Drafts are generated review artifacts, not the long-term source of truth once GitHub issues exist.

Only after review, use `85-create-github-issues-from-drafts.md` to generate `scripts/create-backlog-issues.sh` and `.backlog-drafts/.../create-issues-review.md`. Do not run the generated script unless the maintainer explicitly asks for issue creation.

Do not let Codex create GitHub issues directly during the initial scan.

## Security

Viewer links, `/v1` bearer session tokens, and browser session cookies are credentials and should be treated as secrets. Browser cookie mode should use HttpOnly Secure cookies, explicit allowed origins, `credentials: "include"`, and CSRF headers for unsafe requests; browser token persistence should not use localStorage in production. Public deployment still needs TLS, rate limiting, log review, proxy hardening, operational testing, and deployment-specific retention, backup, and deletion enforcement. Do not route `/v1` as an unreviewed public catch-all; expose only explicitly reviewed route groups, and keep `/admin/api/...` and `/admin` off public edges.

Please see [SECURITY.md](SECURITY.md) for supported versions and vulnerability reporting guidance. Do not report security vulnerabilities through public GitHub issues.

## Roadmap

- Create future `open-proofline/web-client`, `open-proofline/ios-client`, `open-proofline/android-client`, and `open-proofline/protocol` repositories when their scopes are ready
- Plan any future protocol or data-layout compatibility migrations separately from the completed repository/module/artifact rename
- Continue hardening optional PostgreSQL metadata support while preserving SQLite local/default support
- Complete the remaining cluster-safe upload operation semantics before multi-node production deployment
- Keep cluster backup, restore, and failure runbooks current as optional PostgreSQL, S3-compatible storage, and coordination behavior evolve
- Keep the regional stream-ingress relay temporary,
  ciphertext-only, and subordinate to the core API for authorization,
  idempotency, durable blob commits, and metadata while metrics, production
  service identity, and deployment hardening are added later. Current relay
  fanout is optimistic, encrypted-only, explicitly unconfirmed until core
  commit succeeds, and followed by bounded confirmation, rejection, or terminal
  failure state when the commit outcome is known. Current relay readiness is
  safe aggregate status only
- WireGuard-only bind/firewall deployment guidance
- Mode-driven access, escalation, retention, sharing, viewer, and key-custody
  behavior after protocol and security design
- Server-side support for trusted-contact dead-man switch workflows after access-control design
- Production key custody, account/device wrapped-key delivery, trusted-contact
  account access, grant-scoped contact delivery, and browser/client-side
  decryption
- Optional break-glass/dead-man-switch key access
- Playable media export
- Reverse-proxy/TLS hardening for incident viewer exposure
- Complete route-by-route [`/v1` public deployment review](docs/v1-access-control.md) before broad public product API deployment

## License

Proofline Server is licensed under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). See [LICENSE](LICENSE).
