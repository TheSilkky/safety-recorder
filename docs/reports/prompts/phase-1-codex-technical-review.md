# Phase 1 Codex Prompt: Public Technical Review Report

Run this prompt in Codex Max when the review is best handled as one focused
task. Run it in Codex Ultra when the review has meaningful independent research
lanes that benefit from subagent delegation. Max applies maximum reasoning to
one task; Ultra adds coordinated subagent delegation. Do not use a lower-effort
mode, and do not create artificial parallel work merely to use Ultra.

This prompt creates the Phase 1 source-cited technical review draft. A separate
Phase 2 Codex workflow must validate, clean, and public-harden the draft before
publication. Phase 1 must not write the final report in place of that independent
validation pass.

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

Phase 1 draft report path:

```text
.technical-review-drafts/<YYYY-MM-DD>-proofline-<TARGET_RELEASE_OR_VERSION>-technical-review-draft.md
```

Final Phase 2 report path:

```text
docs/reports/<YYYY-MM-DD>-proofline-<TARGET_RELEASE_OR_VERSION>-technical-review.md
```

Model / Codex mode disclosure:

```text
OpenAI Codex using <MODEL_NAME> in <MAX_OR_ULTRA_MODE>
```

Replace both placeholders with the model and mode actually used. Do not report
an intended model or mode that was not active for the Phase 1 run.

## Repository Context

Proofline is an experimental Go backend for private encrypted incident capture.
It receives already-encrypted recording chunks, stores metadata in SQLite by
default or optional PostgreSQL, keeps encrypted blobs on local disk by default
or in optional S3-compatible object storage, supports optional
Valkey/Redis-compatible short-lived coordination, and exposes a token-scoped
read-only incident viewer.

The product documentation now uses the name Proofline. Repository URLs, the Go module path, Docker image names, GHCR package names, release binary names, runtime protocol identifiers, and default data-layout identifiers use the `open-proofline/server` repository namespace and Proofline names. Historical reports, archived prompts, legacy `/e/{token}` aliases, and historical migration names may still mention `safety-recorder` or `emergency`.

Project-wide public governance posture, political alignment, public-good
framing, public voice, reusable README baseline guidance, and source-of-truth
mapping live in `open-proofline/website`. When the report summarizes those
project-wide claims, use the website source documents rather than inventing a
server-local governance claim:

```text
open-proofline/website/docs/governance-and-political-alignment.md
open-proofline/website/docs/repository-readme-baseline.md
```

The long-term product direction is broader than emergency-only recording. Planned modes include emergency incidents, non-emergency interaction records, timed safety checks, and evidence notes. These are planning direction unless the reviewed tree contains first-class implementation.

Core project boundaries:

- The main `/v1` API uses local account sessions but is not a public product API; it must stay behind the reviewed deployment boundary, and public edges must not route `/admin/api/...`.
- The main API/viewer routes and private `/admin` routes must remain on separate
  listener groups and separate muxes. Public incident-viewer paths remain
  read-only; public edges must not route private write or admin surfaces.
- The current backend treats uploaded bytes as opaque ciphertext.
- Completed evidence bundles are encrypted chunk bundles, not decrypted or
  playable media exports. ZIP entry names remain server-controlled.
- The current backend must not be described as production-ready public infrastructure.
- Current backend incidents are generic by default. Optional incident-mode,
  capture-profile, escalation-policy, and sharing-state metadata are labels
  only unless the reviewed tree implements first-class behavior for them.
- Backend decryption, browser decryption, production key custody, break-glass
  access, public account portals, OAuth/JWT, push notifications, SMS,
  Messenger, production web/iOS/Android client behavior, and first-class
  escalation policies are future or out-of-scope items unless explicitly
  implemented in the reviewed tree.
- Recipient-key metadata, trusted-contact relationships, sharing grants,
  wrapped-key storage or delivery, and related account behavior must be
  described from code and tests at `<REVIEWED_COMMIT_SHA>`. Do not describe
  those surfaces as wholly future when the reviewed commit implements them,
  and do not infer production client decryption or complete key custody merely
  from implemented metadata or API routes.
- The reviewed commit may implement some or all of the accepted post-quantum
  envelope for server-side validation, wrapped-key profile validation, bundle
  hints, or simulator reference flows. Verify each claimed component against
  code and tests at `<REVIEWED_COMMIT_SHA>`; do not treat server or simulator
  implementation as proof of completed web-client upload, review, decryption,
  key custody, or cross-repository conformance.
- V1 preview, v1.0.0, or real-user evidence-upload readiness claims must be
  checked against `docs/v1-preview-readiness-checklist.md` when that file is
  present. Passing ordinary tests is not enough to support preview-ready
  language if checklist hard blockers remain incomplete.
- Key custody, browser decryption, break-glass, incident-mode, v1 direction,
  post-quantum envelope, and client documents may combine implemented behavior,
  accepted design constraints, and future preview requirements. Classify each
  claim from repository evidence at `<REVIEWED_COMMIT_SHA>` instead of treating
  an entire document as either shipped implementation or future planning.
- Determine `open-proofline/web-client` status from the project map and
  companion-repository evidence applicable to the reviewed commit. Do not call
  it absent when those sources identify a current experimental companion, and
  do not treat its existence as implemented server behavior or production
  web-client readiness.
- Do not treat documented future work as a current defect merely because it is not implemented.

## Codex Execution And Validation Evidence Policy

This is a read-only review workflow except for creating or updating the Phase 1
draft at the path listed in Inputs. Do not change application code, tests,
migrations, workflows, configuration, source documentation, the final Phase 2
report, GitHub settings, issues, or pull requests. Do not stage or commit files.

Inspect the working tree before research and preserve all existing tracked and
untracked work. Never discard, overwrite, reformat, or incorporate unrelated
user changes into the draft.

Record the current branch, current `HEAD`, reviewed branch/ref, and exact
`<REVIEWED_COMMIT_SHA>`. Resolve `<REVIEWED_BRANCH_OR_REF>` and confirm it points
to `<REVIEWED_COMMIT_SHA>` before continuing; stop and report a mismatch.
Ground repository claims in that reviewed commit. If the current checkout is
not the reviewed commit, do not run a checkout-dependent command against `HEAD`
and attribute its result to the reviewed commit. Use a clean isolated copy,
commit-specific supplied evidence, or mark the command as not independently
executed for the reviewed commit.

Even when `HEAD` equals `<REVIEWED_COMMIT_SHA>`, a dirty working tree is not the
reviewed commit. Use `git show <REVIEWED_COMMIT_SHA>:<path>` or another
commit-addressed read for source inspection. Run tests, builds, or smoke checks
as reviewed-commit evidence only in a clean isolated temporary clone or archive
of that commit whose creation was approved in Phase 0. Do not switch, clean,
reset, reformat, or add a worktree to the maintainer's current checkout. Results
from a dirty working tree may be recorded only as current-workspace evidence,
never as proof about the reviewed commit.

Codex may use non-mutating local shell inspection, run safe local validation in
an approved clean isolated copy of the reviewed commit, and consult
authoritative public web or source documentation. Do not access or change
private or production services. Do not run checks that require real secrets,
private deployment details, production credentials, or private service
endpoints. Disposable local services may be used only with synthetic test
credentials and temporary storage, and must not write generated artifacts into
the repository checkout.

Claim that a command, test, build, container, smoke test, CI run, repository
setting, or external source was checked only when this Phase 1 run actually
checked it or when cited supplied evidence proves it. Record the exact command
or evidence, commit context, result, limitations, and whether it was generated
in this run or supplied by another source. If evidence is unavailable, state
that the check was not independently executed or verified; do not infer a pass.

Do not write that review constraints prohibited network calls, web access, or
external source consultation unless the maintainer explicitly imposed that
constraint for this review.

Recommended validation evidence to request or use when available:

- exact reviewed branch/ref
- exact reviewed commit SHA
- GitHub Actions run URLs for the reviewed commit
- Phase 1 output for the non-mutating formatting check
  `gofmt -l ./cmd ./internal ./migrations`
- Phase 1 or supplied output for `go test ./...`
- Phase 1 or supplied output for `go vet ./...`
- Phase 1, local, or CI output for `docker build -t proofline-server .`
- supplied or public CI evidence for PostgreSQL integration tests, or Phase 1
  output from a disposable local test service that requires no secret DSN
- supplied or public CI evidence for S3-compatible blob storage tests or smoke
  tests, or Phase 1 output using a disposable local service with fixed
  non-secret test credentials only
- supplied or public CI evidence for Valkey/Redis-compatible coordination
  startup-check tests or smoke tests, or Phase 1 output using a disposable local
  service that requires no secrets
- explicit note whether PostgreSQL, S3-compatible storage, and Valkey/Redis-compatible coordination were only reviewed from code/docs or also exercised with live disposable services
- Phase 1 or supplied output for the documented simulator smoke flow only when
  Phase 1 starts a fresh disposable server on an explicit non-default loopback
  URL, uses temporary storage and synthetic credentials, clears inherited
  deployment and `PROOFLINE_SIM_*` configuration before setting explicit test
  values, and proves it cannot target an existing local, private, or production
  service. Otherwise, do not run the simulator and record the limitation.

Do not put raw viewer, incident, session, or future token-like values; secrets;
Authorization headers; request bodies; uploaded file bytes; plaintext; raw keys;
wrapped-key ciphertext; private deployment details; stored paths; object keys;
exploit payloads; or user-safety data into validation summaries or the public
report.

## Source Policy

Use authoritative sources only.

Repository evidence is the anchor for claims about the reviewed tree, but this is not a repository-only review. Repository claims must be grounded in the reviewed commit, and external technical claims must use authoritative sources.

Prioritize repository evidence first:

- repository files in the reviewed tree
- source code, migrations, tests, workflows, Dockerfile, and documentation pinned to `<REVIEWED_COMMIT_SHA>`

Required external-source families when applicable:

- Project-wide public governance posture, political alignment, public-good
  framing, public voice, README baseline, or source-of-truth mapping claims:
  `open-proofline/website/docs/governance-and-political-alignment.md` and
  `open-proofline/website/docs/repository-readme-baseline.md`
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
- List every companion-project source materially relied on, including website
  governance/README-baseline docs or web-client docs, with commit/ref/date and
  limitations.
- List every authoritative external source materially relied on.
- List required authoritative external source categories that were not consulted and explain why.
- List validation commands that were actually supported by supplied evidence.
- List validation commands that were not independently executed or not supported by supplied evidence.
- List the Phase 1 draft artifact and the final Phase 2 target path. Mark the
  final path as a planned output that Phase 1 did not generate.
- List active review connector/tool context, including whether web search was available.

## Citation Requirements

Use portable citation keys only.

Do not use platform-internal or renderer-only citation identifiers such as
`filecite` / `cite` blocks or raw `turnXfileY`, `turnXviewY`, `turnXsearchY`,
`turnXfetchY`, or `turnXopenY` references.

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
- `docs/v1-preview-direction.md`
- `docs/v1-preview-readiness-checklist.md`
- `docs/key-custody.md`
- `docs/browser-decryption.md`
- `docs/break-glass-key-access.md`
- `docs/ios-local-recorder-prototype.md`
- any current or future web-client, iOS, Android, account, protocol,
  Apple-platform, or client-planning documents
- any future client code, Swift/Kotlin/TypeScript package files, Xcode/Android project files, entitlement files, or App Store/Play Store metadata files if they exist in the reviewed tree

When public governance posture, political alignment, public-good framing,
public voice, reusable README baseline guidance, or source-of-truth mapping is
in scope, inspect the website source documents listed in Repository Context.
When the report discusses web-client behavior or prototype limits, inspect the
companion-repository sources applicable to the reviewed commit when accessible,
and clearly separate companion-repository behavior from server behavior.

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
13. Recipient-key, trusted-contact, sharing-grant, and wrapped-key behavior
    implemented at the reviewed commit, plus future production key custody,
    browser/client-side decryption, trusted-contact review, break-glass, and
    server escrow boundaries
14. Web-client companion status and scope applicable to the reviewed commit,
    future iOS/Android/protocol/client planning, and platform assumptions
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
- follow-up handling: GitHub private vulnerability reporting, sanitized public
  hardening/documentation issue, non-security issue, or none
- suggested GitHub issue title only for a sanitized non-vulnerability follow-up
- acceptance criteria

Do not inflate severity merely because a finding is security-adjacent. If a limitation is already documented as out of scope, classify it as a non-finding or future-work item unless there is a contradiction between docs and code.

Do not recommend public GitHub issues containing private vulnerability details,
raw or future token-like values, secrets, Authorization headers, request bodies,
uploaded file bytes, plaintext, raw keys, wrapped-key ciphertext, private
deployment details, stored paths, object keys, exploit details, or user-safety
data.

Route suspected or unresolved vulnerabilities through `SECURITY.md` and GitHub
private vulnerability reporting. Do not put exploitability, reproduction, or
remediation details for an unresolved vulnerability in the public technical
review draft or a public issue suggestion. A sanitized public hardening issue is
appropriate only when it does not disclose a vulnerability.

## Common False Positives To Avoid

- Do not say `/v1` lacks public auth as a vulnerability unless the docs claim it is safe to expose publicly.
- Do not say missing iOS, Android, production web-client behavior, accounts,
  incident modes, capture profiles, escalation policies, sharing state, browser
  decryption, production key custody, or break-glass behavior is a defect when
  docs mark those as future work.
- Do not say `open-proofline/web-client` is missing when the project map and
  companion-repository evidence applicable to the reviewed commit identify it
  as an existing experimental companion repository.
- Do not describe recipient-key, trusted-contact, sharing-grant, or wrapped-key
  behavior as wholly absent or wholly future when code at the reviewed commit
  proves an implemented metadata or API surface. Do not overstate that surface
  as completed production key custody or client-side review.
- Do not describe the post-quantum envelope as wholly future when code at the
  reviewed commit proves implemented server or simulator behavior, and do not
  describe it as fully implemented across server, web-client, key custody, and
  review paths unless evidence proves each component. If reviewing a v1 preview
  readiness claim, verify that the required envelope behavior is implemented,
  documented, tested, and default across the claimed preview paths.
- Do not describe Proofline as ready for v1 preview, v1.0.0, or real-user
  evidence upload unless the reviewed tree satisfies the v1 preview readiness
  checklist hard blockers.
- Do not treat historical `safety-recorder` or `emergency` references in reports, archived prompts, legacy route aliases, or historical migration names as stale product naming by themselves.
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

Write the Phase 1 report only to the draft path listed in Inputs. Do not write
or replace the final Phase 2 report. The draft must clearly separate implemented
behavior from future planning, preserve public-safety restrictions, and avoid
production-readiness claims.
