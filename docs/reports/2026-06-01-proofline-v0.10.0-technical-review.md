# Technical Review of Proofline v0.10.0

**Repository:** `open-proofline/server`
**Reviewed branch/ref:** `main`
**Reviewed commit SHA:** `74ec526123708b7a4904f25b8e805e9847fcfdbe`
**Target release/version:** `v0.10.0`
**Review date:** 2026-06-01
**Phase 2 validation date:** 2026-06-01
**Report status:** Final public report after Codex Phase 2 validation. No
new branch-scoped issue drafts were created because the Phase 1 release-tag
evidence gap was resolved during validation and the remaining non-blocking
cautions were already documented or covered by current review guidance.

**Citation format note:** This report uses portable citation keys only.
Repository citations are pinned to reviewed commit
`74ec526123708b7a4904f25b8e805e9847fcfdbe`; external citations resolve to
canonical documentation URLs. No ChatGPT-internal citation tokens are used.

**AI-assisted review disclosure:** This report began as an OpenAI ChatGPT Deep
Research draft, then was validated, corrected, and public-hardened with Codex.
The supplied inputs did not identify the Deep Research model. This report is
not a formal security audit, penetration test, compliance certification, legal
review, App Store review, Play Store review, or production-readiness
endorsement.

**Public-disclosure note:** This report is intended for public project
documentation. It intentionally avoids raw tokens, secrets, private deployment
details, exploit payloads, raw keys, plaintext media, and user-safety data.

## Executive Summary

Proofline `v0.10.0` is a public-backend prototype groundwork release for the
Go server backend. At the reviewed commit, the server receives already
encrypted chunks through authenticated main `/v1` routes, keeps the private
`/admin` dashboard on a separate listener group, stores metadata in SQLite by
default or optional PostgreSQL, stores encrypted blobs locally by default or in
optional S3-compatible storage, and can use optional Valkey/Redis-compatible
coordination for route-class counters and short-lived complete-upload leases
when explicitly configured. [R-CORE] [R-DOCS] [R-API] [R-CONFIG] [R-COORD]

The reviewed tree preserves the important security boundaries. Uploaded chunks
remain immutable ciphertext, evidence bundles remain encrypted ZIP bundles
rather than playable exports, raw session and incident-viewer tokens are
stored only as hashes, public viewer routes are read-only, and the current
wrapped-key metadata features do not add backend decryption, browser
decryption, raw media-key storage, key escrow, break-glass access, or
trusted-contact accounts. [R-SECURITY-MODEL] [R-THREAT-MODEL] [R-ENCRYPTION]
[R-KEY-CUSTODY]

Phase 2 validation did not identify a new code-level release blocker. The
Phase 1 draft's main release-process concern was that tag-context release
workflow evidence was not available to the draft. That concern was resolved
during Phase 2: tag `v0.10.0` points to the reviewed commit, the tag-triggered
CI run completed successfully, the binary attestation job completed
successfully, the release binary upload job completed successfully, the Docker
publish and Docker image attestation steps completed successfully, and the
GitHub release contains the uploaded Linux binary asset. [V-CI-TAG]
[V-RELEASE]

The remaining cautions are operational and maintenance-oriented rather than
release blockers. Existing `/v1/admin/...` JSON routes remain on the main
handler and must not be routed from a public edge; this is intentional,
documented, and covered by the main API/public viewer listener split work.
Application log redaction currently covers canonical `/i/{token}` and legacy
`/e/{token}` viewer paths; future token-bearing URL patterns must receive the
same review before being added. [R-API] [R-ROUTES] [R-DEPLOYMENT]
[R-PUBLIC-SPLIT] [V-ISSUES]

## Source Registry

### Repository Sources Inspected

Unless noted otherwise, repository entries below were reviewed against commit
`74ec526123708b7a4904f25b8e805e9847fcfdbe`.

| Key | Source type | Location | Purpose | Status | Limitations |
|---|---|---|---|---|---|
| R-CORE | Repository files | `README.md`, `CHANGELOG.md`, `SECURITY.md`, `AGENTS.md` | Product scope, release framing, repository rules, security warnings, public-safety restrictions | Reviewed | Static reading only |
| R-DOCS | Repository files | `docs/README.md`, `docs/architecture.md`, `docs/code-map.md`, `docs/configuration.md`, `docs/development.md` | Architecture, package layout, backend selectors, development and report workflow | Reviewed | Documentation only |
| R-API | Repository file | `docs/api.md` | Current route contracts, listener notes, upload semantics, sharing and wrapped-key API behavior | Reviewed | Does not prove live deployment behavior |
| R-DEPLOYMENT | Repository file | `docs/deployment.md` | Deployment boundary, Docker port exposure, reverse-proxy guidance, token-path logging notes | Reviewed | Deployment guidance only |
| R-SECURITY-MODEL | Repository file | `docs/security-model.md` | Listener boundary, account/session controls, upload/storage controls, current gaps | Reviewed | Threat-oriented documentation only |
| R-THREAT-MODEL | Repository file | `docs/threat-model.md` | Assets, trust boundaries, current controls, known limitations | Reviewed | No live attacker testing |
| R-PLANNING | Repository files | `docs/incident-modes.md`, `docs/v1-access-control.md`, `docs/live-partial-stream-access-boundary.md`, `docs/regional-stream-ingress-relay.md`, `docs/mode-aware-retention-policy.md` | Future mode, access, live-stream, relay, and retention boundaries | Reviewed | Planning-only unless implementation exists |
| R-KEY-CUSTODY | Repository files | `docs/key-custody.md`, `docs/contact-key-sharing-grants.md`, `docs/browser-decryption.md`, `docs/break-glass-key-access.md`, `docs/contact-wrapped-key-metadata-simulator.md` | Current wrapped-key boundary plus future custody/decryption planning | Reviewed | No production key custody or decryption behavior |
| R-ENCRYPTION | Repository files | `docs/encryption.md`, `internal/envelope/*.go` | Simulator envelope, ciphertext-only backend posture, compatibility naming | Reviewed | Simulator/runtime behavior not executed locally |
| R-CONFIG | Repository files | `cmd/api/*.go`, `internal/config/*.go` | Startup wiring, backend selection, listener binding, timeout and rate-limit settings | Reviewed | No local server execution |
| R-ROUTES | Repository file | `internal/httpapi/routes.go` | Main, public viewer, and private-admin route registration | Reviewed | Static inspection only |
| R-HTTP-SEC | Repository files | `internal/httpapi/middleware.go`, `internal/httpapi/response.go`, relevant tests | Request logging, token-path redaction, no-store and browser-security headers | Reviewed | Static inspection plus existing tests only |
| R-UPLOAD | Repository files | `internal/httpapi/upload.go`, `internal/httpapi/chunk_handlers.go`, `internal/httpapi/idempotency.go`, `internal/incidents/upload_operations.go`, tests | Upload parsing, immutable upload fingerprinting, idempotency semantics | Reviewed | No local upload replay |
| R-STORAGE | Repository files | `internal/storage/*.go` | Local and S3-compatible blob storage, temp staging, immutable commit behavior | Reviewed | No live object store or filesystem replay |
| R-BUNDLE | Repository files | `internal/httpapi/bundle_zip.go`, `internal/httpapi/bundle_manifest.go` | ZIP entry naming, encrypted evidence bundle manifests | Reviewed | No bundle generated locally |
| R-SQLITE | Repository files | `internal/db/*.go`, `internal/incidents/*.go`, `migrations/*.sql` | Default SQLite metadata backend and migrations | Reviewed | No live SQLite execution |
| R-POSTGRES | Repository files | `internal/postgresdb/*.go`, `migrations/postgres/*.sql`, `docs/postgresql-metadata-migration.md` | Optional PostgreSQL backend, migrations, parity expectations | Reviewed | No live PostgreSQL execution |
| R-COORD | Repository files | `internal/coordination/*.go`, upload/rate-limit coordination callers | Optional Valkey/Redis-compatible startup checks, counters, and upload leases | Reviewed | No live Valkey/Redis service exercised locally |
| R-CI | Repository files | `.github/workflows/ci.yml`, `.github/dependabot.yml`, `Dockerfile`, `.dockerignore` | CI jobs, workflow permissions, action pinning, binary/Docker build paths | Reviewed | Workflow file inspection only; run evidence separate |
| R-PHASE2-PROMPT | Repository file | `codex/prompts/95-validate-deep-research-report.md` on current branch | Phase 2 validation workflow used for this publication pass | Reviewed | Process guidance, not product behavior |

### External Authoritative Sources

| Key | Source type | Location | Purpose | Status | Limitations |
|---|---|---|---|---|---|
| S-GO-HTTP | Official documentation | Go `net/http` package documentation | HTTP timeout and server behavior context | Consulted by Phase 1 | General Go API documentation |
| S-S3 | Official documentation | Amazon S3 conditional writes documentation | Conditional no-overwrite object-write context | Consulted by Phase 1 | S3-compatible providers may vary |
| S-POSTGRES | Official documentation | PostgreSQL advisory lock documentation | Migration lock strategy context | Consulted by Phase 1 | General PostgreSQL semantics |
| S-OWASP-LOGGING | Security guidance | OWASP Logging Cheat Sheet | Token, secret, and sensitive-data logging exclusions | Consulted by Phase 1 | Guidance, not implementation proof |
| S-REDIS | Official documentation | Redis `EXPIRE` command documentation | Short-lived coordination and counter expiry context | Consulted by Phase 1 | General Redis semantics |
| S-RFC9111 | Standards documentation | RFC 9111 HTTP caching | `Cache-Control: no-store` context | Consulted by Phase 1 | HTTP caching semantics only |
| S-DOCKER | Official documentation | Docker multi-stage build documentation | Dockerfile shape and build-stage context | Consulted by Phase 1 | General Docker guidance |
| S-GITHUB-ACTIONS | Official documentation | GitHub Actions secure use reference | Workflow pinning and CI/CD hygiene context | Consulted by Phase 1 | General GitHub guidance |

### Validation And Execution Evidence

| Key | Evidence type | Location | Purpose | Status | Limitations |
|---|---|---|---|---|---|
| V-CI-TAG | Public GitHub Actions metadata | CI run `26732579720` for tag push `v0.10.0`, head SHA `74ec526123708b7a4904f25b8e805e9847fcfdbe` | Tag-context release workflow evidence | Reviewed; successful | Metadata only; logs and artifacts were not downloaded |
| V-CI-MAIN | Public GitHub Actions metadata | CI run `26732545330` for push to `main`, head SHA `74ec526123708b7a4904f25b8e805e9847fcfdbe` | Main-branch commit CI evidence | Reviewed; successful | Metadata only; logs and artifacts were not downloaded |
| V-CI-SYNC | Public GitHub Actions metadata | CI run `26732873700` for PR sync from `main`, head SHA `74ec526123708b7a4904f25b8e805e9847fcfdbe` | Additional commit-associated CI signal | Reviewed; successful | Metadata only; logs and artifacts were not downloaded |
| V-RELEASE | Public GitHub release metadata | Release `v0.10.0` | Confirms published release and uploaded `proofline-server-linux-amd64` asset | Reviewed | Asset contents and attestation bundle were not downloaded |
| V-ISSUES | Public GitHub issue and PR metadata | Issues `#164`, `#165`, `#166`, `#167`, `#160` and related merged PRs | Confirms listener split, rate limit, and Valkey coordination work were already tracked and closed | Reviewed | Metadata only |
| V-PHASE2 | Local Codex Phase 2 checks | Current branch `add-v0.10.0-technical-review` at `c40f8427b4e51aa68ef3641575067d0234deaf94` before report edits | Report correction, citation cleanup, public-safety review, docs-only validation | Completed | Report-validation only, not runtime proof |

### Sources, Checks, And Commands Not Executed Locally

Phase 2 did not run `go test ./...`, `go vet ./...`, `gofmt`,
`govulncheck`, Docker builds, compose stacks, the API server, the simulator,
PostgreSQL, S3-compatible storage, Valkey/Redis, release artifact download,
attestation verification, or live HTTP requests. Public GitHub Actions
metadata indicates the relevant CI jobs succeeded for the reviewed commit and
tag, including Go tests, `go vet`, PostgreSQL integration tests,
`govulncheck`, binary startup smoke, Docker startup smoke, binary attestation,
release binary upload, Docker publish, and Docker image attestation. [V-CI-TAG]
[V-CI-MAIN]

No live deployment, reverse proxy, firewall, DNS, TLS certificate, host logs,
backup/restore workflow, disk encryption setup, real incident data, real
viewer token, user-safety data, private repository settings, branch ruleset
configuration, environment secrets, or package settings were inspected.

No iOS, Android, web-client, protocol, notification, browser-decryption,
production key-custody, break-glass, or public `/v1` product-authentication
implementation existed in the reviewed tree. Those areas are not treated as
implemented behavior. [R-PLANNING] [R-KEY-CUSTODY]

### Generated Artifacts And Report Outputs

| Artifact | Purpose | Status |
|---|---|---|
| `docs/reports/2026-06-01-proofline-v0.10.0-technical-review.md` | Cleaned public technical review report | Generated by Phase 2 |
| `docs/reports/README.md` | Reports index | Updated to list this report |
| `.backlog-drafts/2026-06-01/add-v0.10.0-technical-review/` | Branch-scoped local issue drafts | Not created; no new actionable findings required a draft |

## Scope And Method

This report validates a Deep Research draft for `open-proofline/server` at
reviewed commit `74ec526123708b7a4904f25b8e805e9847fcfdbe`, target release
`v0.10.0`. The Phase 2 pass checked repository facts against the reviewed
commit and current checked-out branch, removed ChatGPT-internal citation
tokens, corrected release evidence after live metadata review, kept future
designs separate from current implementation, and preserved public-safe
citation keys. [R-PHASE2-PROMPT]

The current branch used for publication work was
`add-v0.10.0-technical-review` at
`c40f8427b4e51aa68ef3641575067d0234deaf94` before these report edits. That
branch had no tracked local changes at the start of Phase 2. Repository facts
in this report remain pinned to the reviewed release commit. [V-PHASE2]

This is a static technical review plus validation of public GitHub metadata.
It is not a formal audit, penetration test, legal review, compliance review,
production-readiness certification, or live deployment review.

## Current Implementation Summary

The backend remains a Go server repository, not a complete Proofline product
suite. The current code starts a main API/viewer listener group and a separate
private-admin dashboard listener group. The main listener carries
authenticated `/v1` product routes, authenticated admin-only JSON API routes,
canonical read-only `/i/{token}` viewer routes, legacy `/e/{token}` viewer
aliases, and token-neutral public viewer static assets. The private-admin
listener carries only `/admin` dashboard routes and `/admin/static/...` assets.
[R-CORE] [R-API] [R-ROUTES]

The listener split is intentionally conservative rather than a production
public-exposure claim. Existing `/v1/admin/...` JSON routes remain on the main
handler for compatibility and must be blocked by any public reverse proxy.
The private `/admin` dashboard is separately bound and must stay behind
localhost, LAN, WireGuard, firewall rules, or a strict private reverse proxy.
Separate bind addresses reduce accidental exposure, but they are not a complete
security model. [R-DEPLOYMENT] [R-SECURITY-MODEL] [R-PUBLIC-SPLIT]

The upload pipeline remains evidence-preserving. File parts stream to temporary
storage while SHA-256 is computed over the accepted ciphertext bytes. The
server validates request metadata, hashes idempotency keys before storage, can
use optional short-lived Valkey/Redis-compatible upload leases, commits final
blobs through no-overwrite storage semantics, and writes metadata only after a
successful blob commit. The durable truth remains metadata rows plus immutable
blob storage, not Valkey lease state. [R-UPLOAD] [R-STORAGE] [R-COORD]

SQLite remains the default metadata backend. PostgreSQL metadata is optional
and explicitly configured. Local filesystem blob storage remains the default.
S3-compatible blob storage is optional and explicitly configured. No
coordination backend is used by default; Valkey/Redis-compatible coordination
is optional and short-lived when configured. These optional backends expand
deployment shapes but do not add backend decryption, public `/v1` readiness, or
complete production-cluster guarantees by themselves. [R-DOCS] [R-SQLITE]
[R-POSTGRES] [R-STORAGE] [R-COORD]

The v0.10.0 wrapped-key work is a metadata and authorization step, not a
decryption step. Account owners can register contact public-key metadata,
create incident or stream scoped sharing grants, and store grant-bound wrapped
media-key metadata. Those routes do not add trusted-contact accounts, public
viewer key delivery, browser decryption, backend decryption, raw media-key
storage, key escrow, break-glass access, or playable media export. [R-API]
[R-KEY-CUSTODY] [R-ENCRYPTION]

Evidence bundles remain encrypted chunk bundles. ZIP entry names are generated
by server-controlled logic, not by client-provided stored paths. Bundle
manifests may include non-secret encryption hints and current display metadata,
but they do not include raw media keys. The backend treats uploaded bytes as
opaque ciphertext and validates hashes over ciphertext. [R-BUNDLE]
[R-SECURITY-MODEL] [R-ENCRYPTION]

Token handling is coherent for the current scope. Session tokens and
incident-viewer tokens are opaque bearer credentials returned raw only at
creation or login time and stored as SHA-256 hashes. Incident-viewer token
lookup collapses invalid, expired, and revoked states into one public error.
Application request logging records route patterns for current token-bearing
viewer paths rather than raw `/i/{token}` or `/e/{token}` values. [R-HTTP-SEC]
[R-SQLITE] [R-POSTGRES]

The release workflow evidence is materially stronger than the Phase 1 draft
could confirm. The tag-triggered CI run succeeded and included the jobs needed
to exercise the tag-only release path: binary attestation, release binary
upload, Docker publication, and Docker image attestation. The public release
for `v0.10.0` is published and includes `proofline-server-linux-amd64`.
[V-CI-TAG] [V-RELEASE]

## Findings And Release Judgement

Phase 2 validation separates resolved Phase 1 concerns from remaining
non-blocking cautions.

| Finding | Blocking | Phase 2 assessment |
|---|---|---|
| Tag-context release workflow evidence was missing from the Phase 1 draft | No, resolved | Live metadata now shows tag `v0.10.0` points to the reviewed commit and the tag-triggered CI run succeeded, including binary attestation, release upload, Docker publication, and Docker image attestation. [V-CI-TAG] [V-RELEASE] |
| Admin JSON APIs remain under `/v1/admin/...` on the main listener | No | Intentional and documented. Public reverse proxies must block these routes, and the listener split design/implementation work is already tracked and closed. [R-API] [R-DEPLOYMENT] [V-ISSUES] |
| Token-path log redaction depends on known token-bearing URL patterns | No | Current `/i/{token}` and `/e/{token}` viewer routes are redacted and covered by tests. Future token-bearing URL patterns need the same review before being introduced. [R-HTTP-SEC] |
| Phase 2 did not run local tests, builds, containers, or simulator flows | No | This report relies on static validation and public CI metadata. Local replay remains useful release hygiene but was not part of this docs-only publication pass. [V-PHASE2] |

No release-blocking, critical, high, or medium-severity code finding survived
Phase 2 validation. The v0.10.0 code and documentation are coherent enough for
publication of this technical review, with the continued caveat that Proofline
Server is experimental and not production-ready public infrastructure.

## Issue Draft Handling

Issue handling mode was `drafts_only`. No new branch-scoped draft issues were
created during this Phase 2 pass.

The Phase 1 release-process blocker was resolved by live tag and release
workflow evidence. The admin JSON listener concern is already covered by
closed listener-split issues and current documentation. The token-path
redaction concern is retained as a review caution for future token-bearing
routes, but current implementation and tests cover the token-bearing viewer
routes present in the reviewed tree. [V-CI-TAG] [V-RELEASE] [V-ISSUES]
[R-HTTP-SEC]

## Follow-Up Notes

Future work should keep the following review checks active:

- Do not route `/v1/admin/...` from public incident viewer edges.
- Keep `/admin` dashboard routes on the private-admin listener only.
- Add explicit logging and redaction review for any new token-bearing URL
  pattern before the route is added.
- Keep wrapped-key records separate from public viewer tokens and ordinary
  ciphertext bundle access unless a later issue explicitly designs a
  decryption-bearing public-link model.
- Treat Valkey/Redis-compatible coordination as short-lived, non-durable
  coordination; metadata and blob storage remain authoritative.
- Continue to treat browser decryption, backend decryption, raw key custody,
  key escrow, break-glass access, trusted-contact accounts, mobile clients, and
  web clients as future explicit design work.

## References

[R-CORE]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe "Core repository files at reviewed commit"
[R-DOCS]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs "Documentation tree at reviewed commit"
[R-API]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs/api.md "API documentation at reviewed commit"
[R-DEPLOYMENT]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs/deployment.md "Deployment documentation at reviewed commit"
[R-SECURITY-MODEL]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs/security-model.md "Security model at reviewed commit"
[R-THREAT-MODEL]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs/threat-model.md "Threat model at reviewed commit"
[R-PLANNING]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs "Planning documents at reviewed commit"
[R-KEY-CUSTODY]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs/key-custody.md "Key custody documentation at reviewed commit"
[R-ENCRYPTION]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs/encryption.md "Encryption documentation at reviewed commit"
[R-CONFIG]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/config "Configuration package at reviewed commit"
[R-ROUTES]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/httpapi/routes.go "Route registration at reviewed commit"
[R-HTTP-SEC]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/httpapi/middleware.go "HTTP middleware at reviewed commit"
[R-UPLOAD]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/httpapi "HTTP upload handlers at reviewed commit"
[R-STORAGE]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/storage "Storage package at reviewed commit"
[R-BUNDLE]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/httpapi "Bundle generation code at reviewed commit"
[R-SQLITE]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/incidents "SQLite incident repository at reviewed commit"
[R-POSTGRES]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/postgresdb "PostgreSQL metadata backend at reviewed commit"
[R-COORD]: https://github.com/open-proofline/server/tree/74ec526123708b7a4904f25b8e805e9847fcfdbe/internal/coordination "Coordination package at reviewed commit"
[R-CI]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/.github/workflows/ci.yml "CI workflow at reviewed commit"
[R-PUBLIC-SPLIT]: https://github.com/open-proofline/server/blob/74ec526123708b7a4904f25b8e805e9847fcfdbe/docs/public-api-listener-split.md "Main API public exposure listener split at reviewed commit"
[R-PHASE2-PROMPT]: https://github.com/open-proofline/server/blob/c40f8427b4e51aa68ef3641575067d0234deaf94/codex/prompts/95-validate-deep-research-report.md "Phase 2 validation prompt on publication branch"
[S-GO-HTTP]: https://pkg.go.dev/net/http "Go net/http package documentation"
[S-S3]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html "Amazon S3 conditional writes documentation"
[S-POSTGRES]: https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS "PostgreSQL advisory locks documentation"
[S-OWASP-LOGGING]: https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html "OWASP Logging Cheat Sheet"
[S-REDIS]: https://redis.io/docs/latest/commands/expire/ "Redis EXPIRE command documentation"
[S-RFC9111]: https://www.rfc-editor.org/rfc/rfc9111 "RFC 9111 HTTP Caching"
[S-DOCKER]: https://docs.docker.com/build/building/multi-stage/ "Docker multi-stage build documentation"
[S-GITHUB-ACTIONS]: https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions "GitHub Actions security hardening documentation"
[V-CI-TAG]: https://github.com/open-proofline/server/actions/runs/26732579720 "CI run for v0.10.0 tag push"
[V-CI-MAIN]: https://github.com/open-proofline/server/actions/runs/26732545330 "CI run for main push at reviewed commit"
[V-CI-SYNC]: https://github.com/open-proofline/server/actions/runs/26732873700 "CI run for sync PR at reviewed commit"
[V-RELEASE]: https://github.com/open-proofline/server/releases/tag/v0.10.0 "GitHub release v0.10.0"
[V-ISSUES]: https://github.com/open-proofline/server/issues?q=is%3Aissue+%28%23164+OR+%23165+OR+%23166+OR+%23167+OR+%23160%29 "Relevant closed issue metadata"
[V-PHASE2]: https://github.com/open-proofline/server/tree/c40f8427b4e51aa68ef3641575067d0234deaf94 "Publication branch state before report edits"
