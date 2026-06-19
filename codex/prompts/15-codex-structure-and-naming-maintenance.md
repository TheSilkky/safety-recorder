# Codex Prompt: Codex Structure and Naming Maintenance

Review and standardize the `codex/` directory structure and file naming conventions.

This is a Markdown/process maintenance task.

Do **not** change application code.
Do **not** change backend behaviour.
Do **not** change docs outside `codex/` and `AGENTS.md` unless explicitly needed to keep prompt workflow references consistent.
Do **not** delete historical prompts unless explicitly requested.
Do **not** run or create GitHub issues.
Do **not** add dependencies.

## Goal

Make `codex/` consistent, reusable, and easy to maintain.

Enforce a clear distinction between:

- reusable prompts
- historical work-order prompts
- archived one-off prompts
- generated local artifacts that should not be committed

## Source of truth

Read:

- `README.md`
- `AGENTS.md`
- `codex/README.md`
- all files under `codex/`
- `docs/codex-change-control.md`, if present
- `docs/development.md`, if relevant
- `open-proofline/website/docs/governance-and-political-alignment.md`, if
  prompt workflow wording touches public governance or public-good framing
- `open-proofline/website/docs/repository-readme-baseline.md`, if prompt
  workflow wording touches README structure, public voice, or source-of-truth
  mapping

## Standard directory structure

Use this structure:

```text
codex/
  README.md
  prompts/
  archive/
  work-orders/
```

Optional, only if the repository already uses it or the maintainer requests it:

```text
codex/reviews/
```

Do not create extra prompt directories without a clear reason.

## Reusable prompt naming

Reusable prompts belong in:

```text
codex/prompts/
```

Filename pattern:

```text
NN-short-kebab-title.md
```

Rules:

- two-digit numeric prefix
- kebab-case title
- `.md` extension
- no spaces
- no date prefix
- one reusable workflow per file

Examples:

```text
00-project-context-check.md
15-codex-structure-and-naming-maintenance.md
80-backlog-scan-issue-drafts.md
```

## Historical prompt naming

Historical prompts belong in:

```text
codex/archive/
codex/work-orders/
```

Filename pattern:

```text
YYYY-MM-DD-short-kebab-title.md
```

Rules:

- date prefix
- kebab-case title
- `.md` extension
- no numeric workflow prefix
- historical prompts must be marked as historical/reference-only

## What belongs where

### `codex/prompts/`

Reusable prompts that can be safely run again against the current repository.

### `codex/work-orders/`

Historical multi-step work-order prompts.

### `codex/archive/`

Old prompts retained for reference but not part of current workflow.

## Audit checks

Check for:

- reusable prompts missing numeric prefixes
- one-off prompts incorrectly placed in `codex/prompts/`
- historical prompts missing date prefixes
- spaces, uppercase words, or inconsistent filenames
- prompt files that reference stale project state
- prompt files that miss the website source documents when their workflow
  covers README structure, public voice, governance posture, or source-of-truth
  mapping
- prompt files that contradict `AGENTS.md`
- prompt files that still say server-side key storage/decryption is permanently impossible
- prompt files that do not distinguish current implementation from future key custody design
- missing prompt entries in `codex/README.md`
- `codex/README.md` order not matching actual prompt files
- generated downloads, ZIP files, or local artifacts accidentally committed under `codex/`

## Permitted changes

Allowed:

- rename prompt files to match convention
- move prompt files between `prompts/`, `archive/`, and `work-orders/`
- update `codex/README.md`
- add short README files inside historical directories if useful
- update `AGENTS.md` only if needed to align prompt workflow rules

Not allowed:

- application code changes
- SQL changes
- Docker changes
- CI changes
- deleting historical prompts without explicit instruction
- changing project behaviour

## Recommended `codex/README.md` contents

Ensure `codex/README.md` documents:

- reusable prompt order
- allowed directory structure
- naming conventions
- historical prompt rules
- validation expectations
- key custody guardrail wording
- backlog draft workflow
- issue/PR workflow

## Validation

After changes:

```bash
git diff --stat
git diff -- AGENTS.md codex
git diff --check
```

If any files outside `AGENTS.md` or `codex/` changed, stop and explain why.

Go tests are not required unless code changed.

## Output

Summarize:

1. files renamed
2. files moved
3. files updated
4. prompt order changes
5. stale prompts found
6. historical prompts left untouched
7. whether any non-Markdown or application files changed
