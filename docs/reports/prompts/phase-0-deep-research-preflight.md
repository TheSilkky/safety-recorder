# Phase 0 Deep Research Prompt: Load Report Instructions And Plan Research

Use this prompt in ChatGPT Deep Research before running the Phase 1 technical review report.

Do **not** produce the technical review report in this Phase 0 step.

This prompt exists because the Phase 1 report prompt is strict. Phase 0 should load the current Phase 1 prompt from the repository, restate the governing rules, and produce a research plan for maintainer approval before the actual report is generated.

## Repository

```text
open-proofline/server
```

Product documentation currently uses the name Proofline. Repository URLs, the Go module path, Docker image names, GHCR package names, release binary names, runtime protocol identifiers, and default data-layout identifiers use the `open-proofline/server` repository namespace and Proofline names. Historical reports, archived prompts, legacy `/e/{token}` aliases, and historical migration names may still mention `safety-recorder` or `emergency`.

Project-wide public governance posture, political alignment, public-good framing,
public voice, reusable README baseline guidance, and source-of-truth mapping
live in `open-proofline/website`, especially:

```text
docs/governance-and-political-alignment.md
docs/repository-readme-baseline.md
```

## Inputs

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

Phase 1 prompt path:

```text
docs/reports/prompts/phase-1-deep-research-technical-review.md
```

Expected Phase 2 validation prompt path:

```text
codex/prompts/95-validate-deep-research-report.md
```

Target report path, if known:

```text
docs/reports/<YYYY-MM-DD>-proofline-<TARGET_RELEASE_OR_VERSION>-technical-review.md
```

Model / tool disclosure:

```text
OpenAI ChatGPT Deep Research using <MODEL_NAME>
```

## Goal

Load and internalize the current Phase 1 prompt from the reviewed repository branch/ref or reviewed commit, then produce a research plan that follows that prompt.

Phase 0 should make sure the actual Phase 1 report run will follow:

- the correct repository scope
- the correct reviewed branch/ref and commit SHA
- the current Proofline naming and compatibility-name rules
- the correct target release/version
- the Phase 1 Source Registry requirements
- the Phase 1 citation requirements
- the Phase 1 validation-evidence policy
- the Phase 1 public-safety restrictions
- the Phase 1 implementation-vs-future-planning boundary
- the Phase 2 handoff expectations

## Required Preflight Steps

1. Open and read the current Phase 1 prompt from the reviewed branch/ref or commit:

   ```text
   docs/reports/prompts/phase-1-deep-research-technical-review.md
   ```

2. Confirm the Phase 1 prompt path and source used.

3. Summarize the governing Phase 1 requirements in your own words.

4. Identify required repository files and directories to inspect, including:

   ```text
   README.md
   SECURITY.md
   CHANGELOG.md
   AGENTS.md
   docs/
   codex/
   .github/
   cmd/
   internal/
   migrations/
   Dockerfile
   .dockerignore
   ```

5. Identify future-design and planning documents that must be reviewed when present, including but not limited to:

   ```text
   docs/incident-modes.md
   docs/key-custody.md
   docs/browser-decryption.md
   docs/break-glass-key-access.md
   docs/ios-local-recorder-prototype.md
   ```

6. Identify the authoritative external source categories required by the Phase 1 prompt, including the v0.8.0 optional backend-support areas:

   - PostgreSQL metadata backend support
   - S3-compatible blob/object storage backend support
   - Valkey/Redis-compatible short-lived coordination backend support
   - exact Go client package documentation on `pkg.go.dev` when package behavior is discussed

7. Identify public governance posture, public voice, README baseline,
   Apple/iOS/Swift, Android, web-client, protocol, legal-adjacent, or App
   Store/Play Store claims that would need primary sources if the report
   discusses them.

8. Identify the required Source Registry sections and what each section must contain.

9. Identify the citation requirements, including repository citation pinning to `<REVIEWED_COMMIT_SHA>`.

10. Identify the validation evidence policy, especially that Deep Research cannot run tests, containers, Docker builds, local shell commands, or simulator smoke tests, while still allowing authoritative external source consultation when available.

11. Identify public-safety restrictions, including prohibited raw tokens, secrets, request bodies, uploaded bytes, plaintext, raw keys, private deployment details, exploit payloads, and user-safety data.

12. Identify common false-positive patterns the Phase 1 report should avoid, including:

    - treating documented future work as current defects
    - treating compatibility names as stale when docs explicitly preserve them
    - treating Proofline as having emergency-services integration
    - treating interaction records as police-specific surveillance features
    - treating preserved compatibility identifiers as stale after the repository/module/artifact rename

13. Produce a proposed research plan.

14. Stop and wait for maintainer approval before running the actual Phase 1 report.

## Important Constraints

Do **not** produce the technical review report in Phase 0.

Do **not** treat this preflight as the Phase 1 report.

Do **not** claim tests, containers, Docker builds, local shell commands, GitHub repository settings, or simulator smoke tests were run unless supplied validation evidence proves that.

Do **not** use ChatGPT-rendered citations as final public Markdown citations.

Do **not** rely on repository-only evidence for external standards, platform, security, CI/CD, Docker, SQLite, dependency, license, Apple/iOS, Android, Swift, web-client, protocol, or legal-adjacent claims when the Phase 1 prompt requires authoritative external sources.

Do **not** include raw tokens, secrets, request bodies, uploaded bytes, plaintext, raw keys, private deployment details, exploit payloads, or user-safety data.

Do **not** claim production readiness.

Do **not** claim formal security audit, penetration test, compliance certification, legal review, App Store review, Play Store review, or production-readiness endorsement.

Do **not** describe future incident modes, account access, key custody, browser decryption, break-glass access, mobile clients, web clients, or the post-quantum envelope as implemented features unless the reviewed tree contains implementation code. The post-quantum envelope is a v1 preview requirement, not current runtime behavior unless implementation has landed.

Do **not** describe `open-proofline/web-client` as absent if the current
project map identifies it as an existing experimental companion repository.
Distinguish current companion repositories from future production behavior and
from future iOS, Android, and protocol repositories.

## Expected Source Plan

The research plan should identify sources in these groups.

### Repository Sources

At minimum, plan to inspect current files such as:

```text
README.md
SECURITY.md
CHANGELOG.md
AGENTS.md
docs/
codex/
.github/
cmd/
internal/
migrations/
Dockerfile
.dockerignore
```

Also include relevant future-planning documents if present:

```text
docs/incident-modes.md
docs/key-custody.md
docs/browser-decryption.md
docs/break-glass-key-access.md
docs/ios-local-recorder-prototype.md
```

When public governance posture, political alignment, public-good framing,
public voice, reusable README baseline guidance, or source-of-truth mapping is
in scope, also plan to inspect the website source documents:

```text
open-proofline/website/docs/governance-and-political-alignment.md
open-proofline/website/docs/repository-readme-baseline.md
```

For the v0.8.0 optional backend-support review, also plan repository inspection for:

- PostgreSQL metadata backend implementation and documentation, including configuration, connection setup, migrations, schema behavior, transaction and constraint behavior, fallback/default behavior, tests or workflows, dependency declarations, and package usage
- S3-compatible blob/object storage implementation and documentation, including configuration, connection setup, object key construction, upload/download behavior, immutability assumptions, hash/checksum handling, fallback/default behavior, tests or workflows, dependency declarations, and package usage
- Valkey/Redis-compatible coordination implementation and documentation, including configuration, connection setup, startup checks, TTL/expiry behavior where implemented or planned, short-lived coordination semantics, fallback/default behavior, tests or workflows, dependency declarations, and package usage

### External Authoritative Sources

Use the Phase 1 prompt as the source of truth for required external source categories.

The plan should distinguish external source consultation from validation command execution. Do not frame the Phase 1 validation-evidence limits as a no-network or no-external-source rule unless the maintainer explicitly imposes one.

The plan should identify which claims require sources from:

```text
go.dev
pkg.go.dev
csrc.nist.gov
nvlpubs.nist.gov
owasp.org
cheatsheetseries.owasp.org
docs.github.com
docs.docker.com
sqlite.org
postgresql.org
docs.aws.amazon.com/AmazonS3/
valkey.io
redis.io
rfc-editor.org
datatracker.ietf.org
doc.traefik.io
developer.apple.com
swift.org
docs.swift.org
Android / Google developer documentation, if Android claims are made
legal primary sources, if recording-law claims are made
```

Use specific documentation pages rather than broad homepages. For the optional backend-support areas, source families should include:

- PostgreSQL official documentation for transactions, constraints, and, when discussed, connection, security, migration, schema, backup, or restore behavior. Example pages include:

  ```text
  https://www.postgresql.org/docs/current/tutorial-transactions.html
  https://www.postgresql.org/docs/current/ddl-constraints.html
  ```

- Amazon S3 official documentation for `PutObject`, `GetObject`, object keys, metadata, and, when discussed, checksums, ETags, multipart behavior, consistency, or authentication behavior. Example pages include:

  ```text
  https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html
  https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html
  https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-keys.html
  https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingMetadata.html
  https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html
  ```

- Provider-specific S3-compatible documentation only when the reviewed repository names that provider or the report discusses provider-specific behavior, for example MinIO.
- Valkey official documentation for `SET`, `EXPIRE`, TTL/expiry behavior, and short-lived coordination semantics when discussed. Example pages include:

  ```text
  https://valkey.io/commands/set/
  https://valkey.io/commands/expire/
  ```

- Redis official documentation for Redis protocol or Redis-compatible behavior when discussed. An example page is:

  ```text
  https://redis.io/docs/latest/develop/reference/protocol-spec/
  ```

- `pkg.go.dev` pages for the exact Go PostgreSQL, S3/object-storage, and Valkey/Redis client packages used by the reviewed implementation when behavior depends on those packages. Current examples may include:

  ```text
  https://pkg.go.dev/github.com/jackc/pgx/v5
  https://pkg.go.dev/github.com/jackc/pgx/v5/stdlib
  https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn
  https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws
  https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3
  https://pkg.go.dev/github.com/redis/go-redis/v9
  ```

### Validation Evidence

Identify validation evidence that was supplied or missing, such as:

```text
GitHub Actions run URLs
local command output
Codex command output
uploaded validation summaries
gofmt output
go test output
go vet output
Docker build output
simulator smoke test output
PostgreSQL metadata backend test, smoke-test, or migration output
S3-compatible storage test, smoke-test, or object-store integration evidence
Valkey/Redis-compatible coordination test, smoke-test, or TTL behavior evidence
attestation verification output
```

If validation evidence is not supplied, the Phase 1 report must say the command or backend path was not independently executed or verified by Deep Research. For optional PostgreSQL metadata, S3-compatible storage, and Valkey/Redis-compatible coordination, distinguish code/docs review from live disposable-service execution or supplied smoke-test evidence.

## Source Registry Planning

The Phase 0 plan must explain how Phase 1 will build the Source Registry before writing findings.

The plan should include the required registry sections from Phase 1:

```markdown
## Source Registry

### Repository sources inspected

### External authoritative sources consulted

### Validation and execution evidence

### Sources, checks, and commands not available or not executed

### Generated artifacts and report outputs
```

Every registry entry should include source ID, source type, location, commit/ref/date, review purpose, status, limitations, and related finding IDs or report sections where applicable.

For optional backend-support areas, the plan should include example `S-*` keys for required authoritative sources. These are examples only; Phase 1 should use exact source keys for the specific sources actually consulted:

```text
S-POSTGRES-TRANSACTIONS
S-POSTGRES-CONSTRAINTS
S-POSTGRES-CONNECTIONS
S-S3-PUTOBJECT
S-S3-GETOBJECT
S-S3-OBJECT-KEYS
S-S3-CHECKSUMS
S-VALKEY-SET
S-VALKEY-EXPIRE
S-REDIS-PROTOCOL
S-GO-POSTGRES-CLIENT
S-GO-S3-CLIENT
S-GO-VALKEY-CLIENT
```

## Citation Planning

The Phase 0 plan must restate that final public citations are handled through portable citation keys, not ChatGPT UI citation tokens.

Portable key families:

```text
R-* for repository sources pinned to <REVIEWED_COMMIT_SHA>
S-* for external authoritative sources
I-* for issue, PR, or report-follow-up references
V-* for validation evidence
```

Repository URLs must be pinned to `<REVIEWED_COMMIT_SHA>`, not `main`, `develop`, or a moving release branch.

If ChatGPT-rendered citations appear in the Phase 1 draft, the plan must instruct Phase 2 Codex validation to convert or replace them using the Source Registry.

## Branch And Issue-Scope Planning

If Phase 1 may suggest follow-up issues, the Phase 0 plan must restate the Phase 1 branch-scope rules.

The plan should require follow-up issue suggestions to distinguish:

- release blockers for the reviewed branch
- non-blocking follow-ups after merge
- findings that require revalidation on `main` or `develop`
- planning-only findings
- sensitive items that should not become public issues

Any Phase 2-generated public issue drafts must include:

```text
Priority
Type
Labels
Branch scope
```

## Report Scope Planning

The Phase 0 plan should map the Phase 1 report to these broad areas:

1. Documentation consistency, Proofline naming, and compatibility-name notes
2. Current implementation versus future incident-mode planning
3. Go backend structure and idiomatic implementation
4. HTTP API behavior and private/public route separation
5. Viewer/incident token generation, hashing, storage, expiry, redaction, and viewer behavior
6. Logging, metrics, proxy examples, and sensitive data exposure
7. Upload handling, hash verification, immutable storage, upload limits, and stream-scoped chunk identity
8. SQLite and PostgreSQL metadata backend support, including configuration, schema/migrations, transactions, constraints, parity with SQLite where claimed, data-integrity boundaries, and validation-evidence limits
9. Local and S3-compatible blob/object storage support, including object key construction, upload/download behavior, immutability assumptions, hash/checksum handling, local-vs-object-store parity where claimed, path/key traversal risks, and fail-closed behavior
10. Optional Valkey/Redis-compatible coordination, including key expiry, short-lived coordination semantics, lock/session assumptions, failure behavior, fallback behavior, and avoiding persistence or security overclaiming
11. ZIP bundle generation, manifest completeness, fail-closed behavior, and path traversal handling
12. Crypto-adjacent simulator envelope and ciphertext-only backend boundary
13. Future key custody, browser/client-side decryption, break-glass, trusted-contact access, and server escrow boundaries
14. Future web/iOS/Android/protocol/client planning and platform assumptions
15. Deployment guidance, Traefik examples, WireGuard/private boundary, and rate limiting
16. Docker/GHCR/GitHub Actions/supply-chain hygiene
17. Public issue/report safety

## Risks, Ambiguities, And Maintainer Decisions Planning

The Phase 0 plan should identify maintainer decisions needed before Phase 1, including whether PostgreSQL metadata, S3-compatible object storage, and Valkey/Redis-compatible coordination should be treated as release-blocking v0.8.0 review areas or optional backend-support review areas.

The plan should also identify whether validation evidence exists for those backend paths. Absence of supplied validation evidence must be recorded as a limitation rather than guessed around.

## Output Format

Return only a Phase 0 preflight response with this structure:

```markdown
# Phase 0 Deep Research Preflight

## Loaded prompt

- Repository:
- Product name / compatibility-name notes:
- Reviewed branch/ref:
- Reviewed commit SHA:
- Target release/version:
- Phase 1 prompt path:
- Phase 1 prompt source used:
- Target report path:

## Summary of governing Phase 1 rules

## Repository source plan

## External authoritative source plan

## Validation evidence plan

## Source Registry plan

## Citation plan

## Branch and issue-scope plan

## Public-safety restrictions

## Current implementation vs future planning boundary

## Expected report structure

## Risks, ambiguities, or maintainer decisions needed before Phase 1

## Proposed Phase 1 execution plan

## Stop point

Ready for maintainer approval before running Phase 1.
```

Do not continue into Phase 1 until the maintainer explicitly approves the plan.

## Follow-Up Instruction After Maintainer Approval

After Phase 0 returns the plan, the maintainer may approve it with a short instruction such as:

```text
Approved. Run Phase 1 using the loaded `docs/reports/prompts/phase-1-deep-research-technical-review.md` as the governing prompt.

Reviewed branch/ref: `<REVIEWED_BRANCH_OR_REF>`
Reviewed commit SHA: `<REVIEWED_COMMIT_SHA>`
Target release/version: `<TARGET_RELEASE_OR_VERSION>`
Review date: `<YYYY-MM-DD>`

Follow the Source Registry, citation, validation-evidence, branch-scope, and public-safety rules from the loaded Phase 1 prompt.
```

Only after that approval should Deep Research produce the Phase 1 technical review report.
