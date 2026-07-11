# Technical Review of Proofline unreleased

- Review date: 2026-07-10
- Phase 2 validation date: 2026-07-11
- Current checkout branch: `upgrade-codex-reports-and-repo-full-validation-security-review`
- Current checkout HEAD at review start: `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`
- Initial working-tree status: clean; `git status --short --branch --untracked-files=all` reported only the branch/upstream header
- Reviewed branch/ref: `upgrade-codex-reports-and-repo-full-validation-security-review`
- Reviewed commit SHA: `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`
- Target release/version: `unreleased`
- Phase 1 draft path: `.technical-review-drafts/2026-07-10-proofline-unreleased-technical-review-draft.md`
- Final Phase 2 report path: `docs/reports/2026-07-10-proofline-unreleased-technical-review.md`
- Issue handling mode: `drafts_only`
- Phase 1 model/tool disclosure: OpenAI Codex using GPT-5.6 Sol in Ultra mode.
- Phase 2 validation tool: OpenAI Codex using independent source, execution-evidence, citation, and disclosure-safety checks.

**Publication status:** Independent Phase 2 validation is complete for this report of the reviewed SHA. Phase 2 created only this public-hardened report and ignored branch-scoped local issue drafts; it did not create GitHub issues, private vulnerability reports, pull requests, releases, or repository-setting changes. No matching disposable review container, network, volume, image, or temporary checkout remained at handoff. Suspected vulnerabilities remain routed to GitHub private vulnerability reporting, and public reproduction or exploit detail is intentionally omitted. [R-REPORT-PROMPT] [R-PHASE2-PROMPT] [R-SECURITY] [V-PHASE2-PUBLICATION] [V-PHASE2-CLEANUP]

This is a technical review, not a formal security audit, penetration test, compliance certification, legal review, production-readiness endorsement, or release approval.

## Executive Summary

The reviewed commit is **not ready for release in its present state**. The review identified one High-severity, six Medium-severity, and five Low-severity findings. The public findings cover the pinned Go security patch level, optional PostgreSQL and S3 correctness, release-control enforcement, and optional-backend test reproducibility. Details of suspected or unresolved vulnerabilities are reserved for GitHub private vulnerability reporting.

Release-blocking findings are F-001 through F-009. F-012 is also release-blocking under the maintainer's Phase 0 decision that optional-backend findings block this release. F-010 and F-011 require private triage but were not independently elevated to release blockers because their demonstrated effects are bounded. The release decision is based on source behavior and release controls, not on a failed happy-path smoke test.

Phase 1 recorded broad positive validation evidence. In a clean isolated copy of the exact reviewed SHA, formatting, Markdown links, the full Go test suite, vet, race tests, both application build paths, the disposable PostgreSQL/MinIO/Valkey stack, an induced retry-path stack, live PostgreSQL integration tests, an S3 deletion smoke, and relay packaging all passed; aggregate statement coverage was 54.6%. Phase 2 independently reran formatting, the 112-file Markdown link check, the full Go test suite, vet, and clean-tree checks in a separate disposable exact-SHA clone. It inspected rather than repeated the Phase 1 race, coverage, Docker, and optional-backend stack evidence. [V-LOCAL-BASE] [V-LOCAL-FULLSTACK] [V-LOCAL-EXTENDED] [V-PHASE2-RECHECK]

Those passes do not clear the release. Public GitHub Actions for the reviewed SHA failed at `Go vulnerability scan`: the repository and pinned builder stages select Go 1.26.4, while the authoritative Go vulnerability entry is fixed in Go 1.26.5. Product-specific applicability was not established in this public report and is reserved for private triage. [R-GO-TOOLCHAIN] [R-DOCKERFILE] [R-DOCKERFILE-INGRESS] [V-CI-RUN] [S-GO-RELEASE] [S-GO-VULN-5856]

The reviewed tree correctly presents Proofline as experimental, maintainer-led, ciphertext-only backend infrastructure rather than a production service. Main API/viewer and private-admin listeners remain separated; public viewer routes are read-only; encrypted bundle generation re-verifies stored chunk size and digest; viewer and session tokens are hashed at rest; and current/future product boundaries remain conservative. The latest website and web-client default-branch sources support that framing. [R-README] [R-CHANGELOG] [R-CMD-SERVERS] [R-HTTPAPI-ROUTES] [R-EVIDENCE-BUNDLE] [R-BUNDLE-HTTP] [R-VIEWER-TOKENS] [R-AUTH-REPOSITORIES] [R-AUTH-POSTGRES] [R-WEBSITE-GOV] [R-WEBCLIENT-README]

The maintainer confirmed during Phase 2 that there are no known public or private deployments. The review therefore found no known deployed-user exposure. That deployment context lowers present disclosure sensitivity, but it does not change the technical findings, release-blocking decisions, or the private-reporting path required for unresolved vulnerabilities. [V-MAINTAINER-CONTEXT] [R-SECURITY]

The v1-preview readiness result remains **Not ready for v1 preview claim**. This review concerns an ordinary experimental pre-v1 release, and passing the ordinary validation set must not be converted into a real-user, public `/v1`, or production-readiness claim. [R-V1-CHECKLIST]

## Source Registry

### Repository sources inspected

All server repository URLs below are pinned to the reviewed SHA. A grouped row lists every materially relied-on file in that evidence family.

| Source ID | Source type and location | Commit/ref/date | Purpose | Status | Limitations | Related findings/sections |
|---|---|---|---|---|---|---|
| R-REPORT-PROMPT | Review policy: [`docs/reports/prompts/phase-1-codex-technical-review.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/reports/prompts/phase-1-codex-technical-review.md) | Server SHA `e609ff8`; 2026-07-10 | Governing scope, source, citation, validation, public-safety, and output rules | Inspected in full | Process evidence, not runtime behavior | All sections |
| R-PHASE2-PROMPT | Phase 2 policy: [`codex/prompts/95-validate-technical-review-report.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/codex/prompts/95-validate-technical-review-report.md) | Server SHA `e609ff8`; executed 2026-07-11 | Independent validation, public-hardening, issue-draft, and publication rules | Inspected in full and executed | Process evidence, not runtime behavior | Publication status; Phase 2 method |
| R-PRIOR-REPORT | Prior technical review: [`docs/reports/2026-06-13-proofline-v0.11.0-rc.1-technical-review.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/reports/2026-06-13-proofline-v0.11.0-rc.1-technical-review.md) | Server SHA `e609ff8`; re-read 2026-07-11 | Commit-pinned comparison for previously reported bundle-integrity work | Inspected | Historical review evidence; current implementation was checked directly | Non-findings |
| R-DOCS-INDEX / R-V1-DIRECTION / R-INCIDENT-MODES / R-SECURITY-MODEL / R-THREAT-MODEL / R-API / R-ENCRYPTION | Current and current-versus-future guidance: [`docs/README.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/README.md), [`docs/v1-preview-direction.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/v1-preview-direction.md), [`docs/incident-modes.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/incident-modes.md), [`docs/security-model.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/security-model.md), [`docs/threat-model.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/threat-model.md), [`docs/api.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/api.md), and [`docs/encryption.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/encryption.md) | Server SHA `e609ff8`; re-read 2026-07-11 | Product boundaries, implemented API/security behavior, incident-label limits, ciphertext posture, and v1 direction | Required Phase 2 reads completed; v1 direction read to EOF | Documentation does not replace commit-addressed implementation evidence | Executive summary; implementation summary; non-findings |
| R-KEY-CUSTODY / R-BROWSER-DECRYPTION / R-BREAK-GLASS / R-IOS-PLAN | Future-design guidance: [`docs/key-custody.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/key-custody.md), [`docs/browser-decryption.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/browser-decryption.md), [`docs/break-glass-key-access.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/break-glass-key-access.md), and [`docs/ios-local-recorder-prototype.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/ios-local-recorder-prototype.md) | Server SHA `e609ff8`; re-read 2026-07-11 | Separate future production custody, client decryption, break-glass, and mobile capture from current server behavior | Required Phase 2 reads completed | Planning sources do not implement the described clients or key access | Implementation summary; false-positive review |
| R-README / R-SECURITY / R-CHANGELOG / R-AGENTS | Project boundaries: [`README.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/README.md), [`SECURITY.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/SECURITY.md), [`CHANGELOG.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/CHANGELOG.md), [`AGENTS.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/AGENTS.md) | Server SHA `e609ff8`; 2026-07-10 | Current status, component ownership, security-reporting route, release wording, review constraints | Inspected | Documentation does not prove deployment behavior | Executive summary, boundaries, handling |
| R-CMD-API / R-CMD-SERVERS | Startup and listener construction: [`cmd/api/main.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/cmd/api/main.go), [`cmd/api/servers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/cmd/api/servers.go) | Server SHA `e609ff8`; checked 2026-07-10 | Backend initialization, dependency checks, handler/listener split | Inspected; startup exercised in disposable stacks | No deliberately invalid S3 startup was run | F-007; non-findings |
| R-HTTPAPI-ROUTES / R-HTTPAPI-API | Route and middleware construction: [`internal/httpapi/routes.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/routes.go), [`internal/httpapi/api.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/api.go) | Server SHA `e609ff8`; checked 2026-07-10 | Main/private route separation and middleware placement | Inspected; route boundaries exercised by tests and relay smoke | Not a public-edge deployment test | Non-findings |
| R-AUTH-HANDLERS / R-ADMIN-WEB-AUTH / R-AUTH-MIDDLEWARE | Authentication flows: [`internal/httpapi/auth_handlers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/auth_handlers.go), [`internal/httpapi/admin_web_handlers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/admin_web_handlers.go), [`internal/httpapi/auth_middleware.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/auth_middleware.go) | Server SHA `e609ff8`; checked 2026-07-10 | Authentication and session lifecycle | Inspected | No production traffic or account data was used | Non-findings |
| R-TOTP / R-AUTH-REPOSITORIES / R-AUTH-POSTGRES | Auth primitives and persistence: [`internal/auth/totp.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/auth/totp.go), [`internal/incidents/auth.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/auth.go), [`internal/postgresdb/auth.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/auth.go) | Server SHA `e609ff8`; checked 2026-07-10 | Authentication policy and SQLite/PostgreSQL session persistence | Inspected; ordinary repository tests passed | No production traffic or account data was used | Non-findings |
| R-WEB-AUTH-CONTROLS / R-WEB-AUTH-TESTS | Browser-session controls and tests: [`internal/httpapi/web_auth.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/web_auth.go), [`internal/httpapi/web_cors.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/web_cors.go), [`internal/httpapi/admin_web_session.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/admin_web_session.go), [`internal/config/web_auth.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/config/web_auth.go), [`internal/httpapi/auth_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/auth_test.go), and [`internal/httpapi/admin_web_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/admin_web_test.go) | Server SHA `e609ff8`; checked 2026-07-10 | Cookie attributes, CSRF binding, credentialed-CORS allowlisting, auth-mode ambiguity rejection, and separate admin session policy | Inspected; relevant tests passed | Loopback-local and private-admin exceptions remain explicit configuration choices | Non-findings |
| R-VIEWER-TOKENS / R-VIEWER-TOKEN-SQLITE / R-VIEWER-TOKEN-POSTGRES / R-VIEWER-TOKEN-TESTS | Viewer-token implementation and tests: SQLite [`identifier generation`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/ids.go) and [`token persistence`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/incident_tokens.go); PostgreSQL [`identifier generation`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/ids.go) and [`token persistence`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/incident_tokens.go); [`HTTP handling/tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/incident_viewer.go), [`viewer tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/incident_viewer_test.go), and [`PostgreSQL contract tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/postgresdb_test.go) | Server SHA `e609ff8`; checked 2026-07-10 | Entropy generation, one-time raw-token return, hash storage/lookup, constant-time comparison, expiry/revocation, generic public failures, and viewer behavior | Inspected; repository and HTTP tests passed | No production traffic or token was used | Non-findings |
| R-HTTP-LOGGING / R-HTTP-LOGGING-TESTS | Application logging controls and tests: [`internal/httpapi/middleware.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/middleware.go), [`internal/httpapi/logging.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/logging.go), [`cmd/api/logging.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/cmd/api/logging.go), [`internal/httpapi/middleware_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/middleware_test.go), [`internal/httpapi/logging_internal_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/logging_internal_test.go), and [`docs/logging-requirements.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/logging-requirements.md) | Server SHA `e609ff8`; checked 2026-07-10 | Request-path redaction and categorized internal/startup errors | Inspected; relevant tests passed | Source review cannot prove every deployment log sink | Non-findings |
| R-UPLOAD-HANDLER | Upload orchestration: [`internal/httpapi/chunk_handlers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/chunk_handlers.go) | Server SHA `e609ff8`; checked 2026-07-10 | Upload state transitions, storage error mapping, envelope validation | Inspected; happy and retry stack paths passed | Some exceptional paths remain unexecuted | F-006; non-findings |
| R-POSTGRES-CHUNKS / R-UPLOAD-OPERATIONS | PostgreSQL upload persistence: [`internal/postgresdb/chunks.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/chunks.go), [`internal/postgresdb/upload_operations.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/upload_operations.go) | Server SHA `e609ff8`; checked 2026-07-10 | Durable upload state and transaction behavior | Inspected; live PostgreSQL package tests passed | Some exceptional paths remain unexecuted | Boundaries |
| R-UPLOAD-DESIGN | [`docs/cluster-safe-upload-semantics.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/cluster-safe-upload-semantics.md) | Server SHA `e609ff8`; checked 2026-07-10 | Current and deferred cluster-safe upload controls | Inspected | Planning text does not implement deferred controls | Boundaries |
| R-LOCAL-STORAGE / R-LOCAL-STORAGE-TESTS | Local immutable commit implementation and tests: [`internal/storage/committed_blobs.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/storage/committed_blobs.go) and [`internal/storage/storage_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/storage/storage_test.go) | Server SHA `e609ff8`; checked 2026-07-10 | Server-controlled paths and local no-overwrite commit behavior | Inspected; tests passed | Tests cover the ordinary local no-overwrite path only | Non-findings |
| R-S3 / R-S3-TESTS | S3 backend: [`internal/storage/s3.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/storage/s3.go), [`internal/storage/s3_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/storage/s3_test.go), [`internal/httpapi/s3_deletion_smoke_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/s3_deletion_smoke_test.go) | Server SHA `e609ff8` | Conditional writes, existence mapping, startup check capability, deletion smoke | Inspected; live MinIO stack and deletion smoke passed | No real 409 collision or broken-bucket startup test | F-006, F-007 |
| R-POSTGRES-WEBAUTHN / R-WEBAUTHN-HANDLERS | [`internal/postgresdb/webauthn_second_factors.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/webauthn_second_factors.go), [`internal/httpapi/webauthn_second_factor_handlers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/webauthn_second_factor_handlers.go) | Server SHA `e609ff8` | PostgreSQL get-or-create concurrency and HTTP error mapping | Inspected | No concurrent WebAuthn live test exists or was added | F-005 |
| R-PQ-ENVELOPE / R-PQ-TESTS / R-PQ-DOC | Accepted PQ frame: [`internal/envelope/pq/envelope.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/envelope/pq/envelope.go), [`internal/envelope/pq/envelope_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/envelope/pq/envelope_test.go), [`docs/post-quantum-envelope.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/post-quantum-envelope.md) | Server SHA `e609ff8`; checked 2026-07-10 | Ciphertext structural validation and current/future PQ boundary | Inspected; package and simulator tests passed | Structural validation is not ciphertext authentication | Boundaries |
| R-SHARING-HTTP / R-SHARING-HTTP-TESTS | Sharing and wrapped-key HTTP surfaces: [`internal/httpapi/account_recipient_key_handlers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/account_recipient_key_handlers.go), [`internal/httpapi/trusted_contact_relationship_handlers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/trusted_contact_relationship_handlers.go), [`internal/httpapi/sharing_handlers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/sharing_handlers.go), [`internal/httpapi/wrapped_key_handlers.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/wrapped_key_handlers.go), and their corresponding [`account-recipient-key`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/account_recipient_key_handlers_test.go), [`relationship`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/trusted_contact_relationship_handlers_test.go), [`grant`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/sharing_handlers_test.go), and [`wrapped-key`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/wrapped_key_handlers_test.go) tests | Server SHA `e609ff8`; checked 2026-07-10 | Owner/recipient scoping and active relationship/key/grant/record filtering | Inspected; HTTP tests passed | Does not establish production key custody or decrypt UX | Non-findings |
| R-SHARING-REPOSITORIES / R-SHARING-REPOSITORY-TESTS | SQLite/PostgreSQL sharing persistence: SQLite [`account recipient keys`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/account_recipient_keys.go), [`relationships`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/trusted_contact_relationships.go), [`grants`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/sharing.go), and [`wrapped keys`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/wrapped_keys.go); PostgreSQL [`account recipient keys`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/account_recipient_keys.go), [`relationships`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/trusted_contact_relationships.go), [`grants`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/sharing.go), and [`wrapped keys`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/wrapped_keys.go); plus [`SQLite contract tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/repository_test.go) and [`PostgreSQL contract tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/postgresdb_test.go) | Server SHA `e609ff8`; checked 2026-07-10 | Backend authorization/filter parity and contract coverage | Inspected; ordinary SQLite and live PostgreSQL tests passed | Tests demonstrate the repository contracts, not a production decrypt client | Non-findings |
| R-DELETION-HTTP / R-DELETION-REPOSITORIES / R-RETENTION-WORKER | Deletion and retention implementation/tests: [`HTTP handlers/tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/deletion_handlers.go), [`HTTP authorization tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/deletion_handlers_test.go); SQLite [`deletion`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/deletion.go) and [`retention`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/retention_pruning.go); PostgreSQL [`deletion`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/deletion.go) and [`retention`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/retention_pruning.go); [`worker`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/retention/deletion_worker.go), [`worker tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/retention/deletion_worker_test.go), [`SQLite contract tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/repository_test.go), and [`PostgreSQL contract tests`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/postgresdb_test.go) | Server SHA `e609ff8`; checked 2026-07-10 | Owner/admin authorization, tombstones, retry handling, and access-metadata pruning | Inspected; SQLite/HTTP/worker and live PostgreSQL tests passed | Does not establish deployed lifecycle behavior | Non-findings |
| R-CI / R-RELEASE-PROMPT / R-DEVELOPMENT | Release controls in tree: [`.github/workflows/ci.yml`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/.github/workflows/ci.yml), [`codex/prompts/90-release-check.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/codex/prompts/90-release-check.md), [`docs/development.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/development.md) | Server SHA `e609ff8` | Jobs, dependencies, release checklist, documented required signals | Inspected and compared with live Actions/rulesets | Workflow text alone cannot prove live settings | F-002, F-009 |
| R-GO-TOOLCHAIN / R-DOCKERFILE / R-DOCKERFILE-INGRESS / R-GETTING-STARTED | [`go.mod`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/go.mod), [`Dockerfile`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/Dockerfile), [`Dockerfile.ingress`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/Dockerfile.ingress), [`README.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/README.md), [`docs/getting-started.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/getting-started.md), and [`docs/post-quantum-envelope.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/post-quantum-envelope.md) | Server SHA `e609ff8`; checked 2026-07-10 | Selected Go version, builder images, and user-facing version references | Inspected; shared builder digest executed and reported Go 1.26.4 | Image execution checked Linux/amd64 only | F-002 |
| R-DEPLOYMENT | [`docs/deployment.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/deployment.md) | Server SHA `e609ff8` | Deployment boundaries and operational warnings | Inspected | No real deployment was exercised | Boundaries |
| R-COMPOSE / R-COMPOSE-PG / R-COMPOSE-S3 / R-COMPOSE-SMOKE / R-COMPOSE-RELAY | [`compose/compose-full.yml`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/compose-full.yml), [`compose/compose-postgresql-local.yml`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/compose-postgresql-local.yml), [`compose/compose-sqlite-s3.yml`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/compose-sqlite-s3.yml), [`compose/smoke-test.sh`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/smoke-test.sh), [`compose/compose-relay-sqlite-local.yml`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/compose-relay-sqlite-local.yml) | Server SHA `e609ff8`; checked 2026-07-10 | Optional-backend stacks, smoke orchestration, relay smoke | Inspected and executed for full/retry/relay variants | Tag-only backend service selection is not reproducible over time | F-009, F-012; validation |
| R-V1-CHECKLIST / R-CLUSTER-BACKUP | [`docs/v1-preview-readiness-checklist.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/v1-preview-readiness-checklist.md), [`docs/cluster-backup-restore-runbook.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/cluster-backup-restore-runbook.md), [`docs/v1-preview-direction.md`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/v1-preview-direction.md) | Server SHA `e609ff8` | Preview gate, optional-backend operational limits, known future work | Inspected; linked issue state sampled live | No backup/restore drill executed | Release decision; follow-ups |
| R-EVIDENCE-BUNDLE / R-SQLITE-DB / R-POSTGRES-MIGRATIONS / R-COORDINATION | [`internal/evidencebundle/bundle.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/evidencebundle/bundle.go), [`internal/db/db.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/db/db.go), [`internal/postgresdb/migrate.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/migrate.go), [`internal/coordination/coordinator.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/coordination/coordinator.go) | Server SHA `e609ff8`; checked 2026-07-10 | Bundle integrity, SQLite opening, migrations, Valkey leases | Inspected; relevant tests and stack passed | No backup, load, or lease-expiry campaign | Non-findings; follow-ups |
| R-BUNDLE-HTTP / R-BUNDLE-HTTP-TESTS | Bundle HTTP construction and tests: [`internal/httpapi/bundles.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/bundles.go), [`internal/httpapi/bundle_zip.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/bundle_zip.go), and [`internal/httpapi/bundles_test.go`](https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/bundles_test.go) | Server SHA `e609ff8`; checked 2026-07-10 | Pre-response verification, controlled ZIP entry names/headers, authorization, and ciphertext-only output | Inspected; tests and full-stack bundle flow passed | No production proxy or large-bundle load test | Non-findings |
| R-WEBSITE-GOV / R-WEBSITE-BASELINE | Website sources: [`governance-and-political-alignment.md`](https://github.com/open-proofline/website/blob/1366ea4086db5668ad821b364b31ae25221866aa/docs/governance-and-political-alignment.md), [`repository-readme-baseline.md`](https://github.com/open-proofline/website/blob/1366ea4086db5668ad821b364b31ae25221866aa/docs/repository-readme-baseline.md) | Website `main` SHA `1366ea4`; checked 2026-07-10 | Canonical governance posture, public voice, source mapping | Inspected at requested latest default-branch commit | Companion source; does not prove server behavior | Executive summary; non-findings |
| R-WEBCLIENT-README / R-WEBCLIENT-SECURITY | Web-client sources: [`README.md`](https://github.com/open-proofline/web-client/blob/ac006f6dd0d88db361a92a92dc3a0557f715b215/README.md), [`docs/security-model.md`](https://github.com/open-proofline/web-client/blob/ac006f6dd0d88db361a92a92dc3a0557f715b215/docs/security-model.md), [`docs/api-client.md`](https://github.com/open-proofline/web-client/blob/ac006f6dd0d88db361a92a92dc3a0557f715b215/docs/api-client.md) | Web-client `develop` SHA `ac006f6`; checked 2026-07-10 | Latest prototype scope and client/server boundary | Inspected at requested latest default-branch commit | Companion behavior is not server implementation | Executive summary; non-findings |
| I-PR-377 | Existing GitHub work: [PR #377](https://github.com/open-proofline/server/pull/377), checked head [`3f7a10804b7a5eb3a032c512ede85ee40c90e3c4`](https://github.com/open-proofline/server/commit/3f7a10804b7a5eb3a032c512ede85ee40c90e3c4), and immutable vulnerability jobs [86175757215](https://github.com/open-proofline/server/actions/runs/29034519007/job/86175757215) and [86175748553](https://github.com/open-proofline/server/actions/runs/29034516394/job/86175748553) | Open; refreshed 2026-07-11 | Duplicate/partial-remediation check for Go builder update | Inspected read-only; both cited vulnerability jobs failed at the checked head | The PR can change after this dated snapshot | F-002 |

### External authoritative sources consulted

| Source ID | Source type and location | Date checked | Purpose | Status | Limitations | Related findings/sections |
|---|---|---|---|---|---|---|
| S-GO-RELEASE | Go release history, [Go 1.26 minor revisions](https://go.dev/doc/devel/release) | 2026-07-10 | Confirm Go 1.26.5 release date and security-fix classes | Consulted | Does not establish Proofline exploitability | F-002 |
| S-GO-VULN-5856 | Go vulnerability database, [`GO-2026-5856`](https://vuln.go.dev/ID/GO-2026-5856.json) | 2026-07-10 | Advisory affected and fixed versions | Consulted | Product applicability, traces, and exploit discussion are withheld for private triage | F-002 |
| S-AWS-PUTOBJECT | AWS S3 [`PutObject`](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html) and [conditional writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html) | 2026-07-10 | `If-None-Match`, 412, and retryable 409 semantics | Consulted | AWS semantics; other S3-compatible providers may differ | F-006 |
| S-AWS-HEADBUCKET | AWS S3 [`HeadBucket`](https://docs.aws.amazon.com/AmazonS3/latest/API/API_HeadBucket.html) | 2026-07-10 | Bucket existence/access check semantics | Consulted | Provider-specific compatibility can differ | F-007 |
| S-POSTGRES-PROTOCOL | PostgreSQL [frontend/backend message flow](https://www.postgresql.org/docs/current/protocol-flow.html) | 2026-07-10 | Transaction and error-recovery semantics | Consulted | Does not execute application code | F-005 |
| S-POSTGRES-ERRORS / S-POSTGRES-UNIQUE | PostgreSQL [error codes](https://www.postgresql.org/docs/current/errcodes-appendix.html) and [unique-index behavior](https://www.postgresql.org/docs/current/index-unique-checks.html) | 2026-07-10 | Transaction-result and uniqueness/concurrency semantics | Consulted | Repository uses the `database/sql`/pgx abstraction | F-005 |
| S-DOCKER-DIGESTS | Docker [pull by digest](https://docs.docker.com/reference/cli/docker/image/pull/#pull-an-image-by-digest-immutable-identifier) | 2026-07-10 | Tag mutability and digest pinning | Consulted | Reproducibility guidance, not a runtime vulnerability claim | F-012 |
| S-GITHUB-RULESETS / S-GITHUB-STATUS | GitHub [ruleset required-status-check rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets) and [status checks](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/about-status-checks) | 2026-07-10 | Meaning of required checks | Consulted | Live repository configuration remains separate evidence | F-009 |
| S-GITHUB-JOBS / S-GITHUB-SECURE | GitHub Actions [job dependencies](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-jobs) and [secure use](https://docs.github.com/en/actions/reference/security/secure-use) | 2026-07-10 | `needs` gating and Actions hardening | Consulted | Does not prove this workflow ran | F-009; non-findings |
| S-VALKEY-SET | Valkey [`SET`](https://valkey.io/commands/set/) semantics | 2026-07-10 | `NX` and expiry lease behavior | Consulted | No dedicated live expiry/ACL campaign | Non-findings; follow-ups |
| S-SQLITE-FK / S-SQLITE-WAL | SQLite [foreign keys](https://sqlite.org/foreignkeys.html) and [WAL](https://sqlite.org/wal.html) | 2026-07-10 | Confirm database-open invariants | Consulted | No load benchmark | Non-findings |
| S-OWASP-LOGGING | OWASP [Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html) | 2026-07-10 | Sensitive-log boundary | Consulted | General guidance | Non-findings |

No Apple-platform or legal sources were consulted because the reviewed server tree makes no implemented Apple behavior or legal-admissibility claim, and this report makes none. Provider-specific MinIO documentation was not required for any finding; MinIO was used only as a disposable S3-compatible test service, with AWS API documentation grounding S3 semantics.

### Validation and execution evidence

All local application validation attributed to the reviewed tree ran in clean detached clones of the exact reviewed SHA, not in the maintainer's working checkout. Synthetic credential values and generated identifiers are redacted from this public report. Commands below preserve the executable and arguments; credential-bearing environment values are shown as placeholders. Phase 2 distinguishes commands it reran from Phase 1 evidence it inspected only.

| Source ID | Source type, location, and exact command/evidence | Commit/ref/date | Purpose/result | Status/limitations | Related findings/sections |
|---|---|---|---|---|---|
| V-REVIEW-TARGET | `git status --short --branch --untracked-files=all`; `git rev-parse --show-toplevel`; `git rev-parse HEAD`; `git branch --show-current`; `git rev-parse refs/heads/upgrade-codex-reports-and-repo-full-validation-security-review`; `git rev-parse refs/remotes/origin/upgrade-codex-reports-and-repo-full-validation-security-review`; `git -C '<isolated exact-SHA checkout>' status --short --branch --untracked-files=all` | Current and isolated checkouts; 2026-07-10 | Current branch, local ref, upstream ref, and isolated checkout all resolved to `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`; initial maintainer checkout was clean; isolated checkout was clean and detached | Confirms source identity, not correctness; temporary host path intentionally omitted | Scope |
| V-PHASE2-TARGET | `git status --short --branch --untracked-files=all`; `git branch --show-current`; `git rev-parse HEAD`; `git rev-parse 'upgrade-codex-reports-and-repo-full-validation-security-review^{commit}'`; `git cat-file -e 'e609ff86028c81bd149839e03d1ffc0eb2ee9e4a^{commit}'`; `git rev-parse refs/remotes/origin/upgrade-codex-reports-and-repo-full-validation-security-review` | Current Phase 2 checkout; refreshed 2026-07-11 | Branch, `HEAD`, reviewed ref, reviewed object, and upstream ref all resolved to `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`; status showed only the final report as untracked because local backlog drafts are ignored | The current checkout is the editing workspace, not runtime evidence for the reviewed tree | Scope; publication status |
| V-LOCAL-BASE | `gofmt -l ./cmd ./internal ./migrations`; `go test ./...`; `go vet ./...`; `scripts/check-markdown-links.py`; `git diff --check`; `docker build --progress=plain -t proofline-server-review:e609ff8 .` | Isolated exact SHA; generated 2026-07-10 | All passed; `gofmt -l` produced no paths; link checker passed 112 files | Main Docker image only in the direct build; relay built separately | General validation |
| V-LOCAL-FULLSTACK | `COMPOSE_PROJECT_NAME=proofline-phase1-e609ff8 compose/smoke-test.sh full`; `COMPOSE_PROJECT_NAME=proofline-phase1-retry-e609ff8 compose/smoke-test.sh full -- --chunks 5 --simulate-failure-every 2`; `COMPOSE_PROJECT_NAME=proofline-phase1-relay-e609ff8 compose/smoke-test.sh relay-sqlite-local` | Isolated exact SHA; generated 2026-07-10 | All passed. Disposable PostgreSQL, MinIO, and Valkey started; authenticated simulator/TOTP, incident, stream, upload, idempotent replay, induced retry, completion, bundle download, simulator-local decrypt verification, and relay readiness/route-boundary checks succeeded | Does not exercise private-report regressions, simultaneous WebAuthn registration, real S3 409, lease expiry, Redis-server compatibility, or backup/restore | F-005–F-007, F-009, F-012; non-findings |
| V-LOCAL-OPTIONAL | `SAFE_POSTGRES_TEST_DSN='<redacted synthetic disposable DSN>' go test ./internal/postgresdb -count=1`; `SAFE_S3_DELETION_SMOKE=1 SAFE_S3_ENDPOINT='<redacted disposable loopback endpoint>' SAFE_S3_REGION='<synthetic region>' SAFE_S3_BUCKET='<redacted disposable bucket>' SAFE_S3_PREFIX='<redacted synthetic prefix>' SAFE_S3_ACCESS_KEY_ID='<redacted synthetic access key>' SAFE_S3_SECRET_ACCESS_KEY='<redacted synthetic secret>' SAFE_S3_FORCE_PATH_STYLE=true go test ./internal/httpapi -run TestS3DeletionSmokeRemovesObjectsAndHidesViewer -count=1` | Isolated exact SHA; generated 2026-07-10 | Live PostgreSQL package tests passed in 26.772s; S3 deletion smoke passed | Only synthetic credential-bearing values and generated identifiers are redacted; no production/private service accessed | F-005–F-007; non-findings |
| V-LOCAL-EXTENDED | `go test -race ./...`; `go test -covermode=atomic -coverprofile='<temporary path>' ./...`; `go tool cover -func='<temporary coverage file>'` | Isolated exact SHA; generated 2026-07-10 | Race suite passed; aggregate statement coverage 54.6% | Coverage is not a security or release-readiness guarantee; opt-in PostgreSQL execution was a separate run and therefore appears as 0% in the aggregate profile | General validation |
| V-GOVULN-OFFICIAL | `docker run --rm -v '<isolated checkout>:/src:ro' -w /src golang:1.26.4 go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...`; `docker run --rm golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 go version` (the same digest is selected by both Dockerfiles) | Exact SHA; generated 2026-07-10 | Official Go 1.26.4 scan failed on `GO-2026-5856`; the shared builder digest reported Go 1.26.4 | Detailed traces and applicability analysis withheld for private triage; container run was Linux/amd64 | F-002 |
| V-PHASE2-GOVULN | `docker run --rm -v '<isolated checkout>:/src:ro' -w /src golang:1.26.4 sh -c 'go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...'`; `docker run --rm golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 go version` | Detached exact SHA; executed 2026-07-11 | The official Go 1.26.4 scan independently exited non-zero and reported `GO-2026-5856`; the exact builder digest independently reported `go1.26.4 linux/amd64` | The advisory scan used the official Go 1.26.4 container; the exact builder digest was used only to verify the builder version; applicability remains private | F-002 |
| V-GOVULN-LOCAL-DISTRO | `go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...`; `go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 -show verbose ./...` | Exact SHA; generated 2026-07-10 | The distribution-customized Go version string `1.26.4-X:nodwarf5` produced no reachable finding | The cause of the discrepancy was not established; this toolchain does not represent the official toolchain selected by the repository and does not clear F-002 | F-002 |
| V-CI-RUN | Public Actions [run 29081200062](https://github.com/open-proofline/server/actions/runs/29081200062) for exact SHA | Push run; 2026-07-10 08:53–08:57 UTC | Overall failure. [PostgreSQL metadata tests](https://github.com/open-proofline/server/actions/runs/29081200062/job/86324199759), [Go tests/vet](https://github.com/open-proofline/server/actions/runs/29081200062/job/86324199760), [binary build](https://github.com/open-proofline/server/actions/runs/29081200062/job/86324583020), [server image build](https://github.com/open-proofline/server/actions/runs/29081200062/job/86324583064), and [relay image build](https://github.com/open-proofline/server/actions/runs/29081200062/job/86324583082) passed. [Go vulnerability scan](https://github.com/open-proofline/server/actions/runs/29081200062/job/86324199779) failed. Tag-only attestation/upload/publish jobs were correctly skipped on the feature branch | No S3/Valkey full-stack job ran in public CI | F-002, F-009 |
| V-GITHUB-CONTROLS | Read-only commands: `gh run view 29081200062 --repo open-proofline/server --json databaseId,headSha,status,conclusion,url,workflowName,event,jobs`; `for id in 16849643 16748997 16849900 16987226; do gh api "repos/open-proofline/server/rulesets/$id"; done`; `gh api repos/open-proofline/server --jq '.security_and_analysis'`; `gh api repos/open-proofline/server/private-vulnerability-reporting --jq '.enabled'`; `gh api 'repos/open-proofline/server/dependabot/alerts?state=open&per_page=100' --jq 'length'`; `gh api repos/open-proofline/server/code-scanning/default-setup --jq '{state,query_suite,languages,updated_at}'`; `gh api repos/open-proofline/server/actions/permissions --jq '{allowed_actions}'` | Live state refreshed 2026-07-11 | The immutable exact-SHA run still had the recorded job results; `develop`, `main`, and `release/v*` still required only `Go tests`, `Build Linux binary`, and `Build Docker image`; private vulnerability reporting and Dependabot security updates were enabled; open Dependabot alerts were zero; secret scanning/push protection were disabled; CodeQL was `not-configured`; Actions permissions allowed all actions | Live settings can change; some endpoints require authenticated read access; exact rulesets remain publicly viewable as [16849643](https://github.com/open-proofline/server/rules/16849643), [16748997](https://github.com/open-proofline/server/rules/16748997), [16849900](https://github.com/open-proofline/server/rules/16849900), and [16987226](https://github.com/open-proofline/server/rules/16987226) | F-009; follow-ups |
| V-GITHUB-ISSUES | `gh issue list --repo open-proofline/server --state all --limit 300 --json number,title,state,url`; `gh pr list --repo open-proofline/server --state all --limit 300 --json number,title,state,url`; exact-title comparison against all nine sanitized recommendations | Live state checked 2026-07-11 | 143 issues with zero open; 234 pull requests with seven open; no exact issue/PR title duplicate among the nine recommendations | State can change; related closed work and open dependency PRs were reviewed as partial overlaps, not exact duplicates | Follow-ups and local draft index |
| V-COMPANION-HEADS | `gh api repos/open-proofline/website --jq '.default_branch'`; `gh api repos/open-proofline/website/commits/main --jq '.sha'`; `gh api repos/open-proofline/web-client --jq '.default_branch'`; `gh api repos/open-proofline/web-client/commits/develop --jq '.sha'`; `git -C ../website cat-file -e '1366ea4086db5668ad821b364b31ae25221866aa:docs/governance-and-political-alignment.md'`; `git -C ../website cat-file -e '1366ea4086db5668ad821b364b31ae25221866aa:docs/repository-readme-baseline.md'`; `git -C ../web-client cat-file -e 'ac006f6dd0d88db361a92a92dc3a0557f715b215:README.md'`; `git -C ../web-client cat-file -e 'ac006f6dd0d88db361a92a92dc3a0557f715b215:docs/security-model.md'`; `git -C ../web-client cat-file -e 'ac006f6dd0d88db361a92a92dc3a0557f715b215:docs/api-client.md'` | Checked 2026-07-11 | Website `main` remained `1366ea4086db5668ad821b364b31ae25221866aa`; web-client `develop` remained `ac006f6dd0d88db361a92a92dc3a0557f715b215`; all cited files existed at those commits | Companion repositories are contextual evidence only | Executive summary; boundaries |
| V-PHASE2-SOURCE-READS | Commit-addressed reviewed-tree source inspection plus current publication-guidance, full-file v1-direction/PQ-envelope, complete Phase 1 draft, future-design, report-policy, and companion-source inspection | Exact SHA/current guidance; 2026-07-11 | Every source required by prompt 95 was read before publication editing | Source review does not substitute for runtime validation | Phase 2 method; implementation boundaries |
| V-PHASE2-RECHECK | Disposable clone at exact SHA: `gofmt -l ./cmd ./internal ./migrations`; `scripts/check-markdown-links.py`; `go test ./...`; `go vet ./...`; `git diff --check`; initial/final `git status --short --branch --untracked-files=all` | Detached exact SHA; executed 2026-07-11 | All passed; formatting produced no paths; link checker passed 112 files; checkout remained clean | Phase 2 did not repeat race, coverage, Docker builds, or optional-backend stacks | Executive summary; Phase 2 validation |
| V-PHASE2-PUBLICATION | `scripts/check-markdown-links.py`; `scripts/check-markdown-links.py .backlog-drafts/2026-07-11/upgrade-codex-reports-and-repo-full-validation-security-review`; `git diff --check`; per-file `git diff --no-index --check /dev/null <generated-markdown>`; `git status --short --branch --untracked-files=all`; portable-key used/defined comparison; commit-addressed `git cat-file -e` checks for cited server/companion paths; public-safety and private-mechanics phrase scans; required draft-section/label checks | Current Phase 2 artifacts; executed 2026-07-11 | Passed: 113 repository Markdown files, 11 draft Markdown files, 12 per-file whitespace checks, 112 used/defined portable citation keys, 108 server paths, five companion paths, nine issue-draft schemas, verified labels, and disclosure scans; status showed only the final report as untracked | The local follow-up drafts are ignored; this row records validation, not remediation | Publication status; handoff |
| V-PHASE2-EXTERNAL | Independent browser opens of every non-GitHub authoritative URL used by the public report, with targeted checks of each material public claim | Checked 2026-07-11 | All URLs resolved; material public claims remained supported | Moving documentation was time-boxed to the validation date; product applicability of the Go advisory remains privately triaged | F-002, F-005, F-006, F-009, F-012; non-findings |
| V-MAINTAINER-CONTEXT | Maintainer statement in the Phase 2 task: there are no known public or private Proofline server deployments and private vulnerabilities can generally be treated as lower disclosure sensitivity in that context | 2026-07-11 | No known deployed-user exposure; used to calibrate disclosure context, not technical severity or release impact | This is maintainer-supplied deployment context, not an independent infrastructure inventory | Executive summary; private handling |
| V-PHASE2-CLEANUP | Disposable-artifact inventory covering Docker containers, networks, volumes, images, and top-level temporary entries filtered for Phase 1/2 review names | Checked 2026-07-11 | Four citation-lane scratch files were removed; the repeated inventory then found no matching disposable container, network, volume, review image, temporary checkout, or scratch file | Name-based cleanup verification only | Generated artifacts; handoff |
| V-ULTRA-REVIEW | Independent Phase 1 and Phase 2 source, backend, citation, execution-evidence, and disclosure-safety review lanes | Exact SHA; 2026-07-10 to 2026-07-11 | Findings and publication wording were independently reconciled against sources and validation evidence | AI-assisted multi-lane review is not an independent human audit | Scope and method |
| V-TOOLS | Local shell, Git, Docker/Compose, Go tooling, SQLite CLI, read-only GitHub CLI/API, public web search/open, and authoritative public documentation | 2026-07-10 | Available and used | No private/production service, credential, endpoint, or user data was accessed | Source registry |

### Sources, checks, and commands not available or not executed

| Source/check ID | Source type and location | Commit/ref/date | Purpose | Status | Limitations | Related findings/sections |
|---|---|---|---|---|---|---|
| V-NO-PRODUCTION | Deployment/runtime check; private or production services | Reviewed SHA; 2026-07-10 | Determine whether deployment-specific controls were directly verified | Not executed | No real account, evidence, token, private endpoint, managed backend, or production credential was accessed; TLS, firewall, proxy, WAF, monitoring, backups, and log sinks remain deployment-specific | Scope; all findings |
| V-NO-PENTEST | Security testing campaign; reviewed application | Reviewed SHA; 2026-07-10 | Bound the assurance level of the review | Not executed | No penetration test, adversarial load test, long-duration soak, fuzzing campaign, or legal/compliance review | Scope and conclusion |
| V-NO-FAULT-INJECTION | Failure/concurrency regression campaign; clean isolated checkout | Reviewed SHA; 2026-07-10 | Test exceptional paths identified by review | Not executed | Security-sensitive scenarios and detailed test designs are reserved for private reports; selected public concurrency/error cases also remain unexecuted | Private triage; F-005–F-006 |
| V-NO-BACKUP-RESTORE | Disposable PostgreSQL/S3 restore drill; optional-backend runbook | Reviewed SHA; 2026-07-10 | Validate durable metadata/blob recovery together | Not executed | The docs already require this before real optional-backend reliance; retained as a readiness boundary rather than an ordinary pre-v1 defect | Follow-ups; production boundary |
| V-NO-VALKEY-CAMPAIGN | Disposable Valkey/Redis failure campaign | Reviewed SHA; 2026-07-10 | Validate expiry, stale ownership, ACL/command restrictions, failover, and Redis-server compatibility | Partially exercised | Ordinary live Valkey coordination passed in the full stack; isolated adverse/compatibility cases did not run | F-009; follow-ups; non-findings |
| V-NO-EXTERNAL-WRITES | GitHub/external mutations | Phase 1 and Phase 2; through 2026-07-11 | Preserve review-only external scope | Deliberately not executed | No GitHub issue, private advisory, PR, release, branch/ruleset change, or security-setting change was created; Phase 2 issue drafts are ignored local files only | Follow-up recommendations |

### Generated artifacts and report outputs

| Artifact ID | Source type and location | Commit/ref/date | Purpose | Status | Limitations | Related findings/sections |
|---|---|---|---|---|---|---|
| G-PHASE1-DRAFT | Markdown draft; `.technical-review-drafts/2026-07-10-proofline-unreleased-technical-review-draft.md` | Reviewed SHA; generated 2026-07-10 | Phase 1 source-cited review input | Retained locally | Ignored pre-publication input; this Phase 2 report supersedes it for publication | All sections |
| G-PHASE2-TARGET | Final report source; `docs/reports/2026-07-10-proofline-unreleased-technical-review.md` | Reviewed SHA; generated/validated 2026-07-11 | Independent public-hardened report | Finalized by Phase 2 | Report of the reviewed SHA only; not remediation validation | All sections |
| G-TEMP-VALIDATION | Removed isolated exact-SHA checkout, temporary coverage profile, disposable containers/networks/volumes, and locally tagged review images | Reviewed SHA; generated/cleaned 2026-07-10 | Approved local validation without mutating the maintainer checkout | Generated temporarily; cleaned after evidence capture | Host path, synthetic credentials, and generated identifiers are intentionally omitted | Validation evidence |
| G-FOLLOWUPS | Sanitized branch-scoped local issue drafts under `.backlog-drafts/2026-07-11/upgrade-codex-reports-and-repo-full-validation-security-review/` | Generated 2026-07-11 | Preserve nine non-vulnerability follow-ups for maintainer review | Local drafts only; no GitHub issues created | Suspected/unresolved vulnerabilities excluded; GitHub state must be rechecked before later creation | Follow-up recommendations |

## Scope And Method

The review anchored every server claim to commit `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`. The current branch and remote tracking ref resolved to that SHA, and all checkout-dependent validation ran in a separate clean clone. The maintainer's working checkout was not reset, cleaned, switched, reformatted, or used as runtime proof. [V-REVIEW-TARGET] [V-PHASE2-TARGET] [R-REPORT-PROMPT]

Ultra mode was used because the repository-wide review materially benefited from independent read-only lanes. Phase 1 covered core HTTP/auth/security/crypto/sharing/deletion behavior; PostgreSQL/S3/Valkey/migration/storage behavior; and documentation, CI, deployment, companion repositories, and live public evidence. Phase 2 separately rechecked source support, execution evidence, citations, current-versus-future boundaries, disclosure safety, GitHub state, and issue-draft hygiene before publication. [V-ULTRA-REVIEW] [V-PHASE2-SOURCE-READS]

Review scope included the repository root documents; `cmd`, `internal`, migrations, tests, Dockerfiles, Compose stacks, GitHub workflows/settings, release prompts, deployment/runbook documents, current-versus-future product documents, the latest requested website governance/baseline sources, and the latest requested web-client default-branch sources. External claims were checked against primary or authoritative documentation, and Phase 2 refreshed every external source materially relied on by this public report. [V-TOOLS] [V-PHASE2-EXTERNAL]

Severity reflects demonstrated impact and prerequisites. Release blocking is a separate policy decision. Optional-backend findings are marked release-blocking because the maintainer explicitly set that policy in Phase 0, even where technical severity is Low or Medium. The absence of known deployments reduces present exposure but does not remove a defect or release gate. Suspected vulnerabilities use the private-reporting path required by `SECURITY.md`; this report omits exploit payloads, raw traces, sensitive identifiers, and step-by-step reproduction. [R-SECURITY] [V-MAINTAINER-CONTEXT]

The validation method intentionally combined ordinary tests with live disposable backends. Happy-path and retry-path success is evidence that the reviewed implementation works under the exercised conditions. It is not evidence that unexercised failure, concurrency, deployment-specific, backup, or security-control paths are safe. [V-LOCAL-FULLSTACK] [V-NO-FAULT-INJECTION]

## Current Implementation Summary

Proofline at the reviewed commit is an experimental Go backend. It accepts already-encrypted complete chunks through authenticated main `/v1` routes, stores metadata in SQLite by default or optional PostgreSQL, stores committed ciphertext on local disk by default or optional S3-compatible object storage, and optionally uses Valkey/Redis-compatible coordination for short-lived leases and rate-limit counters. It also provides a private-admin web/API listener, a token-scoped read-only viewer, encrypted ZIP evidence bundles, and a bounded complete-chunk stream-ingress relay with temporary optimistic encrypted fanout. The relay is subordinate to the core API and does not own durable evidence, viewer, admin, or broad main-API behavior. [R-README] [R-CMD-API] [R-CMD-SERVERS] [R-HTTPAPI-ROUTES] [R-HTTPAPI-API]

Main API/viewer and private-admin route trees are constructed as separate handlers and served on separate configured listener groups. `/admin` and `/admin/api/...` are absent from the main handler; public viewer operations are GET/read-only; the relay smoke confirms admin, main API, viewer, and metrics paths are not exposed by the relay container. Separate binds remain a deployment boundary rather than a complete security model. [R-CMD-SERVERS] [R-HTTPAPI-ROUTES] [R-HTTPAPI-API] [R-COMPOSE-RELAY] [V-LOCAL-FULLSTACK]

Uploads are bounded, staged locally, hashed over accepted bytes, envelope-checked, and committed to server-generated immutable paths or object keys before metadata insertion. Local storage uses a no-overwrite primitive; S3 writes use `If-None-Match: *`. Bundle generation requires completed contiguous streams and verifies stored chunk size and SHA-256 before serving ZIP headers/body. Completed bundles contain encrypted chunk payloads with server-controlled ZIP entry names; they are not decrypted or playable exports. Phase 1's decrypt verification was performed locally by the simulator using development/test key material, not by the server. [R-UPLOAD-HANDLER] [R-LOCAL-STORAGE] [R-LOCAL-STORAGE-TESTS] [R-S3] [R-EVIDENCE-BUNDLE] [R-BUNDLE-HTTP] [R-BUNDLE-HTTP-TESTS] [R-ENCRYPTION] [V-LOCAL-FULLSTACK]

SQLite enables foreign keys and verifies WAL mode with a constrained pool. Both SQLite and PostgreSQL migration paths checksum applied migrations and execute migration steps transactionally; PostgreSQL additionally serializes migration application with an advisory lock. PostgreSQL contract tests cover the principal metadata repository behavior. [R-SQLITE-DB] [R-POSTGRES-MIGRATIONS] [S-SQLITE-FK] [S-SQLITE-WAL] [V-LOCAL-OPTIONAL]

Valkey coordination uses random ownership tokens, expiries, `SET NX`, and a token-matched compare-and-delete script. Coordination is not treated as durable evidence truth, and its failures fail closed for affected operations. The approved full stack exercised ordinary live Valkey coordination but not the more adversarial campaign listed in the validation limitations. [R-COORDINATION] [S-VALKEY-SET] [V-NO-VALKEY-CAMPAIGN]

Generic incidents remain the default. Private incident create/read routes implement nullable `incident_mode`, `capture_profile`, `escalation_policy`, and `sharing_state` fields as labels only. Those labels do not grant access, notify contacts, change retention or key custody, release wrapped keys, or alter public viewer and bundle behavior. [R-INCIDENT-MODES] [R-API] [R-SECURITY-MODEL]

Account-to-account trusted-contact relationships, owner-scoped account/contact recipient-key metadata, owner-scoped sharing grants, wrapped-key storage, and authenticated grant-scoped trusted-contact wrapped-key reads are implemented and authorization-scoped in both repository backends; their HTTP and repository contract tests passed. Trusted-contact incident review and client decryption UX are not implemented. [R-SHARING-HTTP] [R-SHARING-HTTP-TESTS] [R-SHARING-REPOSITORIES] [R-SHARING-REPOSITORY-TESTS] [R-KEY-CUSTODY]

The accepted PQ profile is implemented for server upload shape/identity validation, wrapped-key public metadata and frame validation, key-free bundle profile hints, and the simulator's default reference encryption/decrypt-verification flow. These are structural/public-metadata checks and development reference behavior; they are not server-side ciphertext authentication, backend decryption, production key custody, browser decryption, or cross-repository v1-preview conformance. [R-PQ-ENVELOPE] [R-PQ-TESTS] [R-PQ-DOC] [R-ENCRYPTION] [R-V1-DIRECTION] [R-V1-CHECKLIST] [R-WEBCLIENT-README]

The latest website and web-client sources remain aligned with the server's conservative framing. Governance is maintainer-led and intended to develop toward mission-locked public-good stewardship; the web client is an experimental account/incident-metadata prototype, mock-first and local-live-test oriented, not a recorder, decryption service, emergency workflow, or production account portal. [R-WEBSITE-GOV] [R-WEBSITE-BASELINE] [R-WEBCLIENT-README] [R-WEBCLIENT-SECURITY] [V-COMPANION-HEADS]

## Findings

### F-001: Private vulnerability finding

- **Severity:** High
- **Confidence:** High
- **Reviewed context:** `upgrade-codex-reports-and-repo-full-validation-security-review` at `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`
- **Release impact:** Release-blocking
- **Follow-up handling:** GitHub private vulnerability reporting
- **Suggested public issue title:** None before private triage

Details, affected components, reproduction, remediation design, and regression criteria are withheld pending private triage. The maintainer reports no known deployments, so no deployed-user exposure is known; this does not change release impact. [R-SECURITY] [V-MAINTAINER-CONTEXT]

### F-002: The reviewed Go toolchain fails the public vulnerability gate

- **Severity:** Medium
- **Confidence:** High for affected/fixed versions and the failed gate; product applicability reserved for private triage
- **Current implementation vs future design:** Current build and CI configuration
- **Affected files/functions:** `go.mod`; server and relay builder stages; Go-version documentation; CI `govulncheck`
- **Reviewed context:** Exact reviewed SHA and public Actions run for that SHA
- **Release impact:** Release-blocking security patch and failed required review evidence
- **Follow-up handling:** GitHub private vulnerability reporting for applicability; sanitized release note after remediation
- **Suggested public issue title:** None while applicability is privately assessed

The repository selects Go 1.26.4. Both Dockerfiles reference the same inspected builder image, both Docker build paths passed, and that shared image reported Go 1.26.4. The exact-SHA public `Go vulnerability scan` and clean Phase 1 and Phase 2 official Go 1.26.4 scans failed on `GO-2026-5856`; the Go project lists Go 1.26.5 as the security-fix release. Product-specific applicability is withheld for private triage. [R-GO-TOOLCHAIN] [R-DOCKERFILE] [R-DOCKERFILE-INGRESS] [R-GETTING-STARTED] [R-PQ-DOC] [R-CI] [V-GOVULN-OFFICIAL] [V-PHASE2-GOVULN] [V-CI-RUN] [S-GO-RELEASE] [S-GO-VULN-5856]

The local distribution-customized toolchain produced no reachable finding, but the cause of that discrepancy was not established and it is not the official toolchain selected by `go.mod` or used by the pinned builders. It cannot clear the official scan or CI failure. PR #377 was refreshed on 2026-07-11 at head `3f7a10804b7a5eb3a032c512ede85ee40c90e3c4`; it changes only the builder digests, and its two immutable vulnerability jobs still failed because `go.mod` remained on 1.26.4. [V-GOVULN-LOCAL-DISTRO] [I-PR-377]

Why it matters: the reviewed SHA fails its public security gate and its selected official build toolchain predates the advisory's fixed release.

Minimal actionable fix: update the Go directive/toolchain, both builder digests, badges and version references to a fixed release; reconcile or supersede PR #377; then rerun the official vulnerability scan, full Go validation, both image builds, and approved optional-backend stack.

Acceptance criteria:

1. The repository and both builder images use a fixed official Go release consistently.
2. `govulncheck@v1.3.0 ./...` passes under the selected official toolchain.
3. Public Actions pass for the remediation SHA, including both image builds and PostgreSQL integration.
4. The full disposable PostgreSQL/S3/Valkey stack and retry variant pass on the remediation SHA.
5. Any security applicability statement is approved through private triage and does not speculate beyond evidence.

### F-003: Private vulnerability finding

- **Severity:** Medium
- **Confidence:** High
- **Reviewed context:** `upgrade-codex-reports-and-repo-full-validation-security-review` at `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`
- **Release impact:** Release-blocking
- **Follow-up handling:** GitHub private vulnerability reporting
- **Suggested public issue title:** None before private triage

Details, affected components, reproduction, remediation design, and regression criteria are withheld pending private triage. The maintainer reports no known deployments, so no deployed-user exposure is known; this does not change release impact. [R-SECURITY] [V-MAINTAINER-CONTEXT]

### F-004: Private vulnerability finding

- **Severity:** Medium
- **Confidence:** High
- **Reviewed context:** `upgrade-codex-reports-and-repo-full-validation-security-review` at `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`
- **Release impact:** Release-blocking
- **Follow-up handling:** GitHub private vulnerability reporting
- **Suggested public issue title:** None before private triage

Details, affected components, reproduction, remediation design, and regression criteria are withheld pending private triage. The maintainer reports no known deployments, so no deployed-user exposure is known; this does not change release impact. [R-SECURITY] [V-MAINTAINER-CONTEXT]

### F-005: PostgreSQL WebAuthn user get-or-create is not concurrency-safe

- **Severity:** Low
- **Confidence:** High
- **Current implementation vs future design:** Current optional PostgreSQL backend
- **Affected files/functions:** `postgresdb.Repository.GetOrCreateWebAuthnUser`; WebAuthn registration/verification start handlers
- **Reviewed context:** Exact reviewed SHA
- **Release impact:** Release-blocking under the optional-backend policy
- **Follow-up handling:** Sanitized public non-security bug after duplicate check
- **Suggested public issue title:** `Make PostgreSQL WebAuthn get-or-create concurrency safe`

Concurrent requests can both observe no WebAuthn user. After the losing insert receives a uniqueness error, the code immediately tries to select the winner inside the same PostgreSQL transaction. PostgreSQL leaves an explicit transaction failed after the statement error until rollback/recovery, so that re-read cannot implement the intended conflict recovery. The handler can consequently return a transient false not-found/error during otherwise valid concurrent setup. [R-POSTGRES-WEBAUTHN] [R-WEBAUTHN-HANDLERS] [S-POSTGRES-PROTOCOL] [S-POSTGRES-ERRORS] [S-POSTGRES-UNIQUE]

Why it matters: this is a transient availability/correctness defect, not an authentication bypass. It is release-blocking because it affects the optional PostgreSQL path and no concurrent live-PostgreSQL test covers it.

Minimal actionable fix: use one concurrency-safe PostgreSQL statement pattern, such as `INSERT ... ON CONFLICT DO NOTHING` followed by a select in a usable transaction, or rollback and retry on a fresh transaction. Review sibling email/TOTP enrollment conflict recovery for the same anti-pattern.

Acceptance criteria:

1. A live PostgreSQL test starts concurrent get-or-create calls for one account/RP and every successful caller receives the same durable user handle.
2. No expected uniqueness race maps to `account_not_found` or a 500 response.
3. Other constraint failures remain distinguishable internally but safely mapped externally.
4. SQLite behavior and public/private route boundaries remain unchanged.

### F-006: S3 conditional-write conflict is misclassified as an existing immutable object

- **Severity:** Medium
- **Confidence:** High
- **Current implementation vs future design:** Current optional S3-compatible backend
- **Affected files/functions:** `isS3AlreadyExists`, `S3Store.CommitTemp`, upload duplicate mapping, S3 unit tests
- **Reviewed context:** Exact reviewed SHA
- **Release impact:** Release-blocking under the optional-backend policy
- **Follow-up handling:** Sanitized public non-security bug after duplicate check
- **Suggested public issue title:** `Retry S3 ConditionalRequestConflict instead of reporting duplicate chunk`

The S3 helper maps both `412 PreconditionFailed` and `409 ConditionalRequestConflict`—including any raw HTTP 409—to `ErrAlreadyExists`. AWS specifies that 412 means the `If-None-Match` precondition failed because the key exists, while a 409 conditional conflict should be retried. The HTTP layer consequently reports a permanent `duplicate_chunk` for a condition that does not establish equivalent committed metadata. Tests cover 412 but not the 409 distinction. [R-S3] [R-S3-TESTS] [R-UPLOAD-HANDLER] [S-AWS-PUTOBJECT]

Why it matters: an ordinary transient storage conflict can be presented to the client as a permanent evidence duplicate, preventing the intended retry behavior.

Minimal actionable fix: separate precondition failure from conflicting-operation failure. Apply a bounded retry at the storage layer or surface a safe retryable storage-unavailable result; do not convert an undifferentiated 409 into duplicate evidence.

Acceptance criteria:

1. Smithy error-code and raw HTTP tests cover 412 and 409 separately.
2. 412 preserves immutable duplicate behavior.
3. 409 is retried within a documented bound or returned as a retryable operational failure, never a false duplicate solely from status code.
4. A disposable S3-compatible test exercises the application mapping without logging object keys or request details.

### F-007: The configured S3 backend is not checked before the API begins serving

- **Severity:** Medium
- **Confidence:** High
- **Current implementation vs future design:** Current optional S3-compatible startup path
- **Affected files/functions:** `run`, `newBlobStore`, `S3Store.Check`
- **Reviewed context:** Exact reviewed SHA
- **Release impact:** Release-blocking under the optional-backend policy
- **Follow-up handling:** Sanitized public operational bug
- **Suggested public issue title:** `Check the configured blob backend before API startup`

Startup checks the optional coordination backend and opens/pings the metadata backend. It constructs the blob store but does not call its `Check` method before starting HTTP listeners. For S3, construction validates configuration shape and local temp-directory setup; the existing `Check` method is what calls `HeadBucket` to establish bucket reachability/access. A bad endpoint, unavailable bucket, or insufficient bucket permission can therefore remain undiscovered until a storage operation or a later private admin status request. [R-CMD-API] [R-S3] [S-AWS-HEADBUCKET]

The approved full stack passed because Compose waited for MinIO initialization. It does not test fail-closed startup with a broken configured bucket. [R-COMPOSE] [V-LOCAL-FULLSTACK]

Why it matters: a process can appear started while its configured evidence backend is unusable. This increases the chance that availability failure is first discovered during an upload or operator action.

Minimal actionable fix: invoke the configured blob store's safe check during startup and report only the existing sanitized startup stage/category. If deployments intentionally need delayed object-store readiness, document and implement an explicit reviewed mode rather than silently skipping validation.

Acceptance criteria:

1. Configured S3 startup fails closed when the bucket is unreachable or inaccessible.
2. Local storage startup behavior remains unchanged and checked.
3. Startup errors contain no endpoint, bucket, credential, object key, stored path, or raw SDK error.
4. Disposable success and failure tests cover both local and S3 backends.

### F-008: Private vulnerability finding

- **Severity:** Low
- **Confidence:** High
- **Reviewed context:** `upgrade-codex-reports-and-repo-full-validation-security-review` at `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`
- **Release impact:** Release-blocking
- **Follow-up handling:** GitHub private vulnerability reporting
- **Suggested public issue title:** None before private triage

Details, affected components, reproduction, remediation design, and regression criteria are withheld pending private triage. The maintainer reports no known deployments, so no deployed-user exposure is known; this does not change release impact. [R-SECURITY] [V-MAINTAINER-CONTEXT]

### F-009: Branch and publication controls do not enforce all release-blocking checks

- **Severity:** Medium
- **Confidence:** High
- **Current implementation vs future design:** Current workflow, release prompt, and live GitHub rulesets
- **Affected files/settings:** `.github/workflows/ci.yml`, release-check prompt, development docs, `develop`/`main`/`release/v*` rulesets
- **Reviewed context:** Exact reviewed SHA plus live settings refreshed on 2026-07-11
- **Release impact:** Release-blocking release-assurance gap
- **Follow-up handling:** Sanitized public CI/release-control issue
- **Suggested public issue title:** `Gate protected branches and release publishing on security and optional-backend validation`

The active protected-branch rulesets require only `Go tests`, `Build Linux binary`, and `Build Docker image`. They do not require `Go vulnerability scan`, `PostgreSQL metadata tests`, or `Build stream-ingress Docker image`. The workflow's image publication and binary attestation paths depend on tests, vulnerability scanning, and selected builds, but not PostgreSQL integration. Public CI contains no complete PostgreSQL/MinIO/Valkey job. The release prompt's standard command list likewise omits the vulnerability scan and approved optional-backend stack. [R-CI] [R-RELEASE-PROMPT] [R-DEVELOPMENT] [R-COMPOSE-SMOKE] [V-GITHUB-CONTROLS] [S-GITHUB-RULESETS] [S-GITHUB-JOBS]

The exact-SHA run illustrates the distinction: the overall workflow failed on vulnerability scanning while the three branch-required checks passed. GitHub documents that only configured required checks must pass for the protected-ref control. [V-CI-RUN] [S-GITHUB-RULESETS] [S-GITHUB-STATUS]

Why it matters: merge controls can accept a commit without security, PostgreSQL, or relay checks being required, and release artifact paths can proceed without PostgreSQL evidence. S3 and Valkey have no public CI gate at all. This contradicts the maintainer's decision that optional-backend findings are release-blocking.

Minimal actionable fix: add stable release-blocking jobs for the full optional-backend combination, include the vulnerability/PostgreSQL/relay/full-stack signals in publication dependencies, update the release prompt/docs, and require the emitted check names in the relevant live rulesets.

Acceptance criteria:

1. `develop`, `main`, and `release/v*` require vulnerability, PostgreSQL, relay-image, and full-stack optional-backend checks where applicable.
2. Binary attestation/upload and both image publications depend on every release-blocking job.
3. CI exercises disposable PostgreSQL, S3-compatible storage, and Valkey without real secrets or private services.
4. The release prompt explicitly includes `govulncheck` and the selected full-stack evidence.
5. Live rulesets are re-read after the new check names have emitted successfully.

### F-010: Private vulnerability finding

- **Severity:** Low
- **Confidence:** High
- **Reviewed context:** `upgrade-codex-reports-and-repo-full-validation-security-review` at `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`
- **Release impact:** Private triage required; not independently release-blocking at demonstrated impact
- **Follow-up handling:** GitHub private vulnerability reporting
- **Suggested public issue title:** None before private triage

Details, affected components, reproduction, remediation design, and regression criteria are withheld pending private triage. The maintainer reports no known deployments, so no deployed-user exposure is known; this does not change release impact. [R-SECURITY] [V-MAINTAINER-CONTEXT]

### F-011: Private vulnerability finding

- **Severity:** Low
- **Confidence:** High
- **Reviewed context:** `upgrade-codex-reports-and-repo-full-validation-security-review` at `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`
- **Release impact:** Private triage required; not independently release-blocking at demonstrated impact
- **Follow-up handling:** GitHub private vulnerability reporting
- **Suggested public issue title:** None before private triage

Details, affected components, reproduction, remediation design, and regression criteria are withheld pending private triage. The maintainer reports no known deployments, so no deployed-user exposure is known; this does not change release impact. [R-SECURITY] [V-MAINTAINER-CONTEXT]

### F-012: Optional-backend smoke images are selected by mutable tags

- **Severity:** Low
- **Confidence:** High
- **Current implementation vs future design:** Current release-validation Compose stacks
- **Affected files:** `compose/compose-full.yml`, `compose/compose-postgresql-local.yml`, and `compose/compose-sqlite-s3.yml`
- **Reviewed context:** Exact reviewed SHA
- **Release impact:** Release-blocking under the maintainer's optional-backend policy; reproducibility gap, not a runtime vulnerability
- **Follow-up handling:** Sanitized public supply-chain/testing issue
- **Suggested public issue title:** `Pin disposable PostgreSQL, MinIO, and Valkey smoke images by digest`

The full stack selects PostgreSQL and Valkey by version tag and MinIO server/client by `latest`; the focused PostgreSQL and S3 variants also use tag-only backend images. Docker documents that a tag can resolve to updated contents, while a digest identifies a fixed image. The exact reviewed source can therefore exercise different database, object-store, or coordination bytes on a later rerun. [R-COMPOSE] [R-COMPOSE-PG] [R-COMPOSE-S3] [S-DOCKER-DIGESTS]

Why it matters: the stack passed during this review, so this is not evidence that the tested versions are broken. It is a reproducibility and reviewability gap for evidence that the maintainer designated release-blocking.

Minimal actionable fix: retain readable tags but add manifest digests for every smoke service, remove `latest`, and document/automate reviewed refreshes.

Acceptance criteria:

1. PostgreSQL, MinIO server, MinIO client, and Valkey images in every release smoke variant use explicit digests.
2. A documented refresh command or dependency automation updates tags and digests together.
3. Full, retry, relay, and S3-deletion validation pass after pinning.
4. No production credentials or provider-specific private endpoints are introduced.

## Non-Findings And Confirmed Boundaries

- **Listener and route separation held.** Main API/viewer and private-admin trees are separate handlers, muxes, listener groups, and server objects. Admin write routes are absent from the main mux; viewer routes remain read-only. The relay packaging smoke also confirmed that admin, main API, viewer, and metrics routes are absent from the relay listener. [R-CMD-API] [R-CMD-SERVERS] [R-HTTPAPI-ROUTES] [R-HTTPAPI-API] [R-COMPOSE-RELAY] [V-LOCAL-FULLSTACK]

- **Token and browser-session handling was not promoted as a finding.** Session and viewer credentials use high-entropy random values, stored hashes, expiry/revocation checks, generic failure shapes, and constant-time final comparison where applicable. Browser-cookie support is disabled by default and remains future web-client plumbing, not public `/v1` deployment approval. When explicitly enabled for a non-local main listener, the preferred cookie configuration is Secure, HttpOnly, and `__Host-` constrained; documented loopback-local configuration and the separate path-scoped admin cookie are explicit exceptions. Cookie mode uses session-bound CSRF and exact credentialed-CORS allowlisting, and bearer-plus-cookie ambiguity is rejected. [R-HTTPAPI-API] [R-AUTH-HANDLERS] [R-AUTH-MIDDLEWARE] [R-AUTH-REPOSITORIES] [R-AUTH-POSTGRES] [R-WEB-AUTH-CONTROLS] [R-WEB-AUTH-TESTS] [R-VIEWER-TOKENS] [R-VIEWER-TOKEN-SQLITE] [R-VIEWER-TOKEN-POSTGRES] [R-VIEWER-TOKEN-TESTS]

- **Sensitive application logging controls were materially aligned.** The inspected request, internal-error, panic, viewer-path, and startup logging implementations use safe route patterns/redaction and controlled error categories; their tests did not emit the representative sensitive values under review. [R-HTTP-LOGGING] [R-HTTP-LOGGING-TESTS] [R-SECURITY] [S-OWASP-LOGGING]

- **Upload bounds and normal immutable-commit behavior held under exercised conditions.** Multipart/upload byte limits, local temp quota, account committed-byte quota, hash comparison, stream/media checks, server-generated paths, local no-overwrite commit, S3 `If-None-Match: *`, hashed idempotency dimensions, and Valkey lease behavior are present. F-006 does not erase the positive happy/retry evidence. [R-UPLOAD-HANDLER] [R-LOCAL-STORAGE] [R-LOCAL-STORAGE-TESTS] [R-S3] [R-COORDINATION] [V-LOCAL-FULLSTACK]

- **Evidence-bundle integrity regressions from the prior report are resolved.** Bundle construction now reopens and verifies committed chunk byte size and SHA-256 before sending response headers/body; stream contiguity and server-controlled entry names remain enforced. Completed bundles remain encrypted chunk bundles, not decrypted/playable media. [R-PRIOR-REPORT] [R-EVIDENCE-BUNDLE] [R-BUNDLE-HTTP] [R-BUNDLE-HTTP-TESTS] [R-CHANGELOG]

- **Migration and database-open invariants held.** SQLite foreign keys and WAL mode are verified; migrations use checksums and transactions; PostgreSQL migration execution uses a session advisory lock. Shared repository contract tests and the live PostgreSQL package run passed. F-005 is a narrow PostgreSQL error-recovery/concurrency defect, not a conclusion that the backend generally lacks parity. [R-SQLITE-DB] [R-POSTGRES-MIGRATIONS] [S-SQLITE-FK] [S-SQLITE-WAL] [V-LOCAL-OPTIONAL]

- **Valkey remains non-durable coordination with a sound ordinary lease pattern.** Random owner tokens, expiry, `SET NX`, and compare-and-delete release are present; failures fail closed for affected operations. Lack of a dedicated expiry/ACL/Redis-compatibility campaign is a validation gap and follow-up, not evidence of a current lease exploit. [R-COORDINATION] [S-VALKEY-SET] [V-NO-VALKEY-CAMPAIGN]

- **Sharing, wrapped-key, and deletion authorization boundaries otherwise held under inspection.** Active relationship/key/grant/record filters and owner/recipient scoping fail closed; viewer/bundle surfaces remain key-free. Deletion snapshots metadata-derived paths, blocks write paths after deletion begins, retries safe categories, and prunes access-enabling metadata while retaining tombstones. [R-SHARING-HTTP] [R-SHARING-HTTP-TESTS] [R-SHARING-REPOSITORIES] [R-SHARING-REPOSITORY-TESTS] [R-BUNDLE-HTTP] [R-BUNDLE-HTTP-TESTS] [R-DELETION-HTTP] [R-DELETION-REPOSITORIES] [R-RETENTION-WORKER]

- **PQ recipient consistency is a known preview blocker, not a newly discovered authorization bypass.** Current server validation checks accepted-profile shapes and metadata but does not prove complete client-side key custody, recipient binding, cross-repository conformance, or decryption UX. The v1 checklist already blocks a preview claim on that broader work. No duplicate issue should be created solely from this review. [R-PQ-ENVELOPE] [R-PQ-DOC] [R-V1-CHECKLIST] [R-WEBCLIENT-README]

- **The public/current-versus-future wording remains conservative.** The reviewed changelog describes an ordinary experimental pre-v1 patch, not v1-preview or production readiness. Missing production mobile capture, browser decryption, emergency dispatch, notifications, hosted billing, and complete trusted-contact UX remain documented future/out-of-scope work rather than defects in this server review. [R-README] [R-CHANGELOG] [R-V1-CHECKLIST] [R-WEBCLIENT-README]

- **Website governance and web-client scope were checked at the requested latest commits.** Server public framing follows the canonical website sources, and the companion web client remains a current experimental metadata/account prototype rather than an absent repository or proof of production client readiness. [R-WEBSITE-GOV] [R-WEBSITE-BASELINE] [R-WEBCLIENT-README]

- **Current workflow action references are full-SHA pinned and production Docker runtime images are digest-pinned/non-root.** Live repository enforcement of action pinning, secret scanning, push protection, and CodeQL can be strengthened, but those settings are retained as defense-in-depth follow-ups rather than inflated into demonstrated vulnerabilities. [R-CI] [R-DOCKERFILE] [R-DOCKERFILE-INGRESS] [S-GITHUB-SECURE] [V-GITHUB-CONTROLS]

- **Backup/restore and production-cluster readiness remain explicitly unproven.** The repository already says a PostgreSQL-plus-encrypted-blob restore drill is required before real optional-backend reliance and that the runbook is not production approval. The missing drill is recorded as a limitation and recommended follow-up, not misclassified as a defect in an ordinary experimental pre-v1 release. [R-CLUSTER-BACKUP] [V-NO-BACKUP-RESTORE]

## Follow-Up Recommendations

### Private security handling

Use GitHub private vulnerability reporting, which was confirmed enabled, for F-001, F-003, F-004, F-008, F-010, and F-011, and for the product-applicability assessment associated with F-002. Keep each report narrow enough to triage severity and disclosure independently. Do not paste the Phase 1 draft wholesale into an advisory, and do not move reproduction, exploitability, trace, object, account, session, or deployment detail into public issues. No private report was created by Phase 2. [R-SECURITY] [V-GITHUB-CONTROLS]

### Sanitized public issues

The 2026-07-11 all-state duplicate check found 143 issues with zero open, 234 pull requests with seven open, and no exact title duplicate among these nine recommendations. Issue state can change, so repeat the check before any later GitHub creation. Phase 2 created only sanitized, branch-scoped local drafts under `.backlog-drafts/2026-07-11/upgrade-codex-reports-and-repo-full-validation-security-review/`; suspected or unresolved vulnerabilities were excluded. [R-AGENTS] [V-GITHUB-ISSUES] [G-FOLLOWUPS]

| Priority | Finding/follow-up | Suggested public issue title | Type / suggested labels | Core acceptance scope |
|---|---|---|---|---|
| P1 | F-005 | `Make PostgreSQL WebAuthn get-or-create concurrency safe` | Bug; `backlog`, `bug`, `go`, `testing` | Atomic conflict handling plus concurrent live-PostgreSQL test |
| P1 | F-006 | `Retry S3 ConditionalRequestConflict instead of reporting duplicate chunk` | Bug; `backlog`, `bug`, `go`, `deployment`, `testing` | Separate 412/409 semantics plus application mapping tests |
| P1 | F-007 | `Check the configured blob backend before API startup` | Deployment bug; `backlog`, `bug`, `go`, `deployment`, `testing` | Fail-closed sanitized startup success/failure tests |
| P1 | F-009 | `Gate protected branches and release publishing on security and optional-backend validation` | Release assurance; `backlog`, `ci`, `security`, `testing`, `deployment`, `maintenance` | Workflow dependencies, full stack, prompt/docs, and live rulesets |
| P2 | F-012 | `Pin disposable PostgreSQL, MinIO, and Valkey smoke images by digest` | Supply-chain/testing; `backlog`, `dependencies`, `docker`, `testing`, `maintenance` | All variants pinned; refresh process; rerun stack |
| P1 | Validation gap | `Add disposable PostgreSQL and S3 backup/restore validation` | Test/runbook; `backlog`, `testing`, `deployment`, `documentation` | Consistent restore, missing/mismatched object detection, deletion-state reconciliation; no real secrets |
| P2 | Validation gap | `Add live Valkey expiry, failure, ACL, and Redis-compatibility tests` | Test; `backlog`, `testing`, `go`, `deployment` | TTL/stale ownership/failure/compatibility behavior |
| P2 | Documentation | `Refresh v1 preview readiness blocker status and cross-repository tracking` | Documentation; `backlog`, `documentation`, `maintenance` | Separate implementation status from closed issue state; retain Not-ready boundary |
| P2 | Security hardening | `Enable repository secret scanning, push protection, and CodeQL default setup` | Repository settings; `backlog`, `security`, `maintenance`, `ci` | Confirm plan/availability, enable safely, document resulting required signals if adopted |

The nonexistent `postgresql`, `s3`, `valkey`, and `redis` labels from the Phase 1 suggestions were replaced above with current repository labels. The backup/restore and Valkey campaign recommendations are not contradictions of the successful smoke test; they cover durability and failure modes the smoke flow does not attempt. The v1 tracking refresh is warranted because the checklist calls linked items “open issues” although the all-state check found the referenced issue set closed while the latest web client still documents several corresponding product surfaces as planned/not implemented. Issue closure must not be mistaken for satisfaction of the release gate. [R-V1-CHECKLIST] [R-WEBCLIENT-README] [V-GITHUB-ISSUES] [V-NO-BACKUP-RESTORE]

Also correct two narrow documentation drifts during the relevant follow-up: the cluster backup runbook still describes upload leases as future even though complete-upload leases are implemented, and the PostgreSQL migration/development guidance should reflect the current full Compose environment. These are documentation cleanups, not separate vulnerabilities. [R-CLUSTER-BACKUP] [R-COORDINATION] [R-UPLOAD-HANDLER] [R-DEVELOPMENT] [R-COMPOSE]

### Publication, remediation, and fresh-review order

1. Triage the private findings and retain sensitive mechanics only in GitHub private vulnerability reports.
2. Remediate the release-blocking findings in separately reviewed changes and add the missing private/public regression tests.
3. Because remediation changes the reviewed commit, start a fresh Phase 0/Phase 1 review at the new SHA and rerun formatting, links, unit/integration/race tests, official `govulncheck`, both Docker builds, live PostgreSQL, S3 deletion, full stack, retry stack, relay smoke, public Actions, and live ruleset inspection.
4. Run that fresh review's own independent Phase 2 pass before publishing or making a release decision for the remediation SHA.

## Conclusion

The reviewed commit shows substantial engineering progress and passed every approved normal and retry-path local validation. Core listener separation, ciphertext-only storage, token handling, bundle integrity, migration discipline, and conservative public wording remain strong. Those results are meaningful but not sufficient to approve release.

At `e609ff86028c81bd149839e03d1ffc0eb2ee9e4a`, one High, six Medium, and five Low findings remain. The release-blocking private findings, failed official Go vulnerability gate, public optional-backend findings, and incomplete release enforcement block the release. The mutable optional-backend smoke images are additionally blocking under the maintainer's explicit Phase 0 policy. [V-CI-RUN] [V-LOCAL-FULLSTACK]

The correct decision is:

- **Not approved for release at the reviewed SHA.**
- **Not ready for a v1 preview, v1.0.0, real-user evidence-upload, public `/v1`, or production-readiness claim.**
- **Release eligibility must be reassessed at the remediation SHA through a fresh Phase 0/Phase 1 review and its own Phase 2 pass; this report makes no readiness determination for that future SHA.**

Phase 2 finalized this disclosure-hardened report and created only ignored local follow-up drafts. It did not remediate the reviewed SHA or create private advisories, public issues, pull requests, releases, or repository-setting changes.

## Citation References

[R-REPORT-PROMPT]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/reports/prompts/phase-1-codex-technical-review.md
[R-PHASE2-PROMPT]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/codex/prompts/95-validate-technical-review-report.md
[R-PRIOR-REPORT]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/reports/2026-06-13-proofline-v0.11.0-rc.1-technical-review.md
[R-README]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/README.md
[R-SECURITY]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/SECURITY.md
[R-CHANGELOG]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/CHANGELOG.md
[R-AGENTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/AGENTS.md
[R-V1-DIRECTION]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/v1-preview-direction.md
[R-INCIDENT-MODES]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/incident-modes.md
[R-SECURITY-MODEL]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/security-model.md
[R-API]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/api.md
[R-ENCRYPTION]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/encryption.md
[R-KEY-CUSTODY]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/key-custody.md
[R-CMD-API]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/cmd/api/main.go
[R-CMD-SERVERS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/cmd/api/servers.go
[R-HTTPAPI-ROUTES]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/routes.go
[R-HTTPAPI-API]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/api.go
[R-AUTH-HANDLERS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/auth_handlers.go
[R-AUTH-MIDDLEWARE]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/auth_middleware.go
[R-AUTH-REPOSITORIES]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/auth.go
[R-AUTH-POSTGRES]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/auth.go
[R-WEB-AUTH-CONTROLS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/web_auth.go
[R-WEB-AUTH-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/auth_test.go
[R-VIEWER-TOKENS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/incident_viewer.go
[R-VIEWER-TOKEN-SQLITE]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/incident_tokens.go
[R-VIEWER-TOKEN-POSTGRES]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/incident_tokens.go
[R-VIEWER-TOKEN-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/incident_viewer_test.go
[R-HTTP-LOGGING]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/middleware.go
[R-HTTP-LOGGING-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/middleware_test.go
[R-UPLOAD-HANDLER]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/chunk_handlers.go
[R-LOCAL-STORAGE]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/storage/committed_blobs.go
[R-LOCAL-STORAGE-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/storage/storage_test.go
[R-S3]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/storage/s3.go
[R-S3-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/storage/s3_test.go
[R-POSTGRES-WEBAUTHN]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/webauthn_second_factors.go
[R-WEBAUTHN-HANDLERS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/webauthn_second_factor_handlers.go
[R-PQ-ENVELOPE]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/envelope/pq/envelope.go
[R-PQ-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/envelope/pq/envelope_test.go
[R-PQ-DOC]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/post-quantum-envelope.md
[R-SHARING-HTTP]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/sharing_handlers.go
[R-SHARING-HTTP-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/sharing_handlers_test.go
[R-SHARING-REPOSITORIES]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/sharing.go
[R-SHARING-REPOSITORY-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/incidents/repository_test.go
[R-DELETION-HTTP]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/deletion_handlers.go
[R-DELETION-REPOSITORIES]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/deletion.go
[R-RETENTION-WORKER]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/retention/deletion_worker.go
[R-CI]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/.github/workflows/ci.yml
[R-RELEASE-PROMPT]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/codex/prompts/90-release-check.md
[R-DEVELOPMENT]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/development.md
[R-GO-TOOLCHAIN]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/go.mod
[R-DOCKERFILE]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/Dockerfile
[R-DOCKERFILE-INGRESS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/Dockerfile.ingress
[R-GETTING-STARTED]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/getting-started.md
[R-COMPOSE]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/compose-full.yml
[R-COMPOSE-PG]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/compose-postgresql-local.yml
[R-COMPOSE-S3]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/compose-sqlite-s3.yml
[R-COMPOSE-SMOKE]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/smoke-test.sh
[R-COMPOSE-RELAY]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/compose/compose-relay-sqlite-local.yml
[R-V1-CHECKLIST]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/v1-preview-readiness-checklist.md
[R-CLUSTER-BACKUP]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/docs/cluster-backup-restore-runbook.md
[R-EVIDENCE-BUNDLE]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/evidencebundle/bundle.go
[R-BUNDLE-HTTP]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/bundle_zip.go
[R-BUNDLE-HTTP-TESTS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/httpapi/bundles_test.go
[R-SQLITE-DB]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/db/db.go
[R-POSTGRES-MIGRATIONS]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/postgresdb/migrate.go
[R-COORDINATION]: https://github.com/open-proofline/server/blob/e609ff86028c81bd149839e03d1ffc0eb2ee9e4a/internal/coordination/coordinator.go
[R-WEBSITE-GOV]: https://github.com/open-proofline/website/blob/1366ea4086db5668ad821b364b31ae25221866aa/docs/governance-and-political-alignment.md
[R-WEBSITE-BASELINE]: https://github.com/open-proofline/website/blob/1366ea4086db5668ad821b364b31ae25221866aa/docs/repository-readme-baseline.md
[R-WEBCLIENT-README]: https://github.com/open-proofline/web-client/blob/ac006f6dd0d88db361a92a92dc3a0557f715b215/README.md
[R-WEBCLIENT-SECURITY]: https://github.com/open-proofline/web-client/blob/ac006f6dd0d88db361a92a92dc3a0557f715b215/docs/security-model.md
[I-PR-377]: https://github.com/open-proofline/server/pull/377

[S-GO-RELEASE]: https://go.dev/doc/devel/release
[S-GO-VULN-5856]: https://vuln.go.dev/ID/GO-2026-5856.json
[S-AWS-PUTOBJECT]: https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html
[S-AWS-HEADBUCKET]: https://docs.aws.amazon.com/AmazonS3/latest/API/API_HeadBucket.html
[S-POSTGRES-PROTOCOL]: https://www.postgresql.org/docs/current/protocol-flow.html
[S-POSTGRES-ERRORS]: https://www.postgresql.org/docs/current/errcodes-appendix.html
[S-POSTGRES-UNIQUE]: https://www.postgresql.org/docs/current/index-unique-checks.html
[S-DOCKER-DIGESTS]: https://docs.docker.com/reference/cli/docker/image/pull/#pull-an-image-by-digest-immutable-identifier
[S-GITHUB-RULESETS]: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets
[S-GITHUB-STATUS]: https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/about-status-checks
[S-GITHUB-JOBS]: https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-jobs
[S-GITHUB-SECURE]: https://docs.github.com/en/actions/reference/security/secure-use
[S-VALKEY-SET]: https://valkey.io/commands/set/
[S-SQLITE-FK]: https://sqlite.org/foreignkeys.html
[S-SQLITE-WAL]: https://sqlite.org/wal.html
[S-OWASP-LOGGING]: https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html

[V-REVIEW-TARGET]: #validation-and-execution-evidence
[V-PHASE2-TARGET]: #validation-and-execution-evidence
[V-LOCAL-BASE]: #validation-and-execution-evidence
[V-LOCAL-FULLSTACK]: #validation-and-execution-evidence
[V-LOCAL-OPTIONAL]: #validation-and-execution-evidence
[V-LOCAL-EXTENDED]: #validation-and-execution-evidence
[V-GOVULN-OFFICIAL]: #validation-and-execution-evidence
[V-GOVULN-LOCAL-DISTRO]: #validation-and-execution-evidence
[V-CI-RUN]: https://github.com/open-proofline/server/actions/runs/29081200062
[V-GITHUB-CONTROLS]: #validation-and-execution-evidence
[V-GITHUB-ISSUES]: #validation-and-execution-evidence
[V-COMPANION-HEADS]: #validation-and-execution-evidence
[V-PHASE2-SOURCE-READS]: #validation-and-execution-evidence
[V-PHASE2-RECHECK]: #validation-and-execution-evidence
[V-PHASE2-PUBLICATION]: #validation-and-execution-evidence
[V-PHASE2-EXTERNAL]: #validation-and-execution-evidence
[V-MAINTAINER-CONTEXT]: #validation-and-execution-evidence
[V-PHASE2-CLEANUP]: #validation-and-execution-evidence
[V-PHASE2-GOVULN]: #validation-and-execution-evidence
[V-ULTRA-REVIEW]: #validation-and-execution-evidence
[V-TOOLS]: #validation-and-execution-evidence
[V-NO-FAULT-INJECTION]: #sources-checks-and-commands-not-available-or-not-executed
[V-NO-BACKUP-RESTORE]: #sources-checks-and-commands-not-available-or-not-executed
[V-NO-VALKEY-CAMPAIGN]: #sources-checks-and-commands-not-available-or-not-executed
[G-FOLLOWUPS]: #generated-artifacts-and-report-outputs
