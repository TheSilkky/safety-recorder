# Codex Prompt: Prepare GitHub Issue Creation Commands from Reviewed Branch-Scoped Backlog Drafts

Read reviewed backlog drafts and generate a script that can create GitHub issues with GitHub CLI.

Do **not** modify application code.
Do **not** create GitHub issues directly unless explicitly told to run commands.
Do **not** execute the generated script.
Do **not** include sensitive vulnerability details in public issues.
Do **not** modify workflows, Dockerfiles, SQL migrations, Go code, generated files, binaries, database files, or uploaded blob data.

## Goal

Generate a shell script that creates GitHub issues from maintainer-reviewed backlog draft files, preserving priority in the issue body and applying GitHub labels from each draft.

Create:

```text
scripts/create-backlog-issues.sh
```

## Repository

```text
open-proofline/server
```

## Draft directory

Backlog drafts are normally created by `80-backlog-scan-issue-drafts.md` or the
independent Phase 2 `95-validate-technical-review-report.md` workflow.

They should live under a branch-scoped timestamped directory such as:

```text
.backlog-drafts/YYYY-MM-DD/<branch-slug>/
```

or:

```text
.backlog-drafts/current/<branch-slug>/
```

Before generating the script:

1. Prefer an explicitly provided draft directory, if the maintainer gives one.
2. Otherwise, inspect `.backlog-drafts/` and choose the newest timestamped branch-scoped directory.
3. If multiple directories exist and the newest or intended branch scope is unclear, stop and ask which draft directory to use.
4. Do not use `.backlog-drafts/private-notes/` or any `private-notes/` directory for public issue creation.
5. Read the selected directory `README.md` and every `NNN-*.md` draft before generating a script.
6. If an existing `scripts/create-backlog-issues.sh` points at a different or missing draft directory, replace it only after selecting and validating the intended branch-scoped directory.

## Required draft metadata

Verify every included public draft contains:

```md
## Priority
## Type
## Labels
## Branch scope
```

If any required section is missing, exclude the draft and list it in the output summary.

The `## Labels` section must contain backtick-wrapped GitHub label names as bullet items, for example:

```md
## Labels

Suggested GitHub labels:

- `backlog`
- `security`
- `ci`
```

Every public draft must include:

```md
- `backlog`
```

If a draft is missing `backlog`, exclude it unless the maintainer explicitly approves creating the issue without the backlog label.

## Label handling

The generated script must pass labels to `gh issue create`.

Use one `--label` argument per label.

Example:

```bash
gh issue create \
  --repo "$REPO" \
  --title "$title" \
  --body-file "$draft" \
  --label "backlog" \
  --label "security" \
  --label "ci"
```

Before generating the script, list repository labels if GitHub CLI is available:

```bash
gh label list --repo open-proofline/server --limit 200
```

If a draft contains a label that does not exist in the repository, exclude that draft and list the missing label in the output summary.

Do not create labels unless the maintainer explicitly asks.

Do not silently create unlabeled issues.

Do not silently drop labels.

## Priority handling

Priority remains in the issue body under `## Priority`.

Do not convert priority to a GitHub label unless the repository has matching priority labels and the maintainer explicitly asks to use them.

If priority is missing, exclude the draft and list it in the output summary.

## Branch scope requirements

Verify every included draft contains:

```md
## Branch scope
```

If a draft is missing branch scope, exclude it and list it in the output summary.

If a draft was generated from a release-prep or feature branch, preserve the `Branch scope` section in the GitHub issue body unless the maintainer explicitly requests removal.

If a draft says it must be revalidated on `main`, `develop`, or another target branch before public issue creation, exclude it unless the maintainer explicitly confirms revalidation is complete.

## Requirements

- Use `gh issue create`.
- Derive title from the first Markdown heading.
- Use the entire draft body, including `## Priority`, `## Labels`, and `## Branch scope`.
- Use labels from the `## Labels` section.
- Do not execute the script.
- Quote file paths and arguments safely.
- Exclude drafts marked sensitive/private or not for public creation.
- Exclude drafts missing priority, type, labels, or branch scope.
- Exclude drafts missing the `backlog` label.
- Exclude drafts with non-existent labels unless the maintainer explicitly approves a label creation step.
- Warn that running the script twice may create duplicate issues.

## Expected script behaviour

The script should:

1. Stop on errors.

   ```bash
   set -euo pipefail
   ```

2. Define the repository once.

   ```bash
   REPO="open-proofline/server"
   ```

3. Define the selected branch-scoped draft directory.

4. Refuse to run if the selected draft directory is missing.

5. Refuse to run if GitHub CLI is not installed or not authenticated.

6. Print which issue draft is being created.

7. Use `--body-file`.

8. Include `--label` arguments derived from each draft.

9. Print a warning before creating issues that branch-scoped drafts should have been reviewed for current target-branch relevance.

## Review exclusions

Before generating the script, inspect every draft in the selected draft directory.

Exclude any draft that:

- is marked sensitive
- says not to create publicly
- lives under `private-notes/`
- is missing `## Priority`
- is missing `## Type`
- is missing `## Labels`
- is missing `## Branch scope`
- is missing the `backlog` label
- references a label that does not exist in the repository
- says `sensitive-do-not-publicly-file`
- says `revalidate-on-main-or-develop` without a maintainer note that revalidation is complete
- contains raw tokens
- contains secrets
- contains private deployment details
- contains user safety data
- contains exploit details that should go through `SECURITY.md`

If a draft is excluded, list it in the output summary and explain why.

## Generated script requirements

The script should call `gh issue create` like:

```bash
gh issue create \
  --repo "$REPO" \
  --title "$title" \
  --body-file "$draft" \
  --label "backlog" \
  --label "<other-label>"
```

Use safe quoting for file paths, titles, and labels.

## Issue creation review output

Also create:

```text
.backlog-drafts/<selected-directory>/create-issues-review.md
```

This file should summarize:

- selected draft directory
- branch scope for that directory
- issue drafts included in the script
- issue drafts excluded
- priority values found
- labels used
- missing/non-existent labels found
- command to run the script
- warning that running twice may create duplicates
- warning that branch-scoped drafts should be revalidated if the source branch moved
- warning that private notes are excluded from public issue creation

## Constraints

Allowed files:

- `scripts/create-backlog-issues.sh`
- `.backlog-drafts/<selected-directory>/create-issues-review.md`

Do not change anything else unless needed only for a tiny documentation link.

## Validation

After generating the script:

```bash
git diff --stat
git diff -- scripts/create-backlog-issues.sh .backlog-drafts
git diff --check
```

Do not run Go tests unless code was changed.

Do not execute the script.

## Output

Summarize:

1. selected draft directory
2. selected draft branch scope
3. issue drafts included
4. issue drafts excluded
5. priorities found
6. labels used
7. labels missing from repository, if any
8. script path
9. command to run after maintainer review
10. branch-scope/revalidation warnings
11. reminder that running twice may create duplicate issues
