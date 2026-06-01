# Development

Proofline currently contains a Go backend only. Keep changes small, boring, and testable. The Go module path is `github.com/open-proofline/server` at the repository root, release binaries use `proofline-server-*` names, and the published GHCR image is `ghcr.io/open-proofline/server`.

Compatibility identifiers such as the v1 simulator encryption envelope and default SQLite filename may still use earlier `safety-recorder` names until separate protocol or data-layout migrations are explicitly performed.

## AI Assistance

This repository uses OpenAI Codex as an AI-assisted development tool. Codex may generate or modify code and documentation, but changes are accepted only after maintainer review and testing.

The maintainer remains responsible for correctness, security, licensing, releases, deployment decisions, and real-world use. Use of Codex does not imply endorsement, audit, certification, or maintenance by OpenAI.

For rollback points, scoped prompts, review steps, and backlog handling, see [codex-change-control.md](codex-change-control.md).

## Repository Layout

```text
go.mod            root Go module for the server repository
cmd/api           API server entry point
cmd/simclient     simulator CLI
internal/config   environment configuration, backend selectors, and HTTP timeout parsing
internal/db       SQLite setup, schema_migrations, and compatibility migrations
internal/envelope client-side chunk encryption envelope helpers
internal/httpapi  HTTP handlers, muxes, middleware, bundles, web assets
internal/incidents incident, stream, chunk, checkin, and token models plus SQLite repository code
internal/postgresdb optional PostgreSQL metadata setup, migrations, and repository code
internal/storage  blob-store boundary, local immutable storage, and optional S3-compatible storage
migrations        embedded SQLite schema
migrations/postgres embedded PostgreSQL schema
compose/          local Docker Compose release-smoke stacks and runner script
docs/              project documentation
docs/reports/      public technical review reports and report prompts
.dockerignore      root Docker build-context ignore file for Dockerfile
```

See [code-map.md](code-map.md) for a package-level walkthrough.

## Technical Review Reports

Public technical review reports live in [reports/](reports/). They are
AI-assisted engineering review artifacts reviewed by the maintainer, not formal
security audits, penetration tests, compliance certifications, or production
readiness endorsements.

Use the Phase 1 prompt in
[reports/prompts/phase-1-deep-research-technical-review.md](reports/prompts/phase-1-deep-research-technical-review.md)
to create a source-cited draft outside Codex. Use
[../codex/prompts/95-validate-deep-research-report.md](../codex/prompts/95-validate-deep-research-report.md)
for the Phase 2 Codex cleanup and public-safety validation pass.

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

PostgreSQL metadata integration tests are opt-in so the default local test
suite does not require a database:

```bash
SAFE_POSTGRES_TEST_DSN='<test database DSN>' go test ./internal/postgresdb -count=1
```

The PostgreSQL tests create and drop isolated schemas inside the configured
database. Use a disposable local database or a dedicated test database only.
For a one-off Docker container example, see
[PostgreSQL metadata migration path](postgresql-metadata-migration.md#testing-expectations).
CI also runs these package tests in the `PostgreSQL metadata tests` job against
a disposable PostgreSQL service using fixed CI-only credentials. That job is an
additional integration signal; making it a required branch check is a separate
maintainer ruleset decision.

Local Docker Compose smoke stacks for release preparation live in
[../compose/](../compose/). They exercise the simulator against disposable
SQLite/local, PostgreSQL/local, SQLite/S3-compatible MinIO, and full
PostgreSQL/MinIO/Valkey backend combinations. These stacks are local
development helpers, use fixed test credentials, publish API ports on loopback
by default, and do not make Proofline production-ready public infrastructure.

## Go Readability Standards

Readability-only work should make the Go backend easier to inspect, debug, and safely modify while preserving current behaviour. It must not introduce features, dependency changes, schema changes, route changes, security-model changes, or production-readiness claims.

### Package And File Shape

Keep packages aligned with the responsibilities documented in [code-map.md](code-map.md). Split large files when a package already owns several distinct concepts, but do not create new packages unless the responsibility boundary is clearer than the existing package boundary.

Preferred patterns:

- keep incident, chunk, stream, checkin, and incident-token repository methods grouped by concern
- keep HTTP handlers, request parsing, response shaping, and viewer summaries easy to locate
- keep file names boring and descriptive, such as `chunks.go`, `checkins.go`, or `incident_tokens.go`
- keep exported API surface stable unless the task explicitly requires an API change

Avoid moving code only to make a diff look cleaner. A split should help future reviewers find related invariants, queries, handlers, or tests.

### Function Shape And Naming

Prefer small functions with one clear reason to exist. Names should describe the project behaviour they protect, not generic control flow.

Preferred examples:

- `validateChunkInsertState`
- `parseChunkTimeRange`
- `collectIncidentViewerChunkStats`
- `summarizeIncidentViewerStreams`
- `incidentTokenExpiresAt`

Avoid vague names such as `processData`, `handleThing`, or `doCheck`. If a helper only exists to hide complexity, give it a name that explains the domain step being performed.

Keep the main handler or repository method readable as an ordered story:

1. read or validate inputs
2. load required state
3. enforce invariants
4. perform the write or response transformation
5. return a stable response or typed error

### Behaviour-Preserving Refactors

Readability refactors should preserve:

- HTTP methods, route paths, status codes, JSON field names, and error codes
- database schema, migration history, and stored data meaning
- token creation, hashing, expiry, revocation, and public error-collapsing behaviour
- encryption envelope and key-custody assumptions
- bundle format, ZIP entry naming, and encrypted evidence-bundle semantics
- main `/v1`/viewer and private-admin route separation
- logging exclusions for raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, and future token-like values

When a refactor touches security-sensitive paths, keep the old invariant visible in the new shape. For example, incident viewer code should still make it obvious that invalid, expired, and revoked tokens collapse into the same public error, and upload code should still make the temp-file, hash-verification, immutable-commit, and metadata-write order easy to follow.

### Comments And Invariants

Comments should explain non-obvious project invariants, security boundaries, race handling, or compatibility decisions. Do not comment ordinary Go syntax.

Good comments explain why behaviour exists, for example:

- why raw incident tokens are returned once and only hashes are stored
- why streamed chunks require positive indexes while legacy unstreamed uploads may keep index `0`
- why the schema unique constraint remains the final duplicate guard after HTTP preflight checks
- why viewer summaries must not expose stored paths or encrypted file bytes

If a comment would become stale as soon as a helper is renamed, prefer a clearer helper name instead.

### Error Handling And Validation

Keep validation close to the data it validates. Preserve typed sentinel errors where callers depend on them, and wrap unexpected errors with enough context for debugging.

Preferred patterns:

- return repository sentinel errors such as `ErrNotFound`, `ErrDuplicate`, `ErrInvalidState`, and `ErrIncidentClosed` for expected domain failures
- keep HTTP request parsing helpers responsible for user-facing `400` and `413` responses
- keep internal error logging behind `internalError` so sensitive request data is not logged incidentally
- clean up temporary uploads on failed parsing, duplicate file fields, oversize requests, or invalid metadata

Do not collapse distinct internal error contexts into vague messages just to shorten code.

### Tests And Review Evidence

Tests should describe the behaviour being protected. Prefer table-driven tests when they make edge cases clearer, and use test names that read like behaviour statements.

After Go readability changes, run from the repository root:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
```

For behaviour-sensitive handler, storage, stream, bundle, or simulator changes, also consider the simulator smoke test documented in [codex-change-control.md](codex-change-control.md). For documentation-only readability standards changes, inspect the Markdown diff and links manually.

### Codex Readability Tasks

Reusable readability prompts should reference this section before proposing code changes. If this section and a prompt disagree, treat current source-of-truth docs and code as authoritative, then update the prompt as part of the docs/process change.

Codex output should summarize:

1. files changed
2. readability changes made
3. behaviour-preservation notes
4. validation commands run
5. follow-up work that should become an issue instead of expanding the diff

## Documentation Checks

When editing docs, keep these claims aligned:

- current name, version, and scope in [../README.md](../README.md)
- planned incident modes in [incident-modes.md](incident-modes.md)
- route details in [api.md](api.md)
- security assumptions in [security-model.md](security-model.md) and [threat-model.md](threat-model.md)
- future key custody/decryption designs in [key-custody.md](key-custody.md), [browser-decryption.md](browser-decryption.md), and [break-glass-key-access.md](break-glass-key-access.md)
- package layout in [code-map.md](code-map.md)
- release notes in [../CHANGELOG.md](../CHANGELOG.md)

Do not claim production readiness unless deployment hardening has actually been implemented. Do not treat protocol or data-layout compatibility names as stale just because the repository, module, Docker image, and GHCR artifact names now use the Proofline namespace.

## Backlog Discipline

New ideas discovered during unrelated work should become issues or backlog items unless they are required to finish the current task. Capture the context, acceptance criteria, tests, docs impact, and out-of-scope items instead of expanding the active diff.

Local backlog drafts are review artifacts. Keep them branch-scoped under `.backlog-drafts/YYYY-MM-DD/<branch-slug>/` or `.backlog-drafts/current/<branch-slug>/`, then treat GitHub Issues as the source of truth after reviewed drafts are created publicly.

## Branch Protection And Required Checks

This repository uses GitHub repository rulesets rather than classic branch
protection.

Current branch rulesets:

| Ruleset | Target | Purpose |
|---|---|---|
| `Protect main` | `~DEFAULT_BRANCH`, currently `main` | Stable release line. Final release PRs and hotfixes merge here. |
| `Protect develop` | `refs/heads/develop` | Next-release integration branch. Normal issue PRs merge here after `v0.5.0`. |
| `Protect release/v*` | `refs/heads/release/v*` | Short-lived release-prep branches such as `release/v0.6.0-prep`. |

All three rulesets are active and block branch deletion and non-fast-forward
updates. They require pull requests before merge, one approving review, stale
approval dismissal on new pushes, and strict required status checks.

Required checks:

- `Go tests`
- `Build Linux binary`
- `Build Docker image`

The rulesets allow merge, squash, and rebase merge methods. `Protect develop`
and `Protect release/v*` require review thread resolution. `Protect main`
currently does not require review thread resolution.

The exported rulesets include a repository-role bypass actor with bypass mode
limited to pull requests. Use bypass only for maintainer-authored changes when
no independent write-access reviewer is available, after required checks pass
and the maintainer has reviewed the diff. Routine collaborator changes should
still receive a qualifying approval.

Do not require tag-only jobs such as `Attest Linux binary`, `Upload release
binary`, or `Publish Docker image` as pull request status checks. Those jobs run
only for trusted release/tag contexts and would make normal PRs unmergeable if
required on PRs.

If the rulesets change, update this section to match the exported rulesets.

## Branch Model

After `v0.5.0`, use this branch flow:

```text
issue branches -> develop
develop -> release/vX.Y.Z-prep
release/vX.Y.Z-prep -> main
tag vX.Y.Z from main
main -> develop sync after release
```

Branch purposes:

- `main` is the stable release line.
- `develop` is the next-release integration branch.
- `release/v*` branches are short-lived release-candidate branches.
- Final `v*` tags are created from `main`.
- Release-candidate tags may be created from release-prep branches when
  validating release automation.

When creating PRs, set the intended base branch explicitly:

- issue work for the next release: base `develop`
- release-prep fixes: base `release/vX.Y.Z-prep`
- final release PR: base `main`
- hotfixes: base `main`, then sync back to `develop`

## CI And Release Automation

The CI workflow runs on pull requests, all branch pushes, and `v*` tags. Pushes
to `main`, `develop`, and `v*` tags publish Docker image tags to GHCR when
package publishing is available. The `develop` branch publishes the mutable
`develop` image tag plus a SHA tag. Workflow-level token permissions stay
read-only. A tag-only binary attestation job can mint release attestations for
`v*` tag pushes, and the trusted Docker publish job can mint and publish GHCR
image attestations.
For `v*` tags, CI also uploads the Linux amd64 binary as a GitHub Release asset.
`packages: write` is granted only to the trusted Docker publish job.

CI also records lightweight assurance signals for release review. The `Go tests`
job writes a `go-coverage` artifact and includes the `go tool cover` function
summary in the workflow run summary. The coverage output is advisory only; this
repository does not currently enforce a minimum coverage percentage as a merge
gate. The separate `Go vulnerability scan` job runs `govulncheck` against the Go
packages without requiring repository secrets, including on pull requests from
forks. Release binary attestation, Release binary upload, and trusted GHCR image
publishing depend on that scan passing.

Coverage output, `govulncheck`, builds, and artifact attestations are review
signals. They do not prove that an artifact is vulnerability free, suitable for
public production exposure, or safe to deploy with `/v1` reachable from the
public internet.

## Pinned GitHub Actions

External GitHub Actions in [../.github/workflows/ci.yml](../.github/workflows/ci.yml)
are pinned to full 40-character commit SHAs. Keep the intended upstream version
tag in a same-line comment, for example `owner/action@<sha> # v1`, so reviewers
can see the expected release line and Dependabot can update that version
documentation.

GitHub documents full-length commit SHA pinning as the immutable release model
for actions. When updating an action, verify that the SHA belongs to the
upstream action repository and corresponds to the intended tag or release, not a
fork. Review the action release notes, changed inputs, changed permissions, and
the workflow diff before merging.

Dependabot remains enabled for the `github-actions` ecosystem in
[../.github/dependabot.yml](../.github/dependabot.yml). Prefer reviewing those
pull requests rather than bypassing maintainer review. For manual updates, use
`git ls-remote --tags https://github.com/<owner>/<repo>.git 'refs/tags/<tag>' 'refs/tags/<tag>^{}'`
and pin the peeled commit SHA when the tag is annotated. After any action
update, confirm the required PR checks still pass and that trusted GHCR
publication remains limited to `main`, `develop`, and `v*` tag pushes.

## Artifact Attestations

Release provenance attestations are generated by the CI workflow:

- `Attest Linux binary` attests `proofline-server-linux-amd64` on `v*` tag
  pushes.
- `Upload release binary` uploads `proofline-server-linux-amd64` to the matching
  GitHub Release after the binary attestation job passes.
- `Publish Docker image` attaches an attestation to published GHCR images on
  trusted `main`, `develop`, and `v*` tag pushes.

The workflow keeps top-level permissions read-only. `id-token: write` and
`attestations: write` are granted only to jobs that create attestations. The
release binary upload job gets `contents: write` only so it can create a
minimal Release when needed and upload the binary asset. The Docker publish job
also keeps `packages: write` because it pushes images to GHCR. The image
attestation is pushed to the registry without creating a linked artifact storage
record.

For `v*` tag workflows, CI creates a minimal GitHub Release if one does not
already exist, verifies that the tag exists remotely with
`gh release create --verify-tag`, and uploads `proofline-server-linux-amd64` as
a Release asset.
Release-candidate, alpha, and beta tags are marked as prereleases and are not
promoted as the latest release. The workflow does not overwrite existing
Release assets by default; if the binary asset already exists, the upload step
fails so the maintainer can review the existing asset before retrying.

After a release workflow run, verify the Release and asset with:

```bash
gh release view <tag> --repo open-proofline/server
```

After a release workflow run, verify the downloaded binary artifact with:

```bash
gh attestation verify ./proofline-server-linux-amd64 \
  -R open-proofline/server \
  --signer-workflow open-proofline/server/.github/workflows/ci.yml \
  --source-ref refs/tags/<tag>
```

Verify a published container image with:

```bash
docker login ghcr.io
gh attestation verify oci://ghcr.io/open-proofline/server:<tag> \
  -R open-proofline/server \
  --signer-workflow open-proofline/server/.github/workflows/ci.yml
```

Use `--source-ref refs/tags/<tag>` for a release tag or
`--source-ref refs/heads/main` or `--source-ref refs/heads/develop` for a
branch image when you want the CLI to enforce the expected source ref.
Attestation verification confirms provenance from the expected workflow; it
does not prove that an artifact is vulnerability free or production-ready.

## Pinned Docker Base Images

Base images in [../Dockerfile](../Dockerfile) are pinned with
`tag@sha256:<digest>` references. Keep the human-readable tag before the digest
so reviewers can see the intended release family while builds stay tied to an
immutable manifest.

When refreshing a Docker base-image digest, inspect the current manifest for the
intended tag:

```bash
docker buildx imagetools inspect docker.io/library/golang:1.26-alpine
docker buildx imagetools inspect docker.io/library/alpine:3.23
```

Dependabot is enabled for the `docker` ecosystem in
[../.github/dependabot.yml](../.github/dependabot.yml) and monitors the
root Dockerfile. Prefer reviewing those Dependabot pull requests for
routine base-image refreshes.

Update only the digest for the same intended tag unless the issue, Dependabot
pull request, or release explicitly calls for a version-family change. Review
the upstream image tag, version, architecture coverage, release notes or
changelog where available, and the Dockerfile diff before merging. After a
digest update, run the Docker build locally when practical and confirm the
required CI Docker build still passes.

Only create `v*` tags after the release checklist is complete and the tagged
commit has passed CI. If an emergency fix is needed, keep the change narrow,
preserve review discipline, and document any skipped validation in the release
or follow-up notes.

Codex-generated changes should be reviewed like any other contribution: inspect
the diff, confirm the scope matches the issue, run the relevant validation
commands, and make sure security warnings and main/private-admin boundaries
were not weakened.

## Release Checklist

Before tagging:

- run `gofmt -w ./cmd ./internal ./migrations`
- run `go test ./...`
- run `go vet ./...` when practical
- verify README badges and links
- verify `LICENSE` and `SECURITY.md`
- verify Docker/GHCR notes
- run the relevant local Compose smoke stack from [../compose/](../compose/)
  when validating optional PostgreSQL, S3-compatible, or Valkey backend changes
- verify the GitHub Release binary asset plus release binary and GHCR image attestations
- verify Docker base-image digest pins still match the intended tag families
- verify private/public route separation is documented
- verify raw tokens, request bodies, uploaded bytes, and Authorization headers are not logged
- verify no local DBs, uploaded blobs, generated binaries, `.env` files, or temporary files are committed
