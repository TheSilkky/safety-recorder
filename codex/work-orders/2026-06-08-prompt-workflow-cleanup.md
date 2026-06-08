# Historical Work Order: Prompt Workflow Cleanup

This is a one-off Codex work order for `open-proofline/web-client`.

Date: 2026-06-08
Branch: `codex/prompt-workflow-work-orders-2026-06-08`
Base branch: `develop`
Base commit: `2434b39ccec62cf6a842b0aa9d3bb9219e7d89cb`

## Goal

Update the reusable Codex prompt workflow so prompt naming conventions, issue/PR workflow guidance, source-of-truth requirements, and validation instructions match the current web-client repository state.

This is a prompt/documentation maintenance task only. Do not change React application behavior, routes, API client behavior, authentication behavior, styling, tests, build configuration, or backend assumptions unless the maintainer explicitly expands scope.

## Reasoning

Use very-high reasoning.

This work touches reusable prompt workflow behavior, frontend validation expectations, backend source-of-truth boundaries, and review/fix-mode wording. Keep the changes small, mechanical, and reviewable.

## Source of truth to inspect

Before editing, inspect:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `CHANGELOG.md`, if present
- `docs/README.md`, if present
- relevant docs under `docs/`
- `package.json`
- `package-lock.json`
- `.github/workflows/`
- `codex/README.md`
- every reusable prompt under `codex/prompts/`
- `../server/README.md` and `../server/SECURITY.md` as read-only backend source inputs, if available locally

Do not rely on stale assumptions from this work order if current repository files disagree.

## Scoped changes

### 1. Add naming and workflow conventions to `codex/README.md`

Update `codex/README.md` so it documents the reusable prompt naming convention rather than only listing existing prompts.

Add or align wording for:

- reusable prompts live in `codex/prompts/`;
- reusable prompts use `NN-short-kebab-title.md`;
- use two-digit numeric prefixes;
- use kebab-case;
- use `.md` extension;
- use one reusable workflow per file;
- historical/one-off prompts should not be placed under `codex/prompts/` unless they are intended to be reusable;
- generated backlog or issue-review drafts belong outside `codex/`.

Also document the normal issue/PR workflow:

```text
00-project-context-check.md
05-codex-change-control.md
70-work-on-github-issue.md
75-create-draft-pr-from-current-branch.md
76-request-codex-pr-review.md
```

Clarify that review prompts such as `20-code-review.md` and `30-security-review.md` are review/fix-mode prompts, not ordinary preflight prompts.

### 2. Align validation instructions

Review validation instructions in `codex/README.md` and reusable prompts.

The repo standard validation should be internally consistent. Use the current `package.json` scripts and existing CI reality as source of truth.

At minimum, review and update as needed:

- `codex/README.md`
- `codex/prompts/20-code-review.md`
- `codex/prompts/30-security-review.md`
- `codex/prompts/70-work-on-github-issue.md`
- `codex/prompts/75-create-draft-pr-from-current-branch.md`
- any other reusable prompt with stale validation commands

Decide and document whether `npm run test:e2e` is required for every full validation run or only when route/browser flows change. Avoid contradictory wording.

Ensure relevant implementation/PR prompts include `git diff --check`.

### 3. Add minimal source-of-truth blocks to lightweight prompts

Review lightweight prompts that currently start directly with focus/constraints and lack a source-of-truth section.

At minimum, inspect:

- `codex/prompts/10-frontend-readability-maintenance.md`
- `codex/prompts/20-code-review.md`
- `codex/prompts/30-security-review.md`
- `codex/prompts/50-web-security-header-review.md`

Add a short source-of-truth section where useful:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- relevant docs
- relevant `src/` and tests
- `open-proofline/server` docs for backend behavior and route assumptions

Keep these additions concise. Do not bloat lightweight prompts into the comprehensive review prompt.

### 4. Clarify review-only versus fix mode

Review prompts titled as reviews, especially:

- `codex/prompts/20-code-review.md`
- `codex/prompts/30-security-review.md`
- `codex/prompts/50-web-security-header-review.md`

Add clear mode wording if needed:

- default mode is review-only unless the maintainer requests fixes or the prompt explicitly enters fix mode;
- if fixes are made, keep them minimal and scoped;
- do not add unrelated features during a review pass;
- do not include sensitive vulnerability details in public artifacts.

### 5. Preserve web-client boundaries

Keep existing web-client boundaries intact:

- frontend-only repository;
- server remains source of truth for backend behavior;
- no backend features;
- no recording/capture;
- no browser decryption or key unwrapping unless explicitly scoped and threat-modeled;
- no key escrow;
- no playable export;
- no emergency dispatch;
- no OAuth/JWT;
- no public admin dashboards;
- preserve Tailwind Catalyst/Tailwind Plus licensing boundaries.

## Explicit non-goals

Do not:

- change React application code;
- change route behavior;
- change API client behavior;
- change auth/session behavior;
- change browser storage behavior;
- add recording/capture;
- add browser decryption;
- add key unwrapping;
- add trusted-contact decryption;
- add backend decryption;
- add key escrow;
- add notification delivery;
- add paid registration, billing, Stripe, or payment-provider behavior;
- change package dependencies;
- create GitHub issues;
- create or merge pull requests;
- edit sibling repositories.

## Sensitive-data rules

Do not include raw tokens, session cookies, CSRF tokens, Authorization headers, request bodies, uploaded bytes, plaintext, raw keys, raw media keys, contact private keys, unwrapped secrets, wrapped-key ciphertext, exploit payloads, object-store credentials, stored paths, object keys, private deployment details, or user safety data in public docs, prompts, comments, tests, or summaries.

## Validation

For prompt/docs-only changes, run:

```bash
git diff --check
git diff --stat
git diff -- README.md AGENTS.md SECURITY.md CHANGELOG.md docs codex package.json .github
```

If formatting checks are available and relevant for Markdown, run the documented command, for example:

```bash
npm run format:check
```

If application code changes unexpectedly, stop and explain why. Do not run full frontend validation unless code/config changed or the maintainer explicitly requests it.

## Output

Summarize:

1. files changed;
2. naming/workflow conventions added;
3. validation wording updated;
4. source-of-truth sections added;
5. review-only/fix-mode wording updated;
6. prompts intentionally not changed;
7. validation commands run and results;
8. confirmation that no React app code, route behavior, API behavior, auth/session behavior, issues, PRs, or sibling repositories were changed.
