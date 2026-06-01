# Phase 1 Deep Research Prompt: Public Technical Review Report

Use this prompt in ChatGPT Deep Research, not in Codex.

This prompt creates the first-pass source-cited technical review report. The output must be validated and cleaned by the Phase 2 Codex workflow before publication.

## Inputs

Repository:

```text
open-proofline/server
```

Reviewed branch or ref:

```text
<REVIEWED_BRANCH_OR_REF>
```

Reviewed commit SHA:

```text
<REVIEWED_COMMIT_SHA>
```

Target release / version:

```text
<TARGET_RELEASE_OR_VERSION>
```

Review date:

```text
<YYYY-MM-DD>
```

Target report path, if known:

```text
docs/reports/<YYYY-MM-DD>-proofline-<TARGET_RELEASE_OR_VERSION>-technical-review.md
```

Model / tool disclosure:

```text
OpenAI ChatGPT Deep Research using <MODEL_NAME>
```

## Repository Context

Proofline is an experimental Go backend for private encrypted incident capture.
It receives already-encrypted recording chunks, stores metadata in SQLite by
default or optional PostgreSQL, keeps encrypted blobs on local disk by default
or in optional S3-compatible object storage, supports optional
Valkey/Redis-compatible short-lived coordination, and exposes a token-scoped
read-only incident viewer.

The product documentation now uses the name Proofline. Repository URLs, the Go module path, Docker image names, GHCR package names, and release binary names use the `open-proofline/server` repository namespace. Compatibility identifiers such as the v1 simulator encryption envelope, default SQLite filename, legacy `/e/{token}` aliases, and historical migration names may still use `safety-recorder` or `emergency` until separate protocol or data-layout migrations are explicitly performed.

The long-term product direction is broader than emergency-only recording. Planned modes include emergency incidents, non-emergency interaction records, timed safety checks, and evidence notes. These are planning direction unless the reviewed tree contains first-class implementation.

Core project boundaries:

- The main `/v1` API uses local account sessions but is not a public product API; it must stay behind the reviewed deployment boundary, and public edges must not route `/v1/admin/...`.
- The current backend treats uploaded bytes as opaque ciphertext.
- The current backend must not be described as production-ready public infrastructure.
- Current backend incidents are generic unless the reviewed tree implements first-class incident modes, capture profiles, escalation policies, or sharing state.
- Backend decryption, browser decryption, production key custody, break-glass access, user accounts, OAuth/JWT, push notifications, SMS, Messenger, web/iOS/Android clients, and first-class escalation policies are future or out-of-scope items unless explicitly implemented in the reviewed tree.
- Future key custody, browser decryption, break-glass, incident-mode, and client prototype documents are design/planning guardrails, not shipped implementation.
- Do not treat documented future work as a current defect merely because it is not implemented.

## Validation Evidence Policy

Deep Research cannot run repository tests, containers, Docker builds, local shell commands, or simulator smoke tests.

This policy restricts claims about executing commands, tests, builds, containers, and smoke tests. It does not prohibit authoritative external web or source-document consultation. Deep Research may and should consult authoritative external documentation when available, as required by the Source Policy below.

Do not write that review constraints prohibited network calls, web access, or external source consultation unless the maintainer explicitly imposed a no-network constraint for this review.

Do not claim that Deep Research personally ran tests, built containers, executed Go commands, started the API server, ran the simulator, inspected live GitHub repository settings, or validated a Docker image by executing it.

Use only supplied validation evidence, public CI results, uploaded logs, maintainer-supplied summaries, Codex-supplied command output, or repository workflow files when discussing test/build status.

If no validation evidence is supplied for a command, state that the command was not independently verified by this report.

Recommended validation evidence to request or use when available:

- exact reviewed branch/ref
- exact reviewed commit SHA
- GitHub Actions run URLs for the reviewed commit
- local or Codex output for `gofmt -w ./cmd ./internal ./migrations`
- local or Codex output for `go test ./...`
- local or Codex output for `go vet ./...`
- local or CI output for `docker build -t proofline-server .`
- local or Codex output for PostgreSQL integration tests:

```bash
SAFE_POSTGRES_TEST_DSN='<test database DSN>' go test ./internal/postgresdb -count=1
```

- local or Codex output for S3-compatible blob storage tests or smoke tests using non-secret local/test credentials only
- local or Codex output for Valkey/Redis-compatible coordination startup-check tests or smoke tests
- explicit note whether PostgreSQL, S3-compatible storage, and Valkey/Redis-compatible coordination were only reviewed from code/docs or also exercised with live disposable services
- local or Codex output for the simulator smoke test:

```bash
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

Do not put raw tokens, secrets, request bodies, uploaded bytes, plaintext, raw keys, private deployment details, exploit payloads, or user-safety data into validation summaries or the public report.

## Source Policy

Use authoritative sources only.

Repository evidence is the anchor for claims about the reviewed tree, but this is not a repository-only review. Repository claims must be grounded in the reviewed commit, and external technical claims must use authoritative sources.

Prioritize repository evidence first:

- repository files in the reviewed tree
- source code, migrations, tests, workflows, Dockerfile, and documentation pinned to `<REVIEWED_COMMIT_SHA>`

Required external-source families when applicable:

- Go/toolchain/standard-library/module claims: `go.dev` or `pkg.go.dev`
- AES-GCM, nonce, randomness, authenticated encryption, or cryptographic-strength claims: NIST, Go official docs, or another primary standards/source document
- SQLite WAL, foreign keys, migration, transaction, locking, backup, or restore claims: `sqlite.org`
- PostgreSQL schema, migrations, transactions, isolation, advisory locks, connection pooling, backup, or restore claims: `postgresql.org` and `pkg.go.dev` for the Go PostgreSQL driver/library APIs used by the reviewed tree
- S3-compatible object storage, object keys, conditional writes, checksum, consistency, delete behavior, lifecycle, versioning, or backup claims: AWS S3 official documentation for S3 API semantics, plus provider documentation only when reviewing provider-specific examples such as MinIO
- Valkey/Redis-compatible coordination, ping/startup checks, TTLs, leases, locks, retry coordination, persistence, or non-durable coordination claims: Valkey, Redis, or relevant client-library primary documentation
- MinIO/local S3-compatible testing claims: MinIO official documentation when the report discusses MinIO-specific setup or behavior
- GitHub Actions security, permissions, SHA pinning, Dependabot, provenance, OIDC, workflow hardening, or CI/CD claims: `docs.github.com`
- Docker image pinning, digest semantics, multi-stage builds, runtime image behavior, or container build/publish claims: `docs.docker.com`
- dependency vulnerability/advisory claims: OSV, Go vulnerability database, GitHub Advisory Database, or another primary advisory source
- licence/SPDX/AGPL claims: repository licence plus SPDX, GNU/FSF, or another authoritative licence source
- web-security headers, caching, token-in-URL handling, sensitive-data logging, rate limiting, or browser-facing security claims: OWASP, relevant RFCs, GitHub/Docker docs, Traefik docs for Traefik-specific examples, or other primary sources
- iOS, Swift, Apple-platform, App Store, AVFoundation, background execution, CryptoKit, Keychain, or Apple privacy/safety claims: Apple Developer or Swift primary documentation
- recording-law or legal-admissibility claims: do not provide legal conclusions unless sourced to current authoritative legal material and clearly marked as not legal advice

Avoid random blogs, Stack Overflow, social posts, vendor marketing pages, AI-generated summaries, uncited claims, and stale Apple API examples when current Apple documentation is available.

If external source access is unavailable in the execution environment, state that source access was unavailable in the Source Registry and mark affected external technical claims as not independently verified. Do not use missing external sources as evidence supporting a finding; missing source access is a limitation on verification.

If required external sources are available but were not consulted, state that limitation in the Source Registry and mark affected claims as not independently verified.

## Source Registry

Before drafting findings, create a `## Source Registry` section. It must include:

```markdown
## Source Registry

### Repository sources inspected

### External authoritative sources consulted

### Validation and execution evidence

### Sources, checks, and commands not available or not executed

### Generated artifacts and report outputs
```

Every registry entry must include:

- source ID / citation key
- source type
- location
- commit/ref/date
- purpose in the review
- status
- limitations
- related finding IDs or report sections where applicable

Minimum requirements:

- List every repository file materially relied on, pinned to `<REVIEWED_COMMIT_SHA>`.
- List every authoritative external source materially relied on.
- List required authoritative external source categories that were not consulted and explain why.
- List validation commands that were actually supported by supplied evidence.
- List validation commands that were not independently executed or not supported by supplied evidence.
- List generated report outputs, including the target report path if supplied.
- List active review connector/tool context, including whether web search was available.

## Citation Requirements

Use portable citation keys only.

Do not use ChatGPT internal citation tokens such as renderer-only `filecite` / `cite` blocks or raw `turnXfileY`, `turnXviewY`, `turnXsearchY`, `turnXfetchY`, or `turnXopenY` references.

Use this citation style:

```markdown
Repository fact. [R-README] [R-CI]

External-source fact. [S-GITHUB-ACTIONS-SECURE]

Apple-platform planning fact. [S-APPLE-AVFOUNDATION]
```

At the end of the report, include Markdown reference definitions for every citation key.

Repository citations must be pinned to `<REVIEWED_COMMIT_SHA>`, not `main`, `develop`, or a moving release branch. If the SHA is unavailable, clearly mark the report as a draft and include a warning that repository URLs must be commit-pinned before publication.

## Review Scope

Review these repository areas when present in the reviewed tree:

- `README.md`
- `SECURITY.md`
- `CHANGELOG.md`
- `AGENTS.md`
- `docs/`
- `codex/`
- `.github/`
- `cmd/`
- `internal/`
- `migrations/`
- `Dockerfile`
- `.dockerignore`, if present
- GitHub Actions workflows and Dependabot configuration

Pay special attention to future-design and planning documents when present:

- `docs/incident-modes.md`
- `docs/key-custody.md`
- `docs/browser-decryption.md`
- `docs/break-glass-key-access.md`
- `docs/ios-local-recorder-prototype.md`
- any future web, iOS, Android, account, protocol, Apple-platform, or client-planning documents
- any future client code, Swift/Kotlin/TypeScript package files, Xcode/Android project files, entitlement files, or App Store/Play Store metadata files if they exist in the reviewed tree

Technical focus areas:

1. Documentation consistency, Proofline naming, compatibility-name notes, and public-readiness wording
2. Current implementation versus future incident-mode planning
3. Go backend structure and idiomatic implementation
4. HTTP API behavior and private/public route separation
5. Viewer/incident token generation, hashing, storage, expiry, redaction, and viewer behavior
6. Logging, metrics, proxy examples, and sensitive data exposure
7. Upload handling, hash verification, immutable storage, upload limits, and stream-scoped chunk identity
8. SQLite and PostgreSQL migrations, foreign keys, transactions, schema migration tracking, repository parity, and data integrity
9. Local and S3-compatible encrypted blob storage, immutable commit behavior, object-key safety, missing-blob failure handling, and backup/restore assumptions
10. Optional Valkey/Redis-compatible coordination startup checks, failure behavior, and non-durable coordination boundaries
11. ZIP bundle generation, manifest completeness, fail-closed behavior, and path traversal handling
12. Crypto-adjacent simulator envelope, ciphertext-only backend boundary, and naming-compatibility claims
13. Future key custody, browser/client-side decryption, break-glass, trusted-contact access, and server escrow boundaries
14. Future web/iOS/Android/protocol/client planning and platform assumptions
15. Deployment guidance, Traefik examples, WireGuard/private boundary, rate limiting, and no `/v1` public exposure
16. Docker/GHCR/GitHub Actions/supply-chain hygiene
17. Public issue/report safety

## Finding Rules

For every finding, include:

- finding ID
- severity: Critical / High / Medium / Low / Informational
- confidence: High / Medium / Low
- current implementation vs future design
- affected files and functions, or affected planning documents
- repository evidence citation
- authoritative external citation for applicable backend, security, CI/CD, Docker, SQLite, PostgreSQL, S3-compatible storage, Valkey/Redis-compatible coordination, dependency, licence, standards, web-security, Apple/iOS, Swift, or legal-adjacent claim
- explicit `not independently verified` wording if required authoritative external sources were not consulted
- reviewed branch/ref and commit context
- why it matters
- minimal actionable fix
- suggested GitHub issue title
- acceptance criteria

Do not inflate severity merely because a finding is security-adjacent. If a limitation is already documented as out of scope, classify it as a non-finding or future-work item unless there is a contradiction between docs and code.

Do not recommend public GitHub issues for private vulnerabilities, raw tokens, secrets, exploit details, private deployment details, or user safety data.

## Common False Positives To Avoid

- Do not say `/v1` lacks public auth as a vulnerability unless the docs claim it is safe to expose publicly.
- Do not say missing iOS, Android, web-client, accounts, incident modes, capture profiles, escalation policies, sharing state, browser decryption, production key custody, or break-glass behavior is a defect when docs mark those as future work.
- Do not treat remaining `safety-recorder` or `emergency` compatibility identifiers as stale when docs explicitly state those names remain for protocol, data-layout, route-alias, or migration compatibility.
- Do not claim emergency-services integration exists.
- Do not imply Proofline reports crimes, contacts police, guarantees legal admissibility, or provides legal advice.
- Do not treat planned interaction records as police-specific surveillance features; use neutral incident-capture framing.
- Do not claim backend decryption or server-held keys exist unless implementation proves it.
- Do not include sensitive details in public report text or issue drafts.

## Required Output Structure

The report should use this structure:

```markdown
# Technical Review of Proofline <version/ref>

## Executive Summary

## Source Registry

### Repository sources inspected

### External authoritative sources consulted

### Validation and execution evidence

### Sources, checks, and commands not available or not executed

### Generated artifacts and report outputs

## Scope And Method

## Current Implementation Summary

## Findings

## Non-Findings And Confirmed Boundaries

## Follow-Up Recommendations

## Conclusion

## Citation References
```

The report must clearly separate implemented behavior from future planning, preserve public-safety restrictions, and avoid production-readiness claims.
