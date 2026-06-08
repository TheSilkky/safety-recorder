# Historical Work Order: Prompt Workflow Cleanup

This is a one-off Codex work order for `open-proofline/server`.

Date: 2026-06-08
Branch: `codex/prompt-workflow-work-orders-2026-06-08`
Base branch: `develop`
Base commit: `6417c8e345d9a40ca01bfd131d9c1c2f99ae5a5f`

## Goal

Update the reusable Codex prompt workflow so prompt names, workflow ordering, validation instructions, and key-custody design guardrails match the current server repository state.

This is a prompt/documentation maintenance task only. Do not change runtime Go code, migrations, deployment behavior, API behavior, storage behavior, encryption behavior, or tests unless the maintainer explicitly expands scope.

## Reasoning

Use very-high reasoning.

This work touches reusable prompt workflow behavior, key-custody wording, review/fix-mode boundaries, validation instructions, and security review coverage expectations. Keep the changes small, mechanical, and reviewable.

## Source of truth to inspect

Before editing, inspect:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `CHANGELOG.md`
- `docs/README.md`
- `docs/development.md`
- `docs/codex-change-control.md`, if present
- `docs/key-custody.md`, if present
- `docs/contacts-and-viewer-replacement.md`, if present
- `docs/post-quantum-envelope.md`, if present
- `docs/browser-decryption.md`, if present
- `docs/break-glass-key-access.md`, if present
- `docs/security-model.md`
- `docs/threat-model.md`
- `internal/httpapi/routes.go`
- `internal/httpapi/main_rate_limit.go`
- `internal/httpapi/main_rate_limit_test.go`
- `codex/README.md`
- every reusable prompt under `codex/prompts/`

Do not rely on stale assumptions from this work order if current repository files disagree.

## Scoped changes

### 1. Align validation instructions

Review `codex/README.md` and reusable prompts for validation drift.

Ensure Go-code workflows consistently include:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

Docs-only workflows may continue to use docs-safe validation, but should include `git diff --check` where appropriate.

At minimum, review and update as needed:

- `codex/README.md`
- `codex/prompts/20-code-review.md`
- `codex/prompts/30-security-review.md`
- `codex/prompts/70-work-on-github-issue.md`
- `codex/prompts/75-create-draft-pr-from-current-branch.md`
- any other reusable prompt with stale validation commands

### 2. Clarify review-only versus fix mode

Review prompts that are titled as reviews but may edit files.

At minimum, check:

- `codex/prompts/20-code-review.md`
- `codex/prompts/30-security-review.md`

Add clear mode wording if needed:

- default mode is review-only unless the maintainer requests fixes, or the prompt explicitly enters fix mode;
- if fixes are made, keep them minimal and scoped;
- do not add unrelated features during a review pass;
- do not include sensitive vulnerability details in public artifacts.

### 3. Update key-custody design prompts for existing docs

Review:

- `codex/prompts/35-key-custody-and-emergency-access-design.md`
- `codex/prompts/37-browser-decryption-design-spike.md`
- `codex/prompts/38-break-glass-and-dead-mans-switch-key-access-design.md`

Update wording so these prompts create or update the relevant design document instead of blindly creating a duplicate fixed-path document when the document already exists.

Use wording like:

```text
Create or update the relevant design document. If the document already exists, update it rather than creating a duplicate.
```

### 4. Align key-custody prompt with durable recipient-key plus CEK model

Update `35-key-custody-and-emergency-access-design.md` so it reflects the current accepted direction:

- private keys belong to accounts, devices, and trusted contacts;
- content-encryption keys belong to incidents, streams, or bounded chunk groups;
- wrapped access records connect recipient key versions to content keys;
- current runtime behavior remains ciphertext-only unless separately implemented;
- future key custody remains explicit, documented, reviewed, and threat-modeled;
- server storage of wrapped/encrypted keys may be acceptable when explicitly designed;
- raw server-side key access or server-side decryption remains a deliberate break-glass/dead-man-switch/emergency-access design path only.

Do not implement crypto, schema changes, backend decryption, browser decryption, key escrow, or wrapped-key delivery in this task.

### 5. Preserve prompt naming conventions

Do not rename reusable prompts unless needed. If any reusable prompt filename violates the current `NN-short-kebab-title.md` convention, report it before changing it.

Do not modify historical prompts under `codex/archive/`, `codex/features/`, `codex/refactors/`, or `codex/work-orders/` except this work-order file.

### 6. Add rate-limit coverage checks to security review prompt

Update `codex/prompts/30-security-review.md` so security review explicitly checks app-level rate-limit coverage for current route classes.

Add a rate-limit-specific checklist under `## Review focus` that covers:

- route registration and rate-limit classifier coverage staying aligned;
- every new main `/v1` route being assigned a rate-limit class or explicitly documented as intentionally unclassified;
- authenticated write routes for accounts, contacts, sharing grants, wrapped keys, tokens, uploads, streams, deletion, and auth/session flows having appropriate rate-limit coverage;
- browser cookie-auth routes such as web login, web logout, and CSRF being checked for rate-limit coverage or explicitly justified;
- tests covering rate-limit classification for every registered main API route class where practical;
- stale, unused, or misleading rate-limit config fields being removed, renamed, wired, or documented.

Also update the security-review output requirements so Codex reports any registered routes that appear missing from the rate-limit classifier.

This work order only updates the reusable security review prompt. Do not fix runtime rate limiting in this branch. If the route audit finds actual unclassified routes, report that as a follow-up security-hardening issue or separate branch, without including sensitive details beyond route categories that are safe for public issue tracking.

## Explicit non-goals

Do not:

- change Go runtime code;
- change rate-limit classifier code in this branch;
- change migrations;
- change API behavior;
- add backend decryption;
- add browser decryption;
- add key escrow;
- add raw server-held keys;
- add trusted-contact decryption or wrapped-key delivery behavior;
- add notifications;
- add recording/capture;
- add paid registration, billing, Stripe, or payment-provider behavior;
- create GitHub issues;
- create or merge pull requests;
- edit sibling repositories.

## Sensitive-data rules

Do not include raw tokens, secrets, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, raw media keys, contact private keys, unwrapped secrets, wrapped-key ciphertext, exploit payloads, object-store credentials, stored paths, object keys, private deployment details, or user safety data in public docs, prompts, comments, tests, or summaries.

## Validation

For prompt/docs-only changes, run:

```bash
git diff --check
git diff --stat
git diff -- AGENTS.md codex docs README.md SECURITY.md CHANGELOG.md
```

If any Go code changes unexpectedly, stop and explain why. Do not run Go validation unless code changed or the maintainer explicitly requests it.

## Output

Summarize:

1. files changed;
2. validation wording updated;
3. review-only/fix-mode wording updated;
4. key-custody prompt wording updated;
5. whether existing design-doc handling was changed from create-only to create-or-update;
6. rate-limit coverage review wording added to `30-security-review.md`;
7. prompts intentionally not changed;
8. validation commands run and results;
9. confirmation that no runtime code, migrations, API behavior, issues, PRs, or sibling repositories were changed.
