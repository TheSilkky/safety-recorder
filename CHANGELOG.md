# Changelog

## Unreleased

- Added configurable account registration modes for disabled, admin-only, open
  self-registration, and paid-placeholder deployments. Open registration is
  disabled by default, requires SMTP-backed email verification, stores
  verification tokens only as hashes, and keeps paid registration fail-closed
  until a future billing system exists.
- Added optional main `/v1` browser cookie-session login/logout support for the
  future web client, including HttpOnly session cookies, session-bound CSRF
  protection for cookie-authenticated unsafe requests, explicit credentialed
  CORS for configured origins, and bearer-token compatibility for existing
  CLI/simulator/API clients.

## v0.10.0 - 2026-06-01

- Ran the review/update stack and applied small behavior-preserving Go
  readability cleanups across simulator, HTTP wrapped-key metadata, and
  storage helper code.
- Added a planning design for a future optional regional stream-ingress relay
  for complete encrypted chunk uploads while keeping the core API authoritative
  for authorization, idempotency, durable blob commits, metadata, and
  ciphertext-only behavior.
- Added private owner-scoped wrapped media-key metadata storage and delivery
  routes bound to active sharing grants, while keeping public viewer and bundle
  manifests key-free and preserving backend ciphertext-only behavior.
- Added owner-scoped contact public-key registration and incident/stream
  sharing-grant metadata routes with SQLite/PostgreSQL parity, while leaving
  trusted-contact accounts, backend decryption, and key custody behavior out of
  scope.
- Added a planning design for future contact key sharing, trusted-contact
  grants, and wrapped-key metadata while preserving ciphertext-only backend
  behavior.
- Added optional Valkey/Redis-compatible short-lived complete-upload
  coordination leases with safe retry hints, while keeping metadata-backed
  upload operations and blob no-overwrite behavior authoritative.
- Kept the private-admin listener dashboard-only under `/admin`, moved existing
  admin JSON APIs onto the main handler with admin-only access, and switched
  bootstrap/smoke flows to the private `/admin/bootstrap` form.
- Moved the read-only incident viewer onto the main listener with authenticated
  `/v1` routes, split private-admin routes onto their own listener,
  and added main/admin listener configuration with legacy private-bind aliases.
- Added configurable main API route-class rate limiting for authentication,
  bootstrap, account, incident, upload, reconciliation, stream, token, download,
  and admin API routes.
- Added a planning document for the future main API/public viewer listener split
  and private admin-dashboard listener boundary.
- Added a mode-aware retention policy design covering future retention inputs
  for incident modes, safety-check states, sharing/export state, grants, wrapped
  keys, tombstones, backups, dry runs, and public viewer fail-closed behavior.
- Added a planning document for future private reassignment or quarantine of
  legacy unowned incidents while preserving the current admin-only default.
- Expanded the SQLite-to-PostgreSQL metadata migration guidance into an
  explicit private operator runbook with copy order, validation, rollback
  limits, and tooling boundaries.
- Added disabled-by-default retention pruning for expired/revoked viewer-token
  metadata and completed deletion tombstones, with SQLite/PostgreSQL parity and
  count-only maintenance summaries.
- Added local read-only operator commands to preview closed-incident retention
  candidates and inspect deletion job status with safe counts and retry
  categories.
- Added explicit-age orphan temp upload cleanup for local `upload-*` staging
  files, with dry-run support and safe count-only startup logs.
- Added opt-in S3-compatible object-store deletion smoke coverage for incident
  deletion, including public viewer fail-closed checks after blob removal.
- Added simulator ambiguous upload retry coverage so desktop-recorder retries
  treat `Idempotency-Replayed: true` responses as successful uploads after
  response loss and keep conflict output token/path safe.
- Added shared SQLite/PostgreSQL upload-operation race and metadata parity tests
  for duplicate uploads, upload-versus-close/completion interleavings,
  idempotency replay/conflict behavior, token revocation, and completed stream
  bundle metadata reconstruction.

## v0.9.0 - 2026-06-01

- Added configurable app-level rate limiting for public incident viewer page,
  JSON polling, encrypted ZIP download, and static asset route classes, using
  safe route-class keys with local in-memory counters by default and
  Valkey/Redis-compatible counters when optional coordination is configured.
- Added private incident deletion and closed-incident retention enforcement,
  including SQLite/PostgreSQL deletion decision metadata, owner-scoped and
  admin-global deletion routes, a retryable background deletion worker, public
  viewer fail-closed behavior for deleting/deleted incidents, safe maintenance
  error logging, and updated retention/security/API documentation.
- Added a durable desktop-recorder simulator mode to `cmd/simclient`, with
  encrypted local staging, restart/resume upload recovery, generated and local
  pre-recorded file sources, optional ffmpeg video segment capture,
  poor-network retry controls, complete-chunk idempotent uploads, bundle
  decrypt verification, encrypted-only bundle output, offline bundle
  verification, and token/path-safe simulator output.
- Added opt-in simulator-only contact-wrapped key metadata artifacts using
  local development contact keys and the maintained `filippo.io/age` wrapping
  library, while keeping backend manifests, routes, storage, and decryption
  behavior unchanged.
- Ignored the desktop-recorder simulator's default stage key filename so local
  simulator keys are not accidentally staged when a stage directory lives under
  the repository.
- Added optional incident-mode, capture-profile, escalation-policy, and
  sharing-state metadata fields to private incident creation and read responses,
  while preserving generic legacy incidents and leaving access, notifications,
  retention, key custody, public viewer behavior, and bundle behavior unchanged.
- Added the private duplicate chunk reconciliation route for comparing expected
  chunk fingerprints with accepted metadata without re-uploading ciphertext or
  exposing stored values.
- Added `Idempotency-Key` support for complete encrypted chunk uploads, with
  hashed key storage in SQLite or PostgreSQL metadata, equivalent retry success,
  conflict handling for key reuse with different upload inputs, simulator
  replay coverage, and updated API/security documentation.
- Added a GitHub Actions job that runs the optional PostgreSQL metadata
  integration tests against a disposable PostgreSQL service.
- Added private-only liveness and readiness checks for coarse metadata, blob,
  and coordination backend status without exposing backend diagnostics on the
  public incident viewer.
- Added a private admin-only HTML surface under `/admin`, using Go
  templates, unauthenticated token-neutral CSS, browser login/bootstrap forms,
  HttpOnly admin-session cookies, a local account list, admin password-change
  and account password-reset workflows, authenticated state-changing form CSRF
  checks, no-store page behavior, and public mux separation.
- Added local username/password accounts for the private `/v1` API, using bcrypt
  password hashes, opaque server-side session tokens stored only as hashes,
  owner/admin incident authorization, admin account management routes, and a
  fail-closed first-admin bootstrap secret flow.

## v0.8.0 - 2026-05-30

- Added local Docker Compose smoke-test stacks for SQLite/local,
  PostgreSQL/local, SQLite/S3-compatible MinIO, and full
  PostgreSQL/S3-compatible MinIO/Valkey backend combinations, with loopback-only
  API port publishing and a script that runs the simulator against the
  containerized server.
- Added Dependabot tracking for local Docker Compose smoke-test image tags.
- Added a live partial stream access boundary design covering future role-scoped
  live access, open/failed stream exposure, partial manifests, no-store
  behavior, and key-custody dependencies without adding routes or decryption.
- Added SQLite WAL operational guidance covering sidecar files, local storage
  expectations, backup and restore handling, and simple checkpoint-pressure
  checks without changing database behavior.
- Added a simulator-only contact-wrapped key metadata prototype design covering
  local model contact keys, non-secret key IDs, wrapped-key metadata shape,
  bundle-manifest relationship, and future server metadata boundaries without
  adding production key custody or backend decryption.
- Added a first-class incident-mode and escalation schema design covering future
  capture profiles, sharing state, migration from generic incidents, viewer
  wording, retention implications, and access-control/key-custody dependencies
  without adding schema or route behavior.
- Documented the current and future-client policy for `original_filename`
  metadata in viewer summaries and bundle manifests.
- Added an incident deletion and retention enforcement design covering future
  private/admin deletion decisions, tombstones, metadata/blob consistency,
  idempotent retry, retention windows, backup interaction, and safe audit
  fields without implementing deletion behavior.
- Added a future `/v1` access-control design covering a public authenticated
  product API, a separately bound private authenticated admin API, and
  account-owner, trusted-contact, public-link, admin/operator, and optional
  escrow access boundaries while preserving the current private
  unauthenticated `/v1` model.
- Added a cluster backup, restore, and failure runbook covering durable
  PostgreSQL metadata, S3-compatible encrypted blobs, coordination-only
  Valkey/Redis state, private restore validation, and conservative failure
  handling.
- Added optional PostgreSQL metadata storage with a separate migration path,
  explicit `SAFE_METADATA_BACKEND=postgresql` configuration, and opt-in
  integration tests while keeping SQLite as the default.
- Added optional Valkey/Redis-compatible coordination configuration and startup
  health checking while keeping no coordination as the default and deferring
  upload leases and idempotency use to future upload-operation work.
- Added optional S3-compatible encrypted blob storage for committed chunks while
  keeping local filesystem storage as the default.
- Added a resumable upload and upload lease protocol plan that defers
  resumable uploads for a local desktop recorder simulator client, preserves
  complete encrypted chunk retry semantics, calls for adjustable poor-network
  simulation and near-term account-aware simulator flows, and defines future
  cleanup and validation boundaries.
- Added a duplicate-chunk reconciliation API design for future clients to
  compare expected ciphertext hashes and immutable metadata without overwriting
  stored evidence.
- Added a cluster-safe upload operation semantics design covering future
  idempotency keys, durable operation state, commit ordering, equivalent retry
  success, conflict handling, cleanup, and backend-specific follow-up work.
- Published trusted Docker images from `develop` pushes using the mutable
  `develop` GHCR image tag, while keeping release binary publishing limited to
  `v*` tag workflows.
- Introduced a narrow metadata repository interface around the existing SQLite
  incident repository implementation.
- Introduced a narrow blob-store interface around the existing local filesystem
  encrypted blob storage implementation.
- Added backend-selection configuration scaffolding for SQLite, PostgreSQL,
  local filesystem, S3-compatible blob storage, no coordination, and optional
  Valkey/Redis-compatible coordination backends.
- Added a PostgreSQL metadata backend migration-path design covering schema
  parity, migrations, transaction boundaries, tests, and restore expectations.
- Added CI runtime smoke tests for the built Linux binary and Docker image.
- Added a public incident viewer deployment checklist covering public route
  exposure, TLS/HSTS, edge rate limiting, proxy log redaction, viewer-token
  review, and retention/restore expectations.
- Sanitized internal filesystem error logging

## v0.7.0 - 2026-05-28

- Moved the Go module and backend source tree to the repository root as
  `github.com/open-proofline/server`, and normalized new module, Docker, GHCR,
  and release binary artifact references after the `open-proofline/server`
  transfer.
- Updated CI, Docker, development, deployment, prompt, and report-workflow
  references for the repository-root server layout and `proofline-server-*`
  release artifacts.
- Updated the GitHub Actions `download-artifact` dependency while preserving
  full-SHA action pinning.
- Fixed the README Go version badge after the root-module migration.

## v0.6.1 - 2026-05-28

- Updated repository, GHCR badge, and prompt references after the
  `open-proofline/server` transfer.
- Targeted Dependabot updates to the `develop` integration branch for the
  post-release branch model.

## v0.6.0 - 2026-05-27

- Added CI vulnerability and coverage signals for release review, with release
  publishing gated on the vulnerability scan and coverage kept advisory.
- Hardened private API and public token-path security headers for unsupported
  method/error responses.
- Renamed legacy viewer/token terminology to incident-viewer and incident-token terminology, including breaking route/config/schema names for the upcoming release while migrating existing token rows.
- Retained legacy `/e/{token}` public viewer route aliases for already shared pre-rename links.
- Renamed the product in documentation to Proofline while preserving current repository, module, Docker, GHCR, route, and compatibility names.
- Updated active issue templates and reusable Codex prompts to match the
  Proofline product name.
- Documented the planned `open-proofline` multi-repo layout and clarified that this repository is the Go server backend only.
- Documented the broader incident-capture direction, including emergency incidents, non-emergency interaction records, timed safety checks, and evidence notes.
- Added `Phase 0` Deep Research prompt. Loads report instructions and plans research prior to running `Phase 1`
- Documented Go readability standards and aligned the readability-maintenance Codex prompt with them.
- Refactored `server/cmd/api` server lifecycle helpers into a focused file without changing startup or listener behaviour.
- Refactored `server/cmd/simclient` simulator flow helpers into a focused file without changing CLI behaviour.
- Refactored `server/internal/config` bind-address, byte-size, timeout, and environment fallback parsing into focused files without changing configuration behaviour.
- Refactored `server/internal/db` connection, migration orchestration, and compatibility migration helpers into focused files for readability without changing migration behaviour.
- Refactored `server/internal/envelope` key-file, associated-data, chunk encryption, and header parsing helpers into focused files without changing the envelope format.
- Refactored `server/internal/httpapi` summary, bundle, stream-validation, and upload parsing helpers for readability without changing HTTP behaviour.
- Refactored `server/internal/incidents` repository methods into focused chunk, checkin, and incident-token files for readability without changing behaviour.
- Refactored `server/internal/storage` temp upload and immutable blob helpers into focused files for readability without changing storage behaviour.
- Documented the `develop` and `release/v*` repository rulesets, branch model,
  and PR base-branch guidance.

## v0.5.0 - 2026-05-26

- Automated creating a minimal GitHub Release when needed and uploading the Linux amd64 binary as a Release asset for `v*` tag workflows.
- Added release binary and GHCR image artifact attestations to the CI workflow.
- Verified SQLite WAL startup by checking the returned journal mode and failing when WAL cannot be enabled.
- Aligned Docker base-image digest refresh documentation with the runtime Alpine tag family used by the Dockerfile.
- Pinned Docker base images by digest, added Dependabot Docker monitoring, and documented base-image digest refresh review steps.
- Broadened the Docker build-context ignore policy for local-only artifacts under `server/`.
- Pinned GitHub Actions workflow dependencies to full commit SHAs and documented the review process for action updates.
- Added an iOS local recorder prototype plan covering chunking, encrypted staging, retry behavior, and current stream API mapping.
- Added a retention, backup, restore, and secure deletion policy design document.
- Added deployment-edge rate-limiting guidance and Traefik route-group examples.
- Added deployment examples for localhost-only Docker, WireGuard/private-network `/v1` access, and Traefik HTTPS incident viewer exposure.
- Added a configurable default 24-hour incident-token expiry for omitted `expires_at` values.
- Added a public technical review report and report-validation prompt workflow.
