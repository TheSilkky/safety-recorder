# Codex Prompt: Work on GitHub Issue

Work on one GitHub issue in this repository.

## Inputs

Issue number: `<ISSUE_NUMBER>`

Repository:

```text
open-proofline/server
```

Target base branch for the eventual PR:

```text
<TARGET_BASE_BRANCH>
```

Examples:

```text
main
develop
release/v0.5.0-prep
```

If no target base branch is supplied, infer it from the maintainer's explicit task wording or the current release workflow. If it is still unclear, stop and ask before making changes.

## Rules

- Use the current checked-out branch.
- Do not create or checkout another branch.
- Do not create a pull request unless explicitly requested.
- Keep the change scoped to the issue.
- Treat `Target base branch` as the branch the work is intended to merge into.
- Revalidate issue scope against the target base branch when the issue was generated from a release-prep or feature branch.
- Do not add unrelated features.
- Do not change public API behaviour unless the issue requires it.
- Do not weaken security warnings.
- Do not expose `/v1` publicly.
- Preserve private/public listener separation.
- If the issue appears security-sensitive, stop and state whether public issue handling is appropriate before making changes.

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

## First steps

Check the current branch, target base branch, and repository state:

```bash
git status --short --branch --untracked-files=all
git branch --show-current
git rev-parse HEAD
git fetch origin "<TARGET_BASE_BRANCH>"
git log --oneline -5
```

Read the issue:

```bash
gh issue view <ISSUE_NUMBER> --repo open-proofline/server
```

Then read:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- relevant files in `docs/`
- relevant source files
- relevant tests

Before changing files, summarize:

1. issue goal
2. target base branch for the eventual PR
3. whether the issue was generated from a different branch/ref
4. acceptance criteria from the issue
5. files likely affected
6. validation commands
7. out-of-scope items
8. whether this should be docs-only, code-only, or mixed
9. whether the issue is safe for normal public issue handling
10. whether the issue changes key custody/decryption assumptions
11. whether the work should be revalidated against the target base branch before PR creation

## Implementation

Make the smallest useful change that satisfies the issue for the intended target base branch.

Keep the diff reviewable.

If you discover unrelated future work, do not implement it. Suggest a backlog issue instead.

If the issue was generated against a release-prep or feature branch and the current target base branch differs from that branch, re-check that the problem still exists against the target base before implementing.

## Validation

If Go code changed:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
```

If only Markdown changed, inspect docs and links manually. Go tests are not required unless code changed.

If behaviour changed and the simulator is relevant:

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

## Output

Summarize:

1. current branch
2. target base branch
3. files changed
4. implementation summary
5. issue acceptance criteria satisfied
6. validation commands run
7. any follow-up work
8. whether a PR should be opened and against which base branch
