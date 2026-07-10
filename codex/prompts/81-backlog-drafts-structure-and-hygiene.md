# Codex Prompt: Backlog Drafts Structure, Branch Scope, Priority, and Label Hygiene

Review `.backlog-drafts/` structure and backlog draft workflow hygiene.

This is a documentation/process maintenance task.

Do **not** change application code.
Do **not** create GitHub issues.
Do **not** close GitHub issues.
Do **not** execute issue creation scripts.
Do **not** delete files unless explicitly requested.
Do **not** modify workflows, Dockerfiles, SQL, Go code, generated binaries, local DBs, or uploaded blob data.

## Goal

Ensure backlog drafts are clearly treated as generated/reviewable local artifacts, not as the source of truth once GitHub issues exist.

Standardize `.backlog-drafts/` structure, naming, branch-scoping, priority metadata, and label metadata.

Check whether reusable prompts enforce that structure.

## Source of truth

Read:

- `README.md`
- `AGENTS.md`
- `docs/codex-change-control.md`, if present
- `codex/README.md`
- `codex/prompts/80-backlog-scan-issue-drafts.md`
- `codex/prompts/85-create-github-issues-from-drafts.md`
- `codex/prompts/82-review-open-issues-for-stale-or-fixed.md`, if present
- `codex/prompts/95-validate-technical-review-report.md`, if present
- `.backlog-drafts/`, if present
- `scripts/create-backlog-issues.sh`, if present
- `.gitignore`

If GitHub CLI is available, inspect repository labels:

```bash
gh label list --repo open-proofline/server --limit 200
```

## Standard `.backlog-drafts/` structure

Preferred generated output for branch-scoped drafts:

```text
.backlog-drafts/
  YYYY-MM-DD/
    <branch-slug>/
      README.md
      001-short-kebab-title.md
      002-short-kebab-title.md
      create-issues-review.md
      private-notes/
        README.md
        001-private-note.md
```

Fallback if date is unavailable:

```text
.backlog-drafts/current/<branch-slug>/
```

Legacy unscoped drafts may still exist, but new drafts should be branch-scoped unless the maintainer explicitly requests a global backlog scan.

## Required public issue draft metadata

Every public issue draft must include:

```md
## Priority

P0 / P1 / P2 / P3

## Type

bug / maintenance / security-hardening / documentation / feature / deployment / ci / testing / planning

## Labels

Suggested GitHub labels:

- `backlog`
- one or more topic/type labels

## Branch scope

...
```

Every public issue draft must include `backlog` under `## Labels`.

Every public issue draft must include at least one additional topic/type label.

Private notes do not need GitHub labels, but must never be used for public issue creation.

## Branch scope metadata

Every public issue draft should include:

```md
## Branch scope

- Current branch: `<CURRENT_BRANCH>`
- Current HEAD: `<CURRENT_HEAD>`
- Target branch, if known: `<TARGET_BRANCH_OR_UNKNOWN>`
- Reviewed branch/ref, if applicable: `<REVIEWED_BRANCH_OR_REF>`
- Reviewed commit SHA, if applicable: `<REVIEWED_COMMIT_SHA>`
- Target release/version, if applicable: `<TARGET_RELEASE_OR_VERSION>`
- Scope classification: `release-blocker-current-branch` / `follow-up-after-merge` / `revalidate-on-main-or-develop` / `planning-only`
- Scope note: This draft was generated from the branch above. Revalidate against the target branch before creating or closing public GitHub issues if the branch has moved or has not yet merged.
```

Private notes should also identify the branch scope, but must not include raw tokens, secrets, exploit payloads, private deployment details, or user safety data.

## Naming rules

Issue drafts:

```text
NNN-short-kebab-title.md
```

Rules:

- three-digit numeric prefix
- kebab-case title
- `.md` extension
- no spaces
- no root-level issue draft files

Index file:

```text
README.md
```

Issue creation review:

```text
create-issues-review.md
```

Private notes:

```text
private-notes/NNN-short-kebab-title.md
```

Private notes must never be used for public issue creation.

## Commit policy

Recommended default for public repositories:

- `.backlog-drafts/` should normally be ignored and not committed.
- Actual backlog source of truth should be GitHub Issues.
- Historical drafts can be moved into a clearly marked archive only if useful.

If `.backlog-drafts/` is already committed, recommend one of:

1. remove from repository and add `.backlog-drafts/` to `.gitignore`
2. move to `codex/archive/YYYY-MM-DD-initial-backlog-drafts/` with a README explaining it is historical
3. keep only if there is a clear reason and no stale/duplicate issue drafts

## Audit checks

Check:

- whether `.backlog-drafts/` exists
- whether it uses flat root-level drafts
- whether it uses timestamped directories
- whether new drafts are branch-scoped under `<branch-slug>/`
- whether public drafts include `## Priority`
- whether public drafts include `## Type`
- whether public drafts include `## Labels`
- whether public drafts include `## Branch scope`
- whether labels are backtick-wrapped Markdown bullets
- whether public drafts include the `backlog` label
- whether public drafts include at least one additional topic/type label
- whether draft labels exist in the repository when GitHub CLI is available
- whether branch-scoped drafts identify current branch and current HEAD
- whether branch-scoped drafts identify reviewed branch/ref and reviewed commit SHA when produced from report validation
- whether `create-issues-review.md` is in the selected branch-scoped scan directory
- whether `private-notes/` exists and is excluded from issue creation
- whether `.backlog-drafts/` is ignored in `.gitignore`
- whether issue drafts duplicate already-created GitHub issues
- whether any issue creation script points to stale flat paths
- whether `80-backlog-scan-issue-drafts.md` enforces branch-scoped output and required metadata
- whether `85-create-github-issues-from-drafts.md` selects a branch-scoped draft directory, preserves branch scope in issue bodies, and passes labels to `gh issue create`
- whether `95-validate-technical-review-report.md` creates branch-scoped report issue drafts with priority and labels
- whether any draft contains raw tokens, secrets, private deployment info, exploit details, or user safety data

## Permitted changes

Only make changes if the maintainer asked for implementation, not just review.

Allowed when implementation is requested:

- update `codex/prompts/80-backlog-scan-issue-drafts.md`
- update `codex/prompts/85-create-github-issues-from-drafts.md`
- update `codex/prompts/95-validate-technical-review-report.md`
- update `codex/README.md`
- update `docs/codex-change-control.md`
- update `.gitignore`
- move `.backlog-drafts/` into a historical archive
- remove stale issue creation scripts only if explicitly requested

Not allowed:

- application code changes
- creating issues
- closing issues
- executing scripts
- deleting drafts without explicit instruction
- creating labels unless explicitly requested

## Public repository recommendation

Before making the repo public or before relying on generated drafts:

```text
.gitignore includes .backlog-drafts/
new generated drafts are branch-scoped
new generated drafts include Priority, Type, Labels, and Branch scope
issue creation scripts fail closed for missing metadata
committed .backlog-drafts/ is removed or archived
scripts/create-backlog-issues.sh is removed, archived, or updated to branch-scoped draft paths
```

## Validation

After changes, if any:

```bash
git diff --stat
git diff -- .gitignore .backlog-drafts codex docs scripts
git diff --check
```

Validate draft metadata:

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

If only Markdown and `.gitignore` changed, Go tests are not required.

If application code changed, stop and explain why.

## Output

Summarize:

1. current `.backlog-drafts/` structure
2. whether it matches the branch-scoped standard
3. stale/duplicate drafts found
4. drafts missing priority/type/labels/branch scope
5. missing or non-existent labels found
6. script/path mismatches found
7. `.gitignore` recommendation
8. prompt workflow changes recommended
9. public-repo cleanup recommendation
10. any files changed, if implementation was requested
