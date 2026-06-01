# Codex Prompt: Release Check

Review the repo before tagging or publishing a release.

Do **not** add features.
Do **not** make broad refactors.
Do **not** change application behaviour unless required to fix a release-blocking bug.

## Inputs

Target release / version:

```text
<TARGET_RELEASE_OR_VERSION>
```

Current release-prep branch, if applicable:

```text
<TARGET_RELEASE_BRANCH>
```

Target final base branch:

```text
<TARGET_FINAL_BASE_BRANCH>
```

Examples:

```text
main
```

If this release is staged through a release-prep branch, verify that the branch is intended to merge into the target final base branch before tagging.

## Goal

Confirm the repo is ready for a tagged release.

This is a final pre-release check for correctness, documentation, security warnings, tests, build metadata, release notes, branch/PR targeting, and accidental committed junk.

## Source of truth

Before making changes, read current source-of-truth files as relevant:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
- relevant files in `docs/`
- relevant source files
- relevant tests
- relevant issue or PR, if this is issue/PR work

Do not rely on stale assumptions from this prompt if the repository has changed.

## Branch and PR targeting checks

Run:

```bash
git status --short --branch --untracked-files=all
git branch --show-current
git rev-parse HEAD
```

If this is a release-prep branch, verify:

- current branch matches `<TARGET_RELEASE_BRANCH>`
- release PR target should be `<TARGET_FINAL_BASE_BRANCH>`
- any draft PRs created from this branch use the intended base branch
- branch-scoped issue drafts are not treated as global issues without revalidation
- no prompt or PR body assumes `main` unless `main` is the intended target branch

## Global constraints

- Keep changes scoped to the task.
- Do not add unrelated features.
- Do not weaken security warnings.
- Do not claim production readiness.
- Do not expose `/v1` publicly.
- Do not log raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, or future token-like values.
- Do not add React, Node, npm, OAuth, JWT, user accounts, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features unless explicitly requested.
- Prefer Go standard library where practical.
- Preserve private/public listener separation.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour as an incidental implementation detail.
- Any key custody/decryption change must be an explicit security-sensitive task that updates the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the iPhone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.

## Release checklist

Check:

- all tests pass
- `gofmt` has been run
- `go vet` passes, if practical
- README version/scope is accurate
- `CHANGELOG.md` includes the release
- `LICENSE` exists and matches the documented SPDX identifier
- `SECURITY.md` exists and does not promise production readiness
- `README.md` links to `LICENSE` and `SECURITY.md`
- `docs/api.md` matches implemented routes
- `docs/code-map.md` matches package layout
- `docs/encryption.md` matches envelope implementation
- `docs/key-custody.md` or equivalent design docs match current roadmap, if present
- simulator encryption defaults/key-file behaviour are documented
- `docs/threat-model.md` or `docs/security-model.md` matches current security assumptions
- `docs/codex-change-control.md` and `codex/README.md` match prompt workflow, if present
- issue/PR/backlog workflow prompts are listed in `codex/README.md`
- PR creation/review prompts support non-`main` base branches where applicable
- Docker/GHCR notes are current
- GitHub Actions workflow names and badges are correct
- environment variable docs match implementation
- bind address variables are documented
- main/private-admin listener separation is documented
- main `/v1` API exposure warnings are clear
- incident viewer token behaviour is documented
- completed evidence bundle limitations are documented
- simulator commands still work
- no raw secrets/tokens are committed
- no simulator key files are committed
- no generated binaries are committed
- no local SQLite database files are committed
- no uploaded blob data is committed
- no temporary files are committed
- no stale generated artifacts are committed
- no accidental `.env` files are committed
- `.backlog-drafts/` contents are intentional, or excluded if they are local-only drafts

## Security review items

Confirm:

- private write/admin routes are blocked from public viewer edges
- public incident viewer routes are read-only
- raw incident tokens are not logged
- request bodies are not logged
- uploaded file bytes are not logged
- Authorization headers are not logged
- plaintext and raw keys are not logged
- backend key custody/decryption posture is documented accurately
- ZIP download routes do not expose filesystem paths
- ZIP entry names are controlled by the server
- ZIP downloads set safe headers
- token-protected incident viewer pages and downloads use `Cache-Control: no-store`
- incident viewer responses use `Referrer-Policy: no-referrer`
- incident viewer responses use `X-Content-Type-Options: nosniff`
- HSTS is not enabled by default for localhost/dev HTTP unless explicitly gated by config
- documentation does not claim production readiness

## Commands

From the repository root, run:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
```

If `go vet ./...` fails because of a known harmless issue, document the reason rather than silently ignoring it.

## Manual smoke tests

If practical, run the backend:

```bash
SAFE_AUTH_BOOTSTRAP_SECRET='replace-with-local-bootstrap-secret' go run ./cmd/api
```

In another terminal, create the first local admin if the test database does not
already have one:

```bash
curl -sS -X POST http://127.0.0.1:8081/admin/bootstrap \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'bootstrap_secret=replace-with-local-bootstrap-secret' \
  --data-urlencode 'username=admin' \
  --data-urlencode 'password=replace-with-a-long-local-password'
```

Then run the simulator with account credentials:

```bash
PROOFLINE_SIM_USERNAME='admin' \
PROOFLINE_SIM_PASSWORD='replace-with-a-long-local-password' \
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

Confirm:

- the simulator creates an incident
- the simulator creates an incident token
- chunks upload successfully
- checkins are sent
- the stream completes
- the incident viewer URL works
- completed-stream download buttons appear
- encrypted bundle download works
- local decrypt verification succeeds
- no plaintext or keys are printed

## Release notes

If the release is ready, prepare or verify a `CHANGELOG.md` entry.

The entry should include:

- added features
- changed behaviour
- fixed bugs
- security-relevant changes
- documentation changes
- known limitations

Do not overstate stability or production readiness.

## Output format

Return:

1. Release readiness: ready / not ready
2. Current branch and intended final base branch
3. Blocking issues
4. Non-blocking issues
5. Tests and commands run
6. Documentation updates needed
7. Suggested version tag
8. Suggested changelog entry
9. Any backlog follow-ups

If you make fixes:

- keep them minimal
- explain what changed
- run validation again
