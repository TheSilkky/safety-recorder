# Codex Prompt: Review Open Issues for Stale or Fixed Work

Review open GitHub issues and identify issues that may be fixed, stale, duplicate, superseded, or still valid.

Do **not** change application code.
Do **not** change documentation unless explicitly requested.
Do **not** create new issues unless explicitly requested.
Do **not** close issues unless explicitly requested.
Do **not** merge pull requests.
Do **not** make repository changes as part of the initial review.

## Goal

Review open issues against the current repository state and current branch.

When an issue appears fixed, identify whether it is fixed on the current branch, fixed on the target branch, already merged, or still needs verification after merge.

## Branch scope

Before reviewing issues, record:

```bash
git status --short --branch --untracked-files=all
git branch --show-current
git rev-parse HEAD
```

Use the current checked-out branch as the review scope.

If reviewing a release-prep branch, recommendations must distinguish:

- fixed on current branch
- fixed on target branch
- still needs verification after merge
- safe to close now

## Output directory

Create branch-scoped review drafts under:

```text
.issue-review-drafts/YYYY-MM-DD/<branch-slug>/
```

If the date is unavailable, use:

```text
.issue-review-drafts/current/<branch-slug>/
```

Each issue review file must include a `## Branch scope` section with current branch, current HEAD, target branch if known, and a revalidation note.

## Recommendation statuses

Use one of:

- `keep-open`
- `fixed-current-branch-not-merged`
- `close-fixed`
- `close-duplicate`
- `close-superseded`
- `needs-update`
- `needs-human-review`
- `sensitive-do-not-publicly-discuss`

Do not recommend `close-fixed` if evidence exists only on an unmerged release-prep or feature branch unless the maintainer explicitly asked to close based on that branch.

## Validation

After creating drafts:

```bash
git diff --stat
git diff -- .issue-review-drafts
git diff --check
```

Do not run Go tests unless code changed.
