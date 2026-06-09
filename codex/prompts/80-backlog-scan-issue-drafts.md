# Codex Prompt: Backlog Scan and Branch-Scoped Issue Drafts

Scan the repository and create reviewed backlog issue drafts.

This prompt is reusable. It must discover the current repo state each time it runs rather than relying on stale hard-coded candidate issues.

Do **not** change application code.
Do **not** change application behaviour.
Do **not** create GitHub issues directly in this task.
Do **not** run `gh issue create`.
Do **not** modify workflows, Dockerfiles, SQL migrations, Go code, generated files, binaries, database files, or uploaded blob data.
Do **not** add features.

## Goal

Review the current repository, current documentation, current issue backlog, current branch, and recent merged work.

Produce a small set of high-quality branch-scoped backlog issue drafts for future work.

The drafts should be specific enough that the maintainer can review them and later create GitHub issues manually or with GitHub CLI.

## Repository

```text
open-proofline/server
```

## Branch scope

Before scanning, record:

```bash
git status --short --branch --untracked-files=all
git branch --show-current
git rev-parse HEAD
```

Use the current checked-out branch as the draft issue scope.

Create a filesystem-safe branch slug by replacing `/` and other non-alphanumeric separators with `-`.

Examples:

```text
release/v0.5.0-prep -> release-v0.5.0-prep
develop -> develop
feature/foo -> feature-foo
```

If running from a detached HEAD, use the reviewed branch/ref supplied by the maintainer. If no branch/ref is available, stop and ask for the intended branch scope.

## Source of truth to inspect

Read current repository files where present:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `LICENSE`
- `docs/README.md`
- `docs/v1-preview-direction.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/configuration.md`
- `docs/deployment.md`
- `docs/encryption.md`
- `docs/key-custody.md`, if present
- `docs/browser-decryption.md`, if present
- `docs/break-glass-key-access.md`, if present
- `docs/ios-local-recorder-prototype.md`, if present
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/code-map.md`
- `docs/development.md`
- `docs/codex-change-control.md`, if present
- `internal/config`
- `internal/db`
- `internal/httpapi`
- `internal/incidents`
- `internal/storage`
- `internal/envelope`
- `cmd/api`
- `cmd/simclient`
- `.github/workflows`
- `.github/ISSUE_TEMPLATE`, if present
- `codex/prompts`

Also inspect current GitHub issues, PRs, and labels if GitHub CLI is available:

```bash
gh issue list --repo open-proofline/server --state all --limit 100
gh pr list --repo open-proofline/server --state all --limit 50
gh label list --repo open-proofline/server --limit 200
```

If GitHub CLI is unavailable, continue using local repo files and mention that issue, PR, and label validation were limited.

## Existing issue duplicate check

Before drafting any issue:

1. Check whether an existing open issue already covers it.
2. Check whether a closed issue or merged PR recently completed it.
3. Check whether the finding is branch-specific and may disappear after merge/rebase.
4. If an existing issue covers it, do not create a duplicate draft.
5. If an existing issue is close but incomplete, create a draft suggesting an update/comment instead of a duplicate issue.
6. If uncertain, include a note in the scan index rather than generating a duplicate issue.

## Areas to scan

Look for future work in these categories:

1. Correctness
2. Security hardening
3. Deployment hardening
4. Database/migration maturity
5. Configuration
6. Testing gaps
7. Documentation gaps
8. Simulator/dev tooling
9. iOS-client prerequisites
10. Operational readiness
11. Release/CI polish
12. Codex workflow/process improvements
13. Key custody / emergency access design
14. Branch/release-candidate follow-up work

## Candidate discovery guidance

Do not blindly recreate the same backlog every run.

Good candidate signals:

- documented known gaps
- TODO/FIXME comments
- missing validation compared with docs
- docs saying “future work”
- security model gaps
- threat-model mitigations not implemented
- key custody or emergency access design gaps
- code paths with tests missing
- deployment warnings without examples
- workflows mentioned in docs but not automated
- simulator gaps against intended client flow
- open issues that need splitting/refinement
- recently merged work that introduced follow-up tasks

Bad candidate signals:

- vague “improve code”
- duplicate of existing issue
- feature that contradicts README/AGENTS scope
- production claims beyond current maturity
- public issue containing sensitive vulnerability details
- anything requiring secrets, raw tokens, or private deployment details

## Sensitive findings

If you find a likely security vulnerability that should not be public:

- Do not create a normal public issue draft for it.
- Create a private note under the branch-scoped private notes directory:

```text
.backlog-drafts/YYYY-MM-DD/<branch-slug>/private-notes/
```

If the date is not available, use:

```text
.backlog-drafts/current/<branch-slug>/private-notes/
```

- Clearly mark it:

```text
PRIVATE SECURITY NOTE - DO NOT CREATE PUBLIC ISSUE
```

- Do not include raw tokens, secrets, user safety data, private deployment details, or exploit payloads.
- Refer to `SECURITY.md` for reporting/handling.

## Output directory

Create a branch-scoped timestamped draft directory:

```text
.backlog-drafts/YYYY-MM-DD/<branch-slug>/
```

If the date is not easily available, use:

```text
.backlog-drafts/current/<branch-slug>/
```

Inside it, create one Markdown file per proposed issue.

Filename format:

```text
NNN-short-kebab-title.md
```

Also create:

```text
.backlog-drafts/YYYY-MM-DD/<branch-slug>/README.md
.backlog-drafts/YYYY-MM-DD/<branch-slug>/private-notes/README.md
```

or:

```text
.backlog-drafts/current/<branch-slug>/README.md
.backlog-drafts/current/<branch-slug>/private-notes/README.md
```

The index should list drafted issues grouped by priority/category, include skipped duplicates, and state the branch scope. The private-notes README should state that private notes must never be used for public issue creation. Do not create private note files unless a finding is unsafe for a public draft.

## Number of issues

Prefer quality over volume.

Default target:

```text
5 to 12 high-quality issue drafts
```

If fewer than 5 good issues exist, create fewer and say why.

If more than 12 exist, include only the highest-value drafts and list lower-priority candidates in the index as “future scan candidates.”

## Required issue draft format

Each public issue draft must use this structure and section order:

```md
# <Issue title>

## Priority

P0 / P1 / P2 / P3

## Type

bug / maintenance / security-hardening / documentation / feature / deployment / ci / testing / planning

## Labels

Suggested GitHub labels:

- `backlog`
- one or more existing topic/type labels, for example: `bug`, `maintenance`, `security`, `docs`, `deployment`, `testing`, `ios`, `ci`

## Branch scope

- Current branch: `<CURRENT_BRANCH>`
- Current HEAD: `<CURRENT_HEAD>`
- Target branch, if known: `<TARGET_BRANCH_OR_UNKNOWN>`
- Reviewed branch/ref, if applicable: `<REVIEWED_BRANCH_OR_REF_OR_NOT_APPLICABLE>`
- Reviewed commit SHA, if applicable: `<REVIEWED_COMMIT_SHA_OR_NOT_APPLICABLE>`
- Target release/version, if applicable: `<TARGET_RELEASE_OR_VERSION_OR_NOT_APPLICABLE>`
- Scope classification: `release-blocker-current-branch` / `follow-up-after-merge` / `revalidate-on-main-or-develop` / `planning-only`
- Scope note: This draft was generated from the branch above. Revalidate against the target branch before creating or closing public GitHub issues if the branch has moved or has not yet merged.

## Summary

One or two sentences.

## Context

Why this matters and what repo files/docs support it.

Mention existing related issues or PRs if relevant.

## Proposed change

What should change.

## Acceptance criteria

- [ ] ...
- [ ] ...
- [ ] ...

## Tests / validation

- [ ] `gofmt -w ./cmd ./internal ./migrations`, if Go files change
- [ ] `go test ./...`, if code changes
- [ ] `go vet ./...`, if code changes or CI/testing changes
- [ ] `git diff --check`, if files change
- [ ] simulator smoke test, if relevant
- [ ] docs updated, if relevant
- [ ] revalidate on target branch before public issue creation, if branch-scoped

## Out of scope

What this issue must not include.

## Notes

Any references to files, docs, related issues, related PRs, branch scope, or future work.
```

## Priority guide

Use:

```text
P0 = urgent correctness/security issue that should be handled before further feature work
P1 = important before real-world/private deployment or before iOS work
P2 = useful near-term improvement
P3 = polish, documentation, cleanup, or governance
```

Do not overuse P0.

## Priority and label requirements

- Do not create any public issue draft without `## Priority`.
- Do not create any public issue draft without `## Type`.
- Do not create any public issue draft without `## Labels`.
- Do not create any public issue draft without `## Branch scope`.
- Every public issue draft must include the `backlog` label.
- Every public issue draft must include at least one additional type/topic label.
- Keep labels as backtick-wrapped Markdown bullets.
- Do not invent new labels unless the maintainer explicitly requested label creation.
- If GitHub CLI is available, compare draft labels against `gh label list`.
- If no suitable label exists, use the closest existing label and note the uncertainty under `## Notes`.

Likely existing labels include:

- `backlog`
- `bug`
- `maintenance`
- `security`
- `docs`
- `deployment`
- `testing`
- `ios`
- `ci`

Repository labels can change. If GitHub CLI is available, use only labels that
exist in `gh label list`. If a good topic label such as `simulator` or
`planning` does not exist, use the closest existing label and note the mismatch
under `## Notes`; do not invent or create labels during this task.

## Requirements

- Keep issues specific and actionable.
- Do not create huge umbrella issues unless they are clearly planning/docs issues.
- Do not include secrets, raw tokens, private deployment info, or user safety data.
- Do not include exploit details in public issue drafts.
- Do not include issue drafts for work already completed by current code.
- Do not include issue drafts already covered by open issues.
- Do not add jokes or informal commentary to issue drafts.
- Do not claim the project is production-ready.
- Include branch scope in every issue draft.
- Include priority, type, and GitHub label metadata in every issue draft.
- Keep all output as Markdown.

## Validation

After creating drafts:

```bash
git diff --stat
git diff -- .backlog-drafts
git diff --check
```

Check required metadata:

```bash
python3 - <<'PY'
from pathlib import Path
import sys

bad = []
required = ["## Priority", "## Type", "## Labels", "## Branch scope"]
for path in Path(".backlog-drafts").rglob("*.md"):
    if path.name in {"README.md", "create-issues-review.md"} or "private-notes" in path.parts:
        continue
    text = path.read_text(encoding="utf-8")
    missing = [section for section in required if section not in text]
    if missing:
        bad.append((str(path), missing))
    if "## Labels" in text and "`backlog`" not in text:
        bad.append((str(path), ["missing `backlog` label"]))

for path, missing in bad:
    print(path, "missing:", ", ".join(missing))

if bad:
    sys.exit(1)
PY
```

Do not run Go tests unless code was changed.

If any files outside `.backlog-drafts/` changed, stop and explain why.

## Output

Summarize:

1. current branch and HEAD
2. draft directory created
3. number of issue drafts created
4. priority breakdown
5. labels used
6. categories covered
7. existing issues/PRs checked
8. duplicates skipped
9. branch-scoped drafts that require revalidation on target branch
10. issues that should be reviewed before public creation
11. sensitive/private notes created, if any
12. suggested next command for reviewing or creating GitHub issues manually
