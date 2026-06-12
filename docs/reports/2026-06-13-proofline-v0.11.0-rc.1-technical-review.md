# Technical Review of Proofline v0.11.0-rc.1

Review date: 2026-06-13
Reviewed branch/ref: `v0.11.0-rc.1`
Reviewed commit SHA: `4644ec7648e9718a5cb31a148c10d67497bf468d`
Target report path: `docs/reports/2026-06-13-proofline-v0.11.0-rc.1-technical-review.md`
Maintainer-supplied model/tool disclosure: `OpenAI GPT-5.5 Pro-Extended`
Phase 2 validation: Codex validated this report for public-safety wording, portable citations, reviewed-SHA pinning, and report-publication hygiene on 2026-06-13.

This is a technical review report. It is not a formal security audit, penetration test, compliance certification, legal review, App Store review, Play Store review, production-readiness endorsement, or release approval. [R-REPORT-SCOPE] [V-PHASE2-CODEX]

## Executive Summary

The reviewed tree presents Proofline v0.11.0-rc.1 as an experimental Go backend for private encrypted incident capture. The current implementation is server-side only: it accepts already-encrypted uploads, stores metadata in SQLite by default or optional PostgreSQL, stores encrypted blobs locally by default or in optional S3-compatible object storage, supports optional Valkey/Redis-compatible short-lived coordination, and exposes a token-scoped read-only incident viewer. The documentation consistently warns that the backend is not production-ready, is not a public product API, and must not be exposed as public infrastructure without further route, TLS, proxy, browser, logging, and operational review. [R-README] [R-SECURITY] [R-REPORT-SCOPE]

No Critical or High findings were identified from this source-and-documentation review. The review found three Low-severity, non-blocking issues or hardening opportunities:

- F-001: the PostgreSQL CI service image is tag-based rather than digest-pinned.
- F-002: completed ZIP bundle generation preflights blob existence but does not verify chunk hash and length before serving bundle bytes.
- F-003: rollback deletion after blob commit but metadata failure ignores cleanup errors, making orphan encrypted blobs harder to detect.

The reviewed code shows meaningful alignment with the documented boundaries: main API and private-admin route trees are constructed separately, admin API routes are on the admin handler, viewer routes are read-only and token-scoped, request logging uses route-pattern redaction for token-bearing paths, viewer tokens are generated with 256-bit random material and stored as hashes, upload handlers hash and validate uploaded ciphertext, local and S3 blob paths are server-controlled, S3 writes use conditional no-overwrite semantics, PostgreSQL migrations use checksums and an advisory lock, and Valkey coordination uses short-lived lease semantics with token-matched release. [R-CMD-API] [R-HTTPAPI-ROUTES] [R-HTTPAPI-MIDDLEWARE] [R-INCIDENT-TOKENS] [R-HTTPAPI-UPLOAD] [R-STORAGE-PATHS] [R-STORAGE-S3] [R-POSTGRES-MIGRATE] [R-COORDINATION] [R-VALKEY]

Several potential false positives were treated as non-findings. Missing iOS, Android, browser decryption, public account portal, key escrow, break-glass, emergency-services integration, playable export, and first-class incident-mode behavior are documented as future or out-of-scope unless implementation exists. The reviewed planning documents describe future direction while repeatedly preserving the current ciphertext-only backend boundary. [R-INCIDENT-MODES] [R-KEY-CUSTODY] [R-BROWSER-DECRYPTION] [R-BREAK-GLASS]

No application tests, container builds, simulator smoke tests, live PostgreSQL/S3/Valkey disposable-service tests, or GitHub repository settings inspections were executed by this report. Repository workflow definitions and source files were reviewed, but no run URLs or command transcripts were supplied as validation evidence for those runtime paths. Phase 2 validation was limited to report cleanup, source re-checks for the three findings, citation integrity, markdown links, and diff hygiene. [R-CI] [V-NOT-EXECUTED] [V-PHASE2-CODEX]

## Source Registry

### Repository sources inspected

| Source ID | Source type | Location | Commit/ref/date | Review purpose | Status | Limitations | Related findings / sections |
|---|---|---|---|---|---|---|---|
| R-REPORT-SCOPE | Maintainer-supplied review scope and Codex report-validation prompt | Conversation-supplied reviewed ref, SHA, target version, report path, model/tool disclosure, and Phase 2 instruction | 2026-06-13 | Governing review scope, validation, citation, source, safety, and false-positive rules | Inspected | Scope inputs and prompts are not runtime validation evidence | Scope, method, all findings |
| R-COMMIT | Repository commit | Commit `4644ec7648e9718a5cb31a148c10d67497bf468d` | 2026-06-13 review input | Reviewed SHA and release context | Inspected | Commit metadata is not equivalent to test execution | Scope and method |
| R-README | Repository documentation | `README.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Product scope, current features, warnings, listener boundaries, Docker guidance, roadmap | Inspected | Documentation claims were cross-checked against representative source files, not every route branch | Executive summary, non-findings |
| R-SECURITY | Repository documentation | `SECURITY.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Security posture, supported version, disclosure, public-safety boundaries | Inspected | No private advisory state inspected | Public-safety restrictions, non-findings |
| R-CHANGELOG | Repository documentation | `CHANGELOG.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Release notes and v0.11.0-rc.1 scope | Inspected | Release notes are claims, not test evidence | Executive summary, current implementation |
| R-AGENTS | Repository instructions | `AGENTS.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Maintainer expectations and safety constraints | Inspected | Used as repo-specific review policy, not runtime evidence | Scope and method |
| R-DOCS-README | Repository documentation | `docs/README.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Documentation map, optional backend scope, future-planning boundaries | Inspected | Index page, not exhaustive implementation proof | Source map, non-findings |
| R-ARCHITECTURE | Repository documentation | `docs/architecture.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Architecture, listener topology, future-client boundaries | Inspected | Architecture docs were checked against representative routing and startup code | Route separation, non-findings |
| R-CODE-MAP | Repository documentation | `docs/code-map.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Package map, upload flow, viewer flow, storage behavior, admin boundary | Inspected | Source code remains authoritative for implementation details | Current implementation, findings |
| R-CONFIGURATION | Repository documentation | `docs/configuration.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Runtime config, backend selection, listener binding, rate limits, session settings | Inspected | Configuration paths were sampled in code, not every parser branch | Current implementation |
| R-CI | Repository workflow | `.github/workflows/ci.yml` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | CI jobs, PostgreSQL service image, action pinning, Docker smoke definitions, attestations | Inspected | Workflow definitions only; no run logs or job status independently verified | F-001, validation evidence |
| R-DEPENDABOT | Repository workflow config | `.github/dependabot.yml` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Dependency update grouping and cadence | Inspected | Dependabot config does not prove updates were applied | Supply-chain context |
| R-DOCKERFILE | Repository build file | `Dockerfile` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Server image build, digest-pinned bases, non-root runtime | Inspected | Docker build not executed | F-001 context |
| R-INGRESS-DOCKERFILE | Repository build file | `Dockerfile.ingress` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Relay image build, digest-pinned bases, non-root runtime | Inspected | Docker build not executed | Non-findings |
| R-DOCKERIGNORE | Repository build file | `.dockerignore` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Build-context exclusions for local data, secrets, keys, zips, scripts, drafts | Inspected | Does not prove local users avoid unsafe build contexts outside this file | Supply-chain context |
| R-GOMOD | Repository dependency manifest | `go.mod` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Go version and exact package dependencies for PostgreSQL, S3, Valkey, WebAuthn, crypto | Inspected | Vulnerability scan not executed | Source registry, optional backend review |
| R-CMD-API | Go source | `cmd/api/main.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Startup, backend selection, handler construction, rate limiter selection, cleanup | Inspected | Startup not executed | Current implementation |
| R-HTTPAPI-API | Go source | `internal/httpapi/api.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Handler constructors, defaults, web auth and rate-limit options | Inspected | Constructor behavior reviewed statically | Current implementation |
| R-HTTPAPI-ROUTES | Go source | `internal/httpapi/routes.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Main, admin, public viewer, and relay route maps | Inspected | Route registration reviewed statically, not exercised by HTTP smoke tests | Non-findings |
| R-HTTPAPI-MIDDLEWARE | Go source | `internal/httpapi/middleware.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Logging redaction, no-store headers, recovery behavior | Inspected | Runtime log output not captured | Non-findings |
| R-HTTPAPI-UPLOAD | Go source | `internal/httpapi/upload.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Multipart limits, field parsing, filename cleanup, SHA input validation | Inspected | Upload handler not executed | F-002 context |
| R-HTTPAPI-CHUNK | Go source | `internal/httpapi/chunk_handlers.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Upload transaction, idempotency, stream validation, envelope validation, rollback cleanup | Inspected | Concurrency and backend behavior not live-tested | F-002, F-003 |
| R-HTTPAPI-VIEWER | Go source | `internal/httpapi/incident_viewer.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Viewer token creation, metadata responses, read-only viewer payloads, privacy headers | Inspected | Viewer not browser-tested | Non-findings |
| R-HTTPAPI-BUNDLE-ZIP | Go source | `internal/httpapi/bundle_zip.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | ZIP entry names, manifest writing, chunk copy behavior | Inspected | ZIP output not generated by this report | F-002 |
| R-HTTPAPI-BUNDLES | Go source | `internal/httpapi/bundles.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Private and viewer bundle routes, completed-stream checks, bundle preflight | Inspected | Bundle routes not exercised | F-002 |
| R-HTTPAPI-BUNDLE-MANIFEST | Go source | `internal/httpapi/bundle_manifest.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Manifest SHA/size fields and encryption hints | Inspected | Manifest semantics reviewed statically | F-002 |
| R-STORAGE-CORE | Go source | `internal/storage/storage.go` and `internal/storage/blob_store.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | BlobStore interface, local storage creation, storage error types | Inspected | Storage operations not executed | F-002, F-003 |
| R-STORAGE-TEMP | Go source | `internal/storage/temp_upload.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Temp upload streaming, size limit, temp quota, SHA computation | Inspected | No large-file or quota smoke test executed | F-002 |
| R-STORAGE-PATHS | Go source | `internal/storage/paths.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Server-controlled stored path generation and cleaning | Inspected | Static review only | Non-findings |
| R-STORAGE-COMMITTED | Go source | `internal/storage/committed_blobs.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Local immutable commit, open, remove behavior | Inspected | Filesystem behavior not tested | F-003 |
| R-STORAGE-S3 | Go source | `internal/storage/s3.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | S3-compatible configuration, object-key cleaning, conditional writes, open/remove | Inspected | No live S3-compatible service test executed | F-002, F-003 |
| R-POSTGRES-DB | Go source | `internal/postgresdb/db.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | PostgreSQL connection setup and migration call | Inspected | PostgreSQL integration tests not executed | Current implementation |
| R-POSTGRES-MIGRATE | Go source | `internal/postgresdb/migrate.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Migration ordering, checksums, transactions, advisory lock | Inspected | Live migration execution not verified | Non-findings |
| R-POSTGRES-CHUNKS | Go source | `internal/postgresdb/chunks.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Chunk transaction, stream validation, uniqueness, quota locking | Inspected | PostgreSQL transaction behavior not live-tested | Non-findings |
| R-POSTGRES-MIGRATION-EMBED | Go source | `migrations/postgres/migrations.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Embedded PostgreSQL migrations | Inspected | File list was sampled, not exhaustively diffed here | Non-findings |
| R-POSTGRES-INIT-SQL | SQL migration | `migrations/postgres/001_init.sql` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Core PostgreSQL schema constraints and indexes | Inspected | Later migrations were not exhaustively quoted in this report | Non-findings |
| R-SQLITE-DB | Go source | `internal/db/db.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | SQLite open, foreign-key pragma, WAL mode, single connection | Inspected | SQLite commands not executed | Non-findings |
| R-COORDINATION | Go source | `internal/coordination/coordinator.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Coordination boundary, no-op default, generic errors, lease token model | Inspected | Valkey behavior not live-tested | Non-findings |
| R-VALKEY | Go source | `internal/coordination/valkey.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | go-redis client configuration, TLS, Lua scripts, SetNX lease, token-matched delete | Inspected | go-redis package page was not available through source consultation | Non-findings |
| R-INCIDENT-TOKENS | Go source | `internal/incidents/incident_tokens.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Viewer token generation, hash storage, lookup, expiry, revocation | Inspected | Token flows not live-tested | Non-findings |
| R-INCIDENT-IDS | Go source | `internal/incidents/ids.go` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Token hashing and random ID generation helpers | Inspected | Randomness not statistically tested | Non-findings |
| R-INCIDENT-MODES | Repository planning doc | `docs/incident-modes.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Mode metadata and future escalation boundaries | Inspected | Planning doc only | Non-findings |
| R-KEY-CUSTODY | Repository planning doc | `docs/key-custody.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Current ciphertext-only and future key-custody boundaries | Inspected | Planning doc only | Non-findings |
| R-BROWSER-DECRYPTION | Repository planning doc | `docs/browser-decryption.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Browser decryption design spike and current viewer behavior | Inspected | Planning doc only | Non-findings |
| R-BREAK-GLASS | Repository planning doc | `docs/break-glass-key-access.md` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Future break-glass and dead-man-switch boundaries | Inspected | Planning doc only | Non-findings |

### External authoritative sources consulted

| Source ID | Source type | Location | Date consulted | Review purpose | Status | Limitations | Related findings / sections |
|---|---|---|---|---|---|---|---|
| S-GITHUB-ACTIONS-PINNING | GitHub documentation | GitHub Actions security hardening documentation on third-party action pinning | 2026-06-13 | Confirm full-length SHA action pinning guidance | Consulted | Does not evaluate this repository's private settings or run history | F-001, non-findings |
| S-DOCKER-DIGESTS | Docker documentation | Docker image pull documentation on digest-pinned immutable identifiers | 2026-06-13 | Confirm digest-pinning behavior and tag drift risk | Consulted | Does not validate the reviewed images build or run | F-001 |
| S-AWS-S3-PUTOBJECT | AWS S3 API documentation | `PutObject` API reference | 2026-06-13 | Confirm `If-None-Match` no-overwrite semantics and checksum behavior | Consulted | AWS S3 semantics; S3-compatible providers may differ | F-002, non-findings |
| S-AWS-S3-GETOBJECT | AWS S3 API documentation | `GetObject` API reference | 2026-06-13 | Confirm object retrieval and checksum-mode response fields | Consulted | AWS S3 semantics; S3-compatible providers may differ | F-002 |
| S-AWS-S3-OBJECT-KEYS | AWS S3 User Guide | Object key naming guidelines | 2026-06-13 | Review object-key safety and path-like segment concerns | Consulted | AWS guidance; local filesystem behavior still repo-specific | Non-findings |
| S-AWS-S3-CHECKSUMS | AWS S3 User Guide | Object integrity and checksum documentation | 2026-06-13 | Review object integrity verification options | Consulted | Does not prove reviewed implementation enabled S3 checksum mode | F-002 |
| S-AWS-S3-DELETEOBJECT | AWS S3 API documentation | `DeleteObject` API reference | 2026-06-13 | Review delete semantics and versioning caveats for rollback cleanup | Consulted | AWS S3 semantics; S3-compatible providers may differ | F-003 |
| S-POSTGRES-TRANSACTIONS | PostgreSQL documentation | Transaction tutorial | 2026-06-13 | Ground transaction/commit/rollback claims | Consulted | Does not execute repository migrations | Non-findings |
| S-POSTGRES-CONSTRAINTS | PostgreSQL documentation | Constraints documentation | 2026-06-13 | Ground uniqueness, foreign-key, and constraint claims | Consulted | Does not validate repository SQL against a live database | Non-findings |
| S-SQLITE-FOREIGN-KEYS | SQLite documentation | Foreign key support documentation | 2026-06-13 | Confirm application must enable SQLite foreign keys | Consulted | Does not execute SQLite open path | Non-findings |
| S-SQLITE-WAL | SQLite documentation | WAL documentation | 2026-06-13 | Ground WAL concurrency and operational caveat claims | Consulted | Does not benchmark or exercise SQLite under load | Non-findings |
| S-VALKEY-SET | Valkey command documentation | `SET` command documentation | 2026-06-13 | Confirm `NX` and expiry options and token-matched lock pattern | Consulted | Does not validate go-redis client behavior | Non-findings |
| S-VALKEY-EXPIRE | Valkey command documentation | `EXPIRE` command documentation | 2026-06-13 | Confirm TTL and expiration semantics | Consulted | Does not test repository TTL values | Non-findings |
| S-REDIS-PROTOCOL | Redis documentation | Redis serialization protocol specification | 2026-06-13 | Reference Redis-compatible protocol context | Consulted | Not used for a finding | Source registry |
| S-OWASP-LOGGING | OWASP Cheat Sheet Series | Logging Cheat Sheet | 2026-06-13 | Ground sensitive logging exclusions | Consulted | General security guidance, not repository-specific validation | Non-findings, F-003 logging guidance |
| S-GO-POSTGRES-CLIENT | Go package documentation | `pkg.go.dev/github.com/jackc/pgx/v5` and `stdlib` package family | 2026-06-13 | Exact Go PostgreSQL package family used by repository | Consulted | Package docs were not used to infer behavior beyond package identity | Source registry |
| S-GO-S3-CLIENT | Go package documentation | `pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3` | 2026-06-13 | Exact Go S3 client package used by repository | Consulted | Package docs were not used as a substitute for AWS S3 API semantics | Source registry |
| S-GO-VALKEY-CLIENT | Go package documentation | `pkg.go.dev/github.com/redis/go-redis/v9` | 2026-06-13 | Exact Go Valkey/Redis client package used by repository | Attempted but not available in this review environment | Package behavior claims were not based on this unavailable page | Sources not available |

### Validation and execution evidence

| Source ID | Source type | Location | Commit/ref/date | Review purpose | Status | Limitations | Related findings / sections |
|---|---|---|---|---|---|---|---|
| V-REVIEW-INPUT | Maintainer-supplied review input | Conversation-supplied branch/ref, SHA, version, date, prompt path, report path, model disclosure | 2026-06-13 | Establish reviewed target and report metadata | Available | Input values were accepted as review scope; no independent git checkout was performed | Scope and method |
| V-CI-WORKFLOW-DEFINITION | Repository workflow definition | `.github/workflows/ci.yml` | `4644ec7648e9718a5cb31a148c10d67497bf468d` | Identify declared CI jobs and smoke checks | Available as source definition only | No run URLs, logs, statuses, or artifacts were supplied or independently verified | F-001, validation limitations |
| V-PHASE2-CODEX | Codex Phase 2 report-validation pass | Current checkout on `develop` at `4644ec7648e9718a5cb31a148c10d67497bf468d` | 2026-06-13 | Validate report metadata, source-backed findings, public-safety wording, citation integrity, markdown links, and diff hygiene | Available | Report/docs validation only; no application tests, containers, simulator runs, or live optional-backend smoke tests were executed | Scope, method, report publication |

### Sources, checks, and commands not available or not executed

| Source ID | Source/check | Status | Limitation |
|---|---|---|---|
| V-NOT-EXECUTED | Application tests, Docker builds, containers, simulator smoke tests, and live optional-backend smoke tests | Not executed by this report | The review must not claim runtime validation for commands, containers, builds, optional backends, or simulator flows without supplied evidence. |
| V-GOFMT | `gofmt -w ./cmd ./internal ./migrations` | Not supplied / not executed | Formatting status was not independently verified. |
| V-GO-TEST | `go test ./...` | Not supplied / not executed | Test status was not independently verified. |
| V-GO-VET | `go vet ./...` | Not supplied / not executed | Vet status was not independently verified. |
| V-DOCKER-BUILD | `docker build -t proofline-server .` | Not supplied / not executed | Server image build was not independently verified. |
| V-COMPOSE-SMOKE | Compose smoke tests | Not supplied / not executed | Compose, listener topology, and optional backend smoke tests were not independently executed. |
| V-POSTGRES-LIVE | PostgreSQL metadata integration test or disposable-service run | Not supplied / not executed | PostgreSQL path was reviewed from code/docs/workflow definitions only. |
| V-S3-LIVE | S3-compatible storage test or disposable object-store smoke test | Not supplied / not executed | S3 path was reviewed from code/docs only. |
| V-VALKEY-LIVE | Valkey/Redis-compatible coordination startup/TTL smoke test | Not supplied / not executed | Valkey path was reviewed from code/docs only. |
| V-SIMCLIENT | Simulator smoke test | Not supplied / not executed | Simulator behavior was not independently verified. |
| V-GITHUB-SETTINGS | Live GitHub repository settings | Not inspected | Workflow files were inspected; repository settings and branch protection were not. |
| S-GO-VALKEY-CLIENT | `pkg.go.dev/github.com/redis/go-redis/v9` exact package page | Attempted but unavailable | Client-library behavior was not independently verified from `pkg.go.dev`; repository code and Valkey command docs were used only for higher-level coordination semantics. |

### Generated artifacts and report outputs

| Artifact ID | Type | Location | Status | Limitations |
|---|---|---|---|---|
| G-REPORT-PUBLISHED | Markdown report | `docs/reports/2026-06-13-proofline-v0.11.0-rc.1-technical-review.md` | Generated | Public report produced after Phase 2 cleanup; runtime validation limits remain explicit. |
| G-ISSUE-DRAFTS | Local issue drafts | `.backlog-drafts/2026-06-13/develop/` | Generated locally | Drafts only; not public GitHub issues unless the maintainer explicitly requests creation. |

## Scope And Method

This review inspected repository files at commit `4644ec7648e9718a5cb31a148c10d67497bf468d`, with emphasis on the maintainer-supplied report scope, README, SECURITY, CHANGELOG, AGENTS, documentation under `docs/`, Codex prompt context, GitHub Actions workflows, Dockerfiles, `cmd/`, `internal/`, `migrations/`, storage backends, coordination code, and public-safety boundaries. [R-REPORT-SCOPE] [R-README] [R-SECURITY] [R-AGENTS]

The method was source and documentation review plus consultation of authoritative external documentation for claims about GitHub Actions, Docker image digests, S3 object semantics, PostgreSQL transactions and constraints, SQLite foreign keys and WAL mode, Valkey TTL and lock patterns, Redis-compatible protocol context, OWASP logging guidance, and relevant Go package families. [S-GITHUB-ACTIONS-PINNING] [S-DOCKER-DIGESTS] [S-AWS-S3-PUTOBJECT] [S-POSTGRES-TRANSACTIONS] [S-SQLITE-FOREIGN-KEYS] [S-VALKEY-SET] [S-OWASP-LOGGING]

This review did not run repository application tests, containers, Docker builds, Compose smoke tests, live GitHub repository-setting checks, live PostgreSQL/S3/Valkey disposable-service tests, or simulator smoke tests. Any statement about CI, build, or smoke behavior is limited to workflow definitions unless explicit validation evidence is supplied later. Phase 2 Codex validation rechecked the three findings against source, removed pre-publication wording, kept citations pinned to the reviewed SHA, and ran report-publication hygiene checks. [R-CI] [V-NOT-EXECUTED] [V-PHASE2-CODEX]

Findings are scoped to the reviewed branch/ref and commit. Follow-up recommendations distinguish release-blocking concerns from non-blocking hardening and future-planning work. Sensitive material was intentionally omitted.

## Current Implementation Summary

Proofline v0.11.0-rc.1 is documented as an experimental backend, not production-ready public infrastructure. The repository documentation states that current code supports a Go backend, encrypted upload storage, a local-account main API, a private admin dashboard, a token-scoped read-only viewer, optional PostgreSQL metadata, optional S3-compatible blob storage, optional Valkey/Redis-compatible coordination, and stream-ingress relay support. [R-README] [R-CHANGELOG] [R-DOCS-README]

Startup loads configuration, initializes the configured coordination backend, opens the configured metadata repository, performs auth bootstrap checks, opens the configured blob store, runs temp upload cleanup when configured, constructs separate main and admin handlers, starts the deletion worker, and starts HTTP servers. The main handler and admin handler are built separately through `httpapi.NewMain` and `httpapi.NewAdmin`. [R-CMD-API] [R-HTTPAPI-API]

The route map keeps main `/v1` routes, relay routes, sharing/wrapped-key routes, and public viewer routes under the main handler, while `/admin/api/...` and `/admin` web routes are registered on the admin handler. The CI stream-ingress smoke job also checks that admin, main API, viewer, and metrics paths return 404 on the relay image. This supports the documented route-boundary intent, while still requiring deployment-level public-edge review before any public exposure. [R-HTTPAPI-ROUTES] [R-CI] [R-ARCHITECTURE]

Request logging records method, redacted route path, status, byte count, and duration, while omitting bodies, upload bytes, Authorization headers, and token-like values. Viewer token paths and compatibility `/e/...` paths are redacted to route patterns, and admin/main API paths are also reduced to safe templates. OWASP logging guidance identifies session identifiers, access tokens, sensitive personal data, passwords, database connection strings, encryption keys, and primary secrets as data that should usually not be recorded directly in logs. [R-HTTPAPI-MIDDLEWARE] [S-OWASP-LOGGING]

Upload handling streams multipart file bytes into temporary storage, enforces request and file-size limits, computes a SHA-256 hash over exactly accepted bytes, compares that hash with the caller-provided lowercase SHA-256 hex value, validates stream identity and envelope identity, acquires an upload coordination lease when configured, commits the blob to server-controlled storage, then inserts chunk metadata. Metadata failures after blob commit trigger rollback deletion attempts. [R-HTTPAPI-UPLOAD] [R-HTTPAPI-CHUNK] [R-STORAGE-TEMP]

Local blob storage creates data and temp directories with owner-only permissions, stages uploads under a temp directory, and commits local blobs with hard-link no-overwrite behavior. Stored paths are generated by the server from validated incident, stream, media type, and chunk index fields, and path cleaning rejects empty, absolute, backslash-containing, dot, dot-dot, and slash-containing segments. [R-STORAGE-CORE] [R-STORAGE-PATHS] [R-STORAGE-COMMITTED]

The S3-compatible blob store requires explicit endpoint, region, bucket, and static credentials; stages uploads locally; checks local temp storage and bucket reachability; constructs cleaned object keys from server-controlled stored paths; writes final objects using `If-None-Match: *`; maps precondition/conflict responses to duplicate-blob errors; and retrieves or deletes objects by cleaned stored path. AWS S3 documents `If-None-Match` for conditional `PutObject` writes and documents object key naming considerations, though S3-compatible providers may differ. [R-STORAGE-S3] [S-AWS-S3-PUTOBJECT] [S-AWS-S3-OBJECT-KEYS]

SQLite opens with a single connection, enables foreign keys, enables WAL mode, and applies migrations. SQLite documentation confirms that foreign-key enforcement must be enabled by the application and that WAL mode supports concurrent readers and a writer, subject to normal locking and checkpoint caveats. [R-SQLITE-DB] [S-SQLITE-FOREIGN-KEYS] [S-SQLITE-WAL]

PostgreSQL opens through the `pgx` database/sql driver, configures pool limits, pings the database, applies embedded PostgreSQL migrations, records migration checksums, uses an advisory lock around migration application, and applies each migration in a transaction. PostgreSQL documentation grounds the all-or-nothing transaction model and constraint behavior used by the repository's schema and code. [R-POSTGRES-DB] [R-POSTGRES-MIGRATE] [R-POSTGRES-MIGRATION-EMBED] [R-POSTGRES-INIT-SQL] [S-POSTGRES-TRANSACTIONS] [S-POSTGRES-CONSTRAINTS]

Valkey/Redis-compatible coordination is optional. The default no-op coordinator always succeeds. The Valkey implementation uses go-redis, supports TLS with a minimum TLS 1.2 configuration when enabled, pings on startup, implements fixed-window rate-limit counters with expiry, acquires upload leases with `SetNX` and expiry, and releases leases only when a stored random token matches. Valkey's `SET` documentation describes `NX` and expiry options, and its lock-pattern guidance recommends non-guessable tokens and token-matched unlock scripts. [R-COORDINATION] [R-VALKEY] [S-VALKEY-SET] [S-VALKEY-EXPIRE]

Viewer tokens are 256-bit random URL-safe bearer tokens returned once on creation, while the repository stores only SHA-256 token hashes. Lookup hashes the raw token, joins against active incidents, performs a constant-time hash equality check, and rejects revoked or expired tokens. Viewer routes set privacy headers, collapse invalid/expired/revoked token states into a generic error, and summarize metadata without stored paths or encrypted file bytes. [R-HTTPAPI-VIEWER] [R-INCIDENT-TOKENS] [R-INCIDENT-IDS]

ZIP bundle generation is available for completed streams and incidents. It requires stream completion, non-empty contiguous stream chunks, matching media type, and existing chunk files before serving. Bundle manifests include chunk indexes, media type, byte size, SHA-256 hashes, timestamps, and encryption hints, but the server does not decrypt media and bundle output remains encrypted chunks plus JSON manifests. [R-HTTPAPI-BUNDLES] [R-HTTPAPI-BUNDLE-ZIP] [R-HTTPAPI-BUNDLE-MANIFEST]

The planning documents consistently preserve the current implementation boundary: incident-mode fields are labels unless future behavior is explicitly implemented; browser decryption is a design spike only; key custody remains future direction; break-glass/dead-man-switch work is not implemented; and Proofline does not currently contact emergency services. [R-INCIDENT-MODES] [R-BROWSER-DECRYPTION] [R-KEY-CUSTODY] [R-BREAK-GLASS]

## Findings

### F-001: PostgreSQL CI service image is tag-based rather than digest-pinned

Severity: Low
Confidence: High
Current implementation vs future design: Current CI/supply-chain hygiene
Branch scope: Reviewed branch/ref `v0.11.0-rc.1`, commit `4644ec7648e9718a5cb31a148c10d67497bf468d`
Release impact: Non-blocking hardening unless the release requires fully reproducible CI service images
Suggested public issue title: `Pin PostgreSQL CI service image by digest or document tag-drift exception`
Priority: Low
Type: CI / supply-chain hygiene
Labels: `ci`, `github_actions`, `docker`, `maintenance`
Sensitive handling: Safe for public issue; do not include private CI logs or credentials

The CI workflow pins third-party GitHub Actions by full commit SHA and the server and relay Dockerfiles use digest-pinned base images. That is a strong pattern. The PostgreSQL integration test job, however, uses the service image `postgres:18-alpine` by tag. Docker documents that pulling by digest pins an exact image version, while tags are convenient identifiers that can point to updated image contents over time. GitHub Actions documentation also recommends pinning third-party actions to full-length commit SHAs because a commit SHA is immutable. [R-CI] [R-DOCKERFILE] [R-INGRESS-DOCKERFILE] [S-DOCKER-DIGESTS] [S-GITHUB-ACTIONS-PINNING]

Why it matters: the reviewed commit can be tested against different PostgreSQL service image contents over time if the tag is updated upstream. This is not a runtime vulnerability and does not imply PostgreSQL support is broken. It is a reproducibility and reviewability gap for a release candidate that otherwise shows careful pinning discipline.

Minimal actionable fix: pin the PostgreSQL service image by digest in `.github/workflows/ci.yml`, or add a short comment documenting why this service image intentionally follows the moving tag and how updates are reviewed. If digest pinning is adopted, align Dependabot or a documented update process with that decision.

Acceptance criteria:

1. The PostgreSQL CI service image is pinned with a digest, or the workflow contains an explicit reviewed exception.
2. The update cadence for that image is documented or handled by dependency automation.
3. PostgreSQL integration tests still run in CI for the reviewed branch.
4. No DSNs, credentials, or database logs containing sensitive data are included in public issue text.

Validation limits: this report inspected the workflow definition only. It did not run the PostgreSQL integration job and did not inspect GitHub Actions run history. [V-NOT-EXECUTED]

### F-002: Bundle generation preflights blob existence but does not verify chunk hash and length before serving ZIP bytes

Severity: Low
Confidence: High
Current implementation vs future design: Current implementation
Branch scope: Reviewed branch/ref `v0.11.0-rc.1`, commit `4644ec7648e9718a5cb31a148c10d67497bf468d`
Release impact: Non-blocking hardening unless the release intends to claim server-side stored-blob integrity verification on download
Suggested public issue title: `Verify stored chunk hash and size before serving completed evidence bundles`
Priority: Low
Type: Data integrity / evidence-bundle hardening
Labels: `backlog`, `maintenance`, `testing`, `security`
Sensitive handling: Safe for public issue if described generically; do not include real incident IDs, stored paths, object keys, uploaded bytes, or ciphertext samples

Upload code computes a SHA-256 hash while streaming accepted bytes into temporary storage, compares it to the caller-provided hash, then stores the accepted hash in metadata. Bundle manifests include each chunk's byte size and SHA-256 hash. Completed bundle generation checks that the stream is complete, that chunks are contiguous and media-type-consistent, and that each chunk can be opened. The preflight function opens and closes each stored chunk but does not recompute its byte length or hash before response headers are set and ZIP bytes are streamed. The ZIP writer copies stored bytes into the archive without checking that the bytes still match metadata. [R-HTTPAPI-UPLOAD] [R-HTTPAPI-CHUNK] [R-HTTPAPI-BUNDLES] [R-HTTPAPI-BUNDLE-ZIP] [R-HTTPAPI-BUNDLE-MANIFEST]

Why it matters: if local storage or an S3-compatible object store returns corrupted or wrong bytes after upload, the server can produce a ZIP whose manifest still contains the original expected hash while the chunk entry differs. A careful downstream verifier could detect the mismatch from the manifest, but the current server path does not fail closed before serving the bundle. For a system whose core value is preserving encrypted evidence, making bundle generation verify stored bytes against metadata before serving would tighten the integrity story without adding decryption.

This is not a claim that corruption is likely, that S3 is unsafe, or that uploaded hashes are ignored. AWS S3 documents checksum mechanisms for object upload/download integrity and checksum-mode response fields for `GetObject`; the reviewed implementation does not appear to use those S3 checksum features in addition to its metadata hash. S3-compatible providers may differ, so repository-side verification remains useful when portability is the goal. [S-AWS-S3-CHECKSUMS] [S-AWS-S3-GETOBJECT] [S-AWS-S3-PUTOBJECT]

Minimal actionable fix: add a pre-send verification step for bundle downloads that opens every chunk, streams it through SHA-256 while counting bytes, compares both values to metadata, and fails with a safe error before writing ZIP headers if any chunk mismatches or is missing. For direct chunk-download routes, consider the same verification or document that clients must verify returned bytes against metadata. For S3, optionally evaluate checksum mode or object metadata as a supplementary signal, but keep repository metadata verification as the portable baseline.

Acceptance criteria:

1. Completed stream and incident bundle paths verify chunk byte size and SHA-256 against metadata before sending the ZIP response.
2. A local-storage test corrupts a committed chunk after metadata insertion and verifies that bundle generation fails closed.
3. An S3-compatible storage test or fake-client test covers mismatched object contents or metadata.
4. The public error does not expose stored paths, object keys, raw ciphertext, private deployment details, or incident-sensitive data.
5. Documentation clarifies whether the server, downstream client, or both are expected to verify bundle chunk hashes.

Validation limits: this report did not generate a ZIP bundle, corrupt a blob, or run local/S3 tests. The finding is based on static source review of bundle preflight and write paths. [V-NOT-EXECUTED]

### F-003: Blob rollback cleanup after metadata failure ignores deletion errors

Severity: Low
Confidence: High
Current implementation vs future design: Current implementation
Branch scope: Reviewed branch/ref `v0.11.0-rc.1`, commit `4644ec7648e9718a5cb31a148c10d67497bf468d`
Release impact: Non-blocking operational hardening
Suggested public issue title: `Track sanitized rollback cleanup failures after blob commit metadata errors`
Priority: Low
Type: Storage cleanup / observability
Labels: `backlog`, `maintenance`, `testing`
Sensitive handling: Safe for public issue if described generically; do not include real stored paths, object keys, incident IDs, object-store credentials, ciphertext, or private deployment topology

The upload flow commits a staged blob before inserting chunk metadata. If metadata insertion then fails because of duplicate metadata, closed/deleting incident state, quota, missing stream/incident, invalid stream state, or another insert error, the handler calls `removeCommittedBlobAfterMetadataFailure`. That helper creates a short timeout context and calls `a.store.Remove`, but it ignores the returned error. [R-HTTPAPI-CHUNK]

Why it matters: if rollback deletion fails, the repository can leave an encrypted blob in local or S3-compatible storage without metadata pointing to it. That does not expose plaintext and does not prove a user-visible failure in ordinary paths, because metadata remains the source of truth. It does, however, create operational drift: storage cost, retention ambiguity, cleanup difficulty, and possible confusion when reconciling object storage against metadata. For S3 specifically, delete behavior can depend on permissions and bucket versioning state; AWS documents that `DeleteObject` behavior differs for non-versioned, versioned, and versioning-suspended buckets. [R-STORAGE-COMMITTED] [R-STORAGE-S3] [S-AWS-S3-DELETEOBJECT]

Minimal actionable fix: make rollback cleanup return or log a sanitized failure signal that avoids stored paths and object keys. A safe implementation could increment a metric, log a generic component/stage/error-class field without sensitive storage identifiers, and optionally enqueue a metadata-free cleanup retry record that uses server-controlled internal identifiers only if the privacy model permits it. The fix should not create a public object listing, reveal object keys, or add a user-facing diagnostic containing private deployment details. OWASP logging guidance supports excluding access tokens, sensitive personal data, credentials, connection strings, encryption keys, and primary secrets from logs. [S-OWASP-LOGGING]

Acceptance criteria:

1. Rollback cleanup failure is observable through a sanitized log or metric that does not include stored paths, object keys, raw request data, uploaded bytes, ciphertext, tokens, or private deployment details.
2. Tests cover a fake blob store whose `Remove` fails after metadata insertion failure and verify the safe observable signal.
3. If retry cleanup is added, it is bounded, idempotent, and uses server-controlled metadata rather than client-supplied paths.
4. Documentation or runbook notes explain that rollback cleanup failure leaves encrypted orphan blobs and must not be debugged through public issue comments containing storage identifiers.

Validation limits: this report did not induce metadata failures or exercise local/S3 rollback cleanup. The finding is based on static source review. [V-NOT-EXECUTED]

## Non-Findings And Confirmed Boundaries

The reviewed documentation repeatedly states that Proofline is experimental and not production-ready. The changelog for v0.11.0-rc.1 explicitly frames the release candidate as ordinary pre-v1 experimental software, not v1 preview, not final v0.11.0, and not production-ready. This is a confirmed boundary, not a defect. [R-README] [R-CHANGELOG] [R-SECURITY]

The current implementation separates main and private-admin handlers at construction and route registration time. Main routes include `/v1`, relay, sharing/wrapped-key, and public viewer routes; admin API and admin web routes are registered under the admin handler. The documentation still correctly warns that separate ports are not a complete security model and that deployment edges must avoid routing admin/operator surfaces publicly. [R-CMD-API] [R-HTTPAPI-ROUTES] [R-ARCHITECTURE] [R-CODE-MAP]

The public viewer is read-only and token-scoped. Raw viewer tokens are returned once when created, repository storage keeps only token hashes, invalid/expired/revoked tokens collapse into a generic public error, token-bearing paths are redacted in logs, and viewer payloads summarize metadata without stored paths or encrypted file bytes. [R-HTTPAPI-VIEWER] [R-INCIDENT-TOKENS] [R-HTTPAPI-MIDDLEWARE]

The legacy `/e/{token}` viewer alias is intentionally preserved for already shared links and should not be treated as stale product naming by itself. The canonical path is `/i/{token}` for new links. [R-HTTPAPI-ROUTES] [R-HTTPAPI-MIDDLEWARE] [R-REPORT-SCOPE]

The reviewed tree includes optional PostgreSQL metadata support. The code opens PostgreSQL through `pgx`, applies embedded migrations with checksums, uses an advisory lock during migrations, applies migration steps transactionally, and uses database constraints and transactional checks for chunk metadata insertion. No live PostgreSQL validation was supplied or executed by this report, so this is a code/docs confirmation rather than live backend certification. [R-POSTGRES-DB] [R-POSTGRES-MIGRATE] [R-POSTGRES-CHUNKS] [R-POSTGRES-INIT-SQL] [S-POSTGRES-TRANSACTIONS] [S-POSTGRES-CONSTRAINTS] [V-NOT-EXECUTED]

The reviewed tree includes optional S3-compatible blob storage. Object keys are derived from cleaned server-controlled paths, prefixes reject unsafe path segments, writes use conditional no-overwrite semantics, and missing objects are mapped to standard not-exist behavior. This report did not execute a live S3-compatible smoke test and does not claim provider-specific parity beyond the reviewed code and AWS S3 semantics used as the authoritative S3 API reference. [R-STORAGE-S3] [R-STORAGE-PATHS] [S-AWS-S3-PUTOBJECT] [S-AWS-S3-OBJECT-KEYS] [V-NOT-EXECUTED]

The reviewed tree includes optional Valkey/Redis-compatible short-lived coordination. The code treats coordination as non-durable, keeps metadata and blob storage as the source of truth, returns generic availability errors, and uses token-matched lease release. This report did not run a Valkey/Redis service and does not claim tested cluster behavior. [R-COORDINATION] [R-VALKEY] [S-VALKEY-SET] [S-VALKEY-EXPIRE] [V-NOT-EXECUTED]

The incident-mode planning document is not an implementation of emergency-services integration, automatic escalation, notifications, key release, retention changes, or decryption. The current optional mode metadata fields are labels unless later code implements explicit behavior. Missing first-class mode behavior is therefore not a current defect. [R-INCIDENT-MODES] [R-REPORT-SCOPE]

The key-custody, browser-decryption, and break-glass documents are future design guardrails. The current backend remains ciphertext-only, does not store raw media keys, does not decrypt media, does not add browser decryption, does not add server escrow, and does not implement emergency-services contact. Missing those future features is not a current defect. [R-KEY-CUSTODY] [R-BROWSER-DECRYPTION] [R-BREAK-GLASS]

The stream-ingress relay path is scoped as a narrow relay surface, not a public admin or viewer surface. The workflow definition includes route smoke checks intended to ensure common admin, main API, viewer, and metrics paths are not mounted on the relay image. This report did not run those smoke checks. [R-ARCHITECTURE] [R-CI] [V-NOT-EXECUTED]

No public issue should include private vulnerabilities, raw tokens, secrets, exploit details, private deployment details, object-store credentials, raw request data, uploaded bytes, plaintext, raw keys, object keys, stored paths, or user-safety data. This is a confirmed publication boundary for any follow-up work. [R-SECURITY] [R-AGENTS] [R-REPORT-SCOPE]

## Follow-Up Recommendations

1. Review the generated local issue drafts before creating public GitHub issues. The drafts are scoped to branch `develop` and the reviewed commit, and should remain public-safe if promoted. [G-ISSUE-DRAFTS] [R-REPORT-SCOPE]

2. Collect validation evidence for the reviewed commit before making release-readiness statements. Recommended evidence includes `gofmt`, `go test ./...`, `go vet ./...`, Docker build output, CI run URLs, PostgreSQL integration output, S3-compatible storage smoke or fake-client output, Valkey/Redis startup and TTL behavior evidence, simulator smoke output, and attestation verification output where applicable. [R-CI] [V-NOT-EXECUTED]

3. Treat F-001, F-002, and F-003 as non-blocking public follow-up issues unless maintainers decide that reproducible CI service images, server-side stored-blob hash verification on download, or orphan-blob cleanup observability are release-blocking for v0.11.0-rc.1. Any public issues must avoid sensitive storage identifiers, raw tokens, uploaded bytes, request bodies, private deployment details, object keys, or exploit payloads. [R-SECURITY] [R-AGENTS]

4. Revalidate optional backend paths with live disposable services or supplied CI evidence. The code/docs review found coherent PostgreSQL, S3-compatible storage, and Valkey/Redis-compatible coordination boundaries, but this report did not execute those backends. [R-POSTGRES-DB] [R-STORAGE-S3] [R-VALKEY] [V-NOT-EXECUTED]

5. Preserve the implementation-vs-future-planning separation in release notes and issues. Avoid describing browser decryption, break-glass access, server escrow, emergency-services integration, trusted-contact incident decryption, public account portals, playable export, OAuth/JWT auth, iOS/Android clients, or first-class incident-mode behavior as implemented unless the reviewed tree contains explicit implementation code and tests. [R-INCIDENT-MODES] [R-KEY-CUSTODY] [R-BROWSER-DECRYPTION] [R-BREAK-GLASS]

## Conclusion

Based on source and documentation review at commit `4644ec7648e9718a5cb31a148c10d67497bf468d`, Proofline v0.11.0-rc.1 appears internally consistent with its documented experimental, ciphertext-only, server-side backend scope. The reviewed tree shows strong route-boundary awareness, sensitive logging avoidance, token hashing, server-controlled blob paths, optional backend fail-closed configuration, and clear planning boundaries for future high-risk features.

The report identified no Critical or High issues. The three Low findings are practical hardening items: pin the PostgreSQL CI service image or document the exception, verify stored chunk hash and length before serving completed bundles, and make rollback cleanup failures safely observable. None of these findings by itself supports a claim that the release candidate is production-ready, and none should be inflated into a public emergency.

The main limitation is runtime validation evidence. No application tests, builds, containers, simulator runs, or live optional-backend smoke tests were independently executed or supplied for this report. Phase 2 publication validation checked the report itself, not the release artifact behavior.

## Citation References

[R-REPORT-SCOPE]: #source-registry "Maintainer-supplied reviewed branch/ref, commit SHA, target version, report path, model/tool disclosure, and Codex report-validation scope."
[R-COMMIT]: https://github.com/open-proofline/server/commit/4644ec7648e9718a5cb31a148c10d67497bf468d
[R-README]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/README.md
[R-SECURITY]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/SECURITY.md
[R-CHANGELOG]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/CHANGELOG.md
[R-AGENTS]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/AGENTS.md
[R-DOCS-README]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/docs/README.md
[R-ARCHITECTURE]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/docs/architecture.md
[R-CODE-MAP]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/docs/code-map.md
[R-CONFIGURATION]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/docs/configuration.md
[R-CI]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/.github/workflows/ci.yml
[R-DEPENDABOT]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/.github/dependabot.yml
[R-DOCKERFILE]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/Dockerfile
[R-INGRESS-DOCKERFILE]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/Dockerfile.ingress
[R-DOCKERIGNORE]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/.dockerignore
[R-GOMOD]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/go.mod
[R-CMD-API]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/cmd/api/main.go
[R-HTTPAPI-API]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/api.go
[R-HTTPAPI-ROUTES]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/routes.go
[R-HTTPAPI-MIDDLEWARE]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/middleware.go
[R-HTTPAPI-UPLOAD]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/upload.go
[R-HTTPAPI-CHUNK]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/chunk_handlers.go
[R-HTTPAPI-VIEWER]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/incident_viewer.go
[R-HTTPAPI-BUNDLE-ZIP]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/bundle_zip.go
[R-HTTPAPI-BUNDLES]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/bundles.go
[R-HTTPAPI-BUNDLE-MANIFEST]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/httpapi/bundle_manifest.go
[R-STORAGE-CORE]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/storage/blob_store.go
[R-STORAGE-TEMP]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/storage/temp_upload.go
[R-STORAGE-PATHS]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/storage/paths.go
[R-STORAGE-COMMITTED]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/storage/committed_blobs.go
[R-STORAGE-S3]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/storage/s3.go
[R-POSTGRES-DB]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/postgresdb/db.go
[R-POSTGRES-MIGRATE]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/postgresdb/migrate.go
[R-POSTGRES-CHUNKS]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/postgresdb/chunks.go
[R-POSTGRES-MIGRATION-EMBED]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/migrations/postgres/migrations.go
[R-POSTGRES-INIT-SQL]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/migrations/postgres/001_init.sql
[R-SQLITE-DB]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/db/db.go
[R-COORDINATION]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/coordination/coordinator.go
[R-VALKEY]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/coordination/valkey.go
[R-INCIDENT-TOKENS]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/incidents/incident_tokens.go
[R-INCIDENT-IDS]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/internal/incidents/ids.go
[R-INCIDENT-MODES]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/docs/incident-modes.md
[R-KEY-CUSTODY]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/docs/key-custody.md
[R-BROWSER-DECRYPTION]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/docs/browser-decryption.md
[R-BREAK-GLASS]: https://github.com/open-proofline/server/blob/4644ec7648e9718a5cb31a148c10d67497bf468d/docs/break-glass-key-access.md

[S-GITHUB-ACTIONS-PINNING]: https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#using-third-party-actions
[S-DOCKER-DIGESTS]: https://docs.docker.com/reference/cli/docker/image/pull/#pull-an-image-by-digest-immutable-identifier
[S-AWS-S3-PUTOBJECT]: https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html
[S-AWS-S3-GETOBJECT]: https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html
[S-AWS-S3-OBJECT-KEYS]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-keys.html
[S-AWS-S3-CHECKSUMS]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html
[S-AWS-S3-DELETEOBJECT]: https://docs.aws.amazon.com/AmazonS3/latest/API/API_DeleteObject.html
[S-POSTGRES-TRANSACTIONS]: https://www.postgresql.org/docs/current/tutorial-transactions.html
[S-POSTGRES-CONSTRAINTS]: https://www.postgresql.org/docs/current/ddl-constraints.html
[S-SQLITE-FOREIGN-KEYS]: https://www.sqlite.org/foreignkeys.html
[S-SQLITE-WAL]: https://www.sqlite.org/wal.html
[S-VALKEY-SET]: https://valkey.io/commands/set/
[S-VALKEY-EXPIRE]: https://valkey.io/commands/expire/
[S-REDIS-PROTOCOL]: https://redis.io/docs/latest/develop/reference/protocol-spec/
[S-OWASP-LOGGING]: https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html
[S-GO-POSTGRES-CLIENT]: https://pkg.go.dev/github.com/jackc/pgx/v5
[S-GO-S3-CLIENT]: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3
[S-GO-VALKEY-CLIENT]: https://pkg.go.dev/github.com/redis/go-redis/v9

[V-REVIEW-INPUT]: #validation-and-execution-evidence "Maintainer-supplied reviewed branch/ref, commit SHA, target version, review date, report path, and model/tool disclosure."
[V-CI-WORKFLOW-DEFINITION]: #validation-and-execution-evidence "Repository workflow definition inspected at the reviewed commit; no run logs supplied."
[V-PHASE2-CODEX]: #validation-and-execution-evidence "Codex Phase 2 report-validation pass for report cleanup, source re-checks, citations, markdown links, and diff hygiene."
[V-NOT-EXECUTED]: #sources-checks-and-commands-not-available-or-not-executed "No application tests, builds, containers, simulator runs, or live backend smoke tests were executed by this report."
[G-ISSUE-DRAFTS]: #generated-artifacts-and-report-outputs "Local branch-scoped issue drafts created under .backlog-drafts; not public GitHub issues."
