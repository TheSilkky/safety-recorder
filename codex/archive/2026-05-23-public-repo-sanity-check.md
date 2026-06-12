# Codex Prompt: Pre-Public Repository Sanity Check

Run sanity checks before making the repository public.

The purpose is low-visibility public access to avoid private-repository limits, not advertising.

Do **not** modify the repository.
Do **not** make the repository public.
Do **not** create, close, or edit GitHub issues.
Do **not** create or merge pull requests.
Do **not** run destructive commands.

## Goal

Review whether the repository is safe and tidy enough to make public.

Produce a report with blocking issues, recommended cleanup, and GitHub About metadata suggestions.

## Repository

```text
TheSilkky/safety-recorder
```

## Required checks

### Repository metadata

Inspect:

- repository visibility
- default branch
- open PRs
- open issues
- GitHub About description, topics, and homepage if available

Suggest:

- description
- topics
- homepage setting
- whether to show packages/releases

### Public safety and security docs

Review:

- `README.md`
- `SECURITY.md`
- `LICENSE`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/deployment.md`

Check:

- not-production-ready warnings are visible
- `/v1` exposure warning is clear
- vulnerability reporting path is usable before public visibility
- no public issue path for private vulnerabilities
- license is present
- security contact/private vulnerability reporting is configured or documented

### Secret and artifact hygiene

Inspect for committed:

- `.env`
- simulator key files
- local databases
- SQLite WAL/SHM files
- uploaded blob data
- generated binaries
- downloaded ZIP bundles
- logs
- raw tokens
- private deployment details

Also check `.gitignore`.

### `codex/` structure

Review:

- directory layout
- naming conventions
- reusable prompt order
- historical prompt separation
- stale prompt wording
- key custody guardrail consistency

Recommend whether to run:

```text
codex/prompts/15-codex-structure-and-naming-maintenance.md
```

### `.backlog-drafts/` hygiene

Review:

- whether `.backlog-drafts/` is committed
- whether it is flat or timestamped
- whether drafts duplicate current issues
- whether private notes exist
- whether scripts point at stale draft paths

Recommend whether to remove, ignore, or archive.

### Scripts and generated files

Review:

- `scripts/`
- issue creation scripts
- release artifacts
- Docker/GHCR references

Check whether any script is one-time/local and should not be public.

### Issues and PRs

Review open issues and PRs.

Identify:

- stale/fixed issues
- sensitive issues that should not be public
- public issue wording that reveals too much
- Dependabot PRs or maintenance PRs that should be handled before public switch

Recommend whether to run:

```text
codex/prompts/82-review-open-issues-for-stale-or-fixed.md
```

### Branch/release hygiene

Check:

- CI exists
- branch protection should be configured
- tags/releases do not imply production readiness
- changelog is honest
- package visibility/metadata is acceptable

## Output

Create a Markdown report under:

```text
.public-sanity-review/YYYY-MM-DD/README.md
```

If date is unavailable, use:

```text
.public-sanity-review/current/README.md
```

Do not change any other files.

## Report format

Use:

```md
# Public Repository Sanity Review

## Verdict

Ready / Ready after cleanup / Not ready

## Blocking before public

## Strongly recommended before public

## Nice to have

## GitHub About recommendations

## `codex/` structure findings

## `.backlog-drafts/` findings

## Secrets/artifacts findings

## Issue/PR findings

## Security policy findings

## Suggested cleanup order

## Commands for maintainer to run manually

## Notes
```

## Validation

After creating the report:

```bash
git diff --stat
git diff -- .public-sanity-review
```

If any files outside `.public-sanity-review/` changed, stop and explain why.

Do not run Go tests unless code changed.
