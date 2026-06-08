# One-off Codex Work Order: Document Develop And Release Branch Rulesets

Historical/reference-only work order.

Use this prompt as context before running `codex/prompts/70-work-on-github-issue.md` or equivalent scoped documentation work.

## Goal

Update repository documentation to reflect the actual GitHub repository rulesets for:

- `main`
- `develop`
- `release/v*`

Use the attached exported ruleset JSON files as the source of truth for exact names, targets, enforcement status, required checks, pull request requirements, and bypass mode.

This is a documentation/process update only.

## Repository

```text
TheSilkky/safety-recorder
```

## Attached ruleset exports

The maintainer supplied these JSON exports:

```text
Protect main.json
Protect develop.json
Protect release_v_.json
```

Read them directly before editing:

```bash
python3 - <<'PY'
from pathlib import Path
import json

for path in [
    Path("Protect main.json"),
    Path("Protect develop.json"),
    Path("Protect release_v_.json"),
]:
    print(f"--- {path}")
    data = json.loads(path.read_text(encoding="utf-8"))
    print("name:", data["name"])
    print("target:", data["target"])
    print("enforcement:", data["enforcement"])
    print("include:", data["conditions"]["ref_name"]["include"])
    print("rules:", [rule["type"] for rule in data["rules"]])
    for rule in data["rules"]:
        if rule["type"] == "pull_request":
            print("pull_request:", rule["parameters"])
        if rule["type"] == "required_status_checks":
            print("required_status_checks:", rule["parameters"])
    print("bypass_actors:", data.get("bypass_actors", []))
PY
```

If the file names differ locally, locate the exported JSON files and inspect those instead.

Do not rely on remembered branch policy. The JSON exports override older docs and assumptions.

## Ruleset facts to document

### `Protect main`

Exported facts:

```text
name: Protect main
target: branch
enforcement: active
conditions.ref_name.include:
  - ~DEFAULT_BRANCH
```

Rules:

```text
deletion
non_fast_forward
pull_request
required_status_checks
```

Pull request parameters:

```text
required_approving_review_count: 1
dismiss_stale_reviews_on_push: true
required_reviewers: []
require_code_owner_review: false
require_last_push_approval: false
required_review_thread_resolution: false
allowed_merge_methods:
  - merge
  - squash
  - rebase
```

Required status checks:

```text
strict_required_status_checks_policy: true
do_not_enforce_on_create: false
required_status_checks:
  - Go tests
  - Build Linux binary
  - Build Docker image
```

Bypass:

```text
actor_type: RepositoryRole
actor_id: 5
bypass_mode: pull_request
```

### `Protect develop`

Exported facts:

```text
name: Protect develop
target: branch
enforcement: active
conditions.ref_name.include:
  - refs/heads/develop
```

Rules:

```text
deletion
non_fast_forward
pull_request
required_status_checks
```

Pull request parameters:

```text
required_approving_review_count: 1
dismiss_stale_reviews_on_push: true
required_reviewers: []
require_code_owner_review: false
require_last_push_approval: false
required_review_thread_resolution: true
allowed_merge_methods:
  - merge
  - squash
  - rebase
```

Required status checks:

```text
strict_required_status_checks_policy: true
do_not_enforce_on_create: false
required_status_checks:
  - Go tests
  - Build Docker image
  - Build Linux binary
```

Document the checks in a stable logical order if desired:

```text
Go tests
Build Linux binary
Build Docker image
```

but do not imply the JSON order is semantically important.

Bypass:

```text
actor_type: RepositoryRole
actor_id: 5
bypass_mode: pull_request
```

### `Protect release/v*`

Exported facts:

```text
name: Protect release/v*
target: branch
enforcement: active
conditions.ref_name.include:
  - refs/heads/release/v*
```

Rules:

```text
deletion
non_fast_forward
required_status_checks
pull_request
```

Pull request parameters:

```text
required_approving_review_count: 1
dismiss_stale_reviews_on_push: true
required_reviewers: []
require_code_owner_review: false
require_last_push_approval: false
required_review_thread_resolution: true
allowed_merge_methods:
  - merge
  - squash
  - rebase
```

Required status checks:

```text
strict_required_status_checks_policy: true
do_not_enforce_on_create: false
required_status_checks:
  - Go tests
  - Build Linux binary
  - Build Docker image
```

Bypass:

```text
actor_type: RepositoryRole
actor_id: 5
bypass_mode: pull_request
```

## Documentation changes to make

### Primary file

Update:

```text
docs/development.md
```

Replace the existing single-ruleset section with a multi-ruleset section.

The section should accurately document:

- the repository uses GitHub repository rulesets, not classic branch protection
- `Protect main` targets `~DEFAULT_BRANCH`, currently `main`
- `Protect develop` targets `refs/heads/develop`
- `Protect release/v*` targets `refs/heads/release/v*`
- all three rulesets are active
- all three block branch deletion
- all three block non-fast-forward updates
- all three require pull requests before merge
- all three require one approving review
- all three dismiss stale approvals after new pushes
- `Protect main` does **not** require review thread resolution
- `Protect develop` and `Protect release/v*` do require review thread resolution
- all three allow merge, squash, and rebase merge methods
- all three require strict required status checks:
  - `Go tests`
  - `Build Linux binary`
  - `Build Docker image`
- all three use pull-request-only bypass for the exported repository role bypass actor
- tag-only jobs must not be required as PR checks

Add or update a branch model section:

```text
main = stable release line
develop = next-release integration branch
release/v* = short-lived release-prep branches
```

Recommended branch flow:

```text
issue branches -> develop
develop -> release/vX.Y.Z-prep
release/vX.Y.Z-prep -> main
tag vX.Y.Z from main
main -> develop sync after release
```

Include PR base guidance:

```text
normal next-release issue work -> base develop
release-prep fixes -> base release/vX.Y.Z-prep
final release PR -> base main
hotfixes -> base main, then sync main back to develop
```

### Secondary files to review

Review and update if stale:

```text
codex/README.md
codex/prompts/75-create-draft-pr-from-current-branch.md
codex/prompts/76-request-codex-pr-review.md
codex/prompts/90-release-check.md
```

Do not update reusable prompts unless their current committed text still assumes `main` as the only PR base branch.

If prompts are already branch-aware, avoid churn and mention that no prompt update was needed.

## Suggested `docs/development.md` wording

Use wording like this, adjusted to fit the surrounding document:

```md
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
limited to pull requests. Use bypass only for maintainer-authored changes when no
independent write-access reviewer is available, after required checks pass and
the maintainer has reviewed the diff.

Do not require tag-only jobs such as `Attest Linux binary`, `Upload release
binary`, or `Publish Docker image` as pull request status checks. Those jobs run
only for trusted release/tag contexts and would make normal PRs unmergeable if
required on PRs.
```

Then add:

```md
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
- Release-candidate tags may be created from release-prep branches when validating release automation.

When creating PRs, set the intended base branch explicitly:

- issue work for the next release: base `develop`
- release-prep fixes: base `release/vX.Y.Z-prep`
- final release PR: base `main`
- hotfixes: base `main`, then sync back to `develop`
```

If this creates duplicate headings or awkward placement, integrate the branch model into the existing branch protection section instead.

## Constraints

Do **not**:

- change application code
- change GitHub Actions workflow behavior unless a stale docs claim forces a tiny doc-only correction
- change actual repository rulesets
- create or close issues
- change release assets
- change Docker image publishing
- change public/private route guidance
- change key custody, decryption, OAuth/JWT, user accounts, SMS, push notifications, or public admin behavior
- claim production readiness
- document planned rulesets as active unless the JSON export shows they are active

## Validation

For this docs/process change:

```bash
git diff --stat
git diff -- docs/development.md codex/README.md codex/prompts
```

No Go tests are required if only Markdown changes.

Check for stale wording:

```bash
grep -Rni "Protect main\|develop\|release/v\*\|--base main\|base main" docs codex/prompts codex/README.md
```

Manual checks:

- `docs/development.md` names all three rulesets.
- `Protect main` still targets `~DEFAULT_BRANCH`, currently `main`.
- `Protect develop` target is `refs/heads/develop`.
- `Protect release/v*` target is `refs/heads/release/v*`.
- Required checks are exactly:
  - `Go tests`
  - `Build Linux binary`
  - `Build Docker image`
- Docs explicitly say tag-only jobs must not be required PR checks.
- Docs describe `main`, `develop`, and `release/v*` branch purposes.
- Docs say PR base branches must be explicit.

## Prompt sequence

Recommended prompt flow:

```text
00-project-context-check.md
05-codex-change-control.md
70-work-on-github-issue.md
40-documentation-update.md
75-create-draft-pr-from-current-branch.md
76-request-codex-pr-review.md
```

When using `75-create-draft-pr-from-current-branch.md`, set the target base branch explicitly.

If this work is done after `develop` exists, the normal target base should likely be:

```text
develop
```

If this is still part of final `v0.5.0` release cleanup, target the appropriate release-prep branch or `main` according to the current workflow.

## Codex output requirements

Return:

1. files changed
2. summary of ruleset docs added
3. exact rulesets documented
4. target branch patterns documented
5. required checks documented
6. prompt files reviewed and whether any were updated
7. validation commands run
8. confirmation that no code/workflow/ruleset behavior changed
9. any manual follow-up required
