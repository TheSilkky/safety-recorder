# Codex Prompt: Readability Maintenance

Review the Go backend for readability and maintainability.

Do **not** add features.
Do **not** change endpoint behaviour.
Do **not** add a web framework unless explicitly requested.
Do **not** reformat unrelated documentation or prompt files.

## Goal

Make the code easier for a human to understand and debug while preserving behaviour.

## Source of truth

Before making changes, read current source-of-truth files as relevant:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
- `docs/v1-preview-direction.md`
- `docs/development.md`, especially `Go Readability Standards`
- relevant files in `docs/`
- relevant source files
- relevant tests
- relevant issue or PR, if this is issue/PR work

Do not rely on stale assumptions from this prompt if the repository has changed.

If this prompt and `docs/development.md` disagree about readability standards, treat the current source-of-truth docs and code as authoritative, then update this prompt as part of the docs/process change.

## Global constraints

- Keep changes scoped to the task.
- Do not add unrelated features.
- Do not weaken security warnings.
- Do not claim production readiness.
- Do not expose `/v1` as an unreviewed public catch-all; keep `/v1/admin/...` and `/admin` off public edges.
- Do not log raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, wrapped-key ciphertext, private deployment details, stored paths, object keys, user safety data, or future token-like values; check logging changes against `docs/logging-requirements.md`.
- Do not add React, Node, npm, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features unless explicitly requested.
- Prefer Go standard library where practical.
- Preserve private/public listener separation.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour as an incidental implementation detail.
- Any key custody/decryption change must be an explicit security-sensitive task that updates the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.

## Go readability standards

Follow `docs/development.md#go-readability-standards` for package shape, function naming, comments, error handling, behaviour-preserving refactors, tests, and review evidence.

In particular, readability-only work should:

- preserve current behaviour and public API shape
- keep package and file splits aligned with documented responsibilities
- use boring, descriptive helper names that explain domain steps
- comment non-obvious invariants rather than ordinary Go syntax
- keep security-sensitive invariants visible after refactors
- keep validation and cleanup close to the data or temporary resource being validated
- summarize behaviour-preservation reasoning in the final output

## Focus on

- splitting overly large files when the package already owns multiple distinct concerns
- clearer handler names
- clearer route registration
- clearer package responsibilities
- reducing duplicated request/response helpers
- comments around non-obvious logic
- private API and incident viewer separation
- upload/storage/hash-verification readability
- stream/bundle logic readability
- encryption-envelope and simulator-key handling readability
- simulator readability
- tests that clearly describe behaviour

## Do not

- change public JSON field names
- change database schema unless required for a bug
- change token/security model
- change encryption envelope format
- change key custody/decryption behaviour
- change bundle format
- add new dependencies unless the maintainer explicitly requests them

## Validation

After changes:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

For documentation-only readability standards changes, inspect the Markdown diff and links manually. Go tests are not required unless code changed.

## Output

Summarize:

1. files changed
2. readability changes made
3. behaviour-preservation notes
4. validation commands run
5. any follow-up work that should become an issue
