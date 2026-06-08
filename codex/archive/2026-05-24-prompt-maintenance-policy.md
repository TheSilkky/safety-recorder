# One-off Codex Work Order: Add Prompt Maintenance Policy

Historical/reference-only prompt.

This is a one-off documentation/process maintenance task for Safety Recorder. Do not treat this file as a reusable workflow prompt after the task is complete.

## Goal

Update the repository documentation so Codex and Deep Research prompts have a clear maintenance policy as the project evolves.

The main outcome should be a small, practical section in `codex/README.md` explaining when and how reusable Codex prompts, one-off prompts, and Deep Research report prompts should be updated when project scope, architecture, security posture, or workflow changes.

Keep the change boring, scoped, reviewable, and documentation-only.

## Source of truth

Before editing, read the current versions of:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `docs/README.md`
- `docs/development.md`
- `docs/codex-change-control.md`
- `docs/reports/README.md`
- `docs/reports/prompts/phase-1-deep-research-technical-review.md`
- `codex/README.md`
- `codex/prompts/15-codex-structure-and-naming-maintenance.md`
- `codex/prompts/36-update-codex-key-custody-guardrails.md`
- `codex/prompts/40-documentation-update.md`
- `codex/prompts/95-validate-deep-research-report.md`

Do not rely on stale assumptions from this prompt if current repository files disagree.

## Scope

Allowed:

- Update `codex/README.md`.
- Optionally update `docs/development.md` if a short cross-reference is useful.
- Optionally update `docs/reports/README.md` if the Deep Research prompt maintenance relationship needs one concise clarification.
- Optionally update `docs/codex-change-control.md` if the change-control workflow should mention prompt-maintenance triggers.
- Optionally add this one-off prompt under `codex/work-orders/` if the maintainer has not already saved it there.

Not allowed:

- Do not change Go code.
- Do not change SQL migrations.
- Do not change Docker files.
- Do not change CI workflows.
- Do not create new reusable prompts unless a repeated workflow is clearly needed.
- Do not move or rename existing prompts unless required to fix a direct inconsistency.
- Do not edit historical prompts in `codex/archive/`, `codex/features/`, `codex/refactors/`, or `codex/work-orders/` except to add this one-off prompt.
- Do not create GitHub issues.
- Do not claim production readiness.
- Do not weaken security warnings.
- Do not expose private `/v1` APIs publicly.
- Do not introduce or imply implementation of backend decryption, browser decryption, server-held raw keys, key escrow, key sharing, SMS, push notifications, OAuth, JWT, user accounts, public admin dashboards, Docker Compose, Kubernetes, or cloud integrations.

## Required content

Add a section to `codex/README.md` with a title similar to:

```md
## When To Update Prompts
```

The section should explain:

1. Source-of-truth order:
   - current code and source-of-truth docs come first
   - reusable Codex prompts are workflow helpers, not project truth
   - Deep Research prompts are report-generation/validation helpers, not project truth
   - historical prompts are reference-only

2. Prompt update order:
   - update implementation or design docs first
   - update `README.md`, `AGENTS.md`, `SECURITY.md`, and relevant `docs/` files as needed
   - update reusable Codex prompts only when their assumptions or guardrails changed
   - update Deep Research prompts when report scope, citation policy, source policy, or recurring validation failures change
   - leave historical prompts untouched unless explicitly requested

3. Trigger matrix:

   Include a compact Markdown table covering at least:

   | Project change | Prompt/doc action |
   |---|---|
   | New API routes or listener exposure | Review `AGENTS.md`, `docs/api.md`, security/threat docs, and relevant review prompts. |
   | Private `/v1` exposure or authentication model changes | Update every reusable prompt that references private/public route separation. |
   | Encryption envelope changes | Update `docs/encryption.md`, simulator prompt, security prompt, and Deep Research review scope. |
   | Key custody, browser decryption, break-glass, or dead-man-switch design changes | Use/update the key-custody prompts and update threat model, security model, encryption docs, and operational guidance. |
   | Bundle, storage, schema, or manifest changes | Update API docs, code-map docs, simulator docs/prompts, and Deep Research scope. |
   | CI/CD, Docker, GHCR, or release workflow changes | Update release/development docs and release/report prompts. |
   | New repeated Codex workflow | Add one reusable `NN-short-kebab-title.md` prompt and list it in `codex/README.md`. |
   | One-off implementation, refactor, or work order | Add a dated historical prompt under `features/`, `refactors/`, or `work-orders/`. |
   | Validated Deep Research report finds a recurring false-positive pattern | Update Phase 1 and/or Phase 2 report prompts so the same mistake is less likely to recur. |

4. Key custody guardrail:
   - preserve the current ciphertext-only backend unless the task explicitly concerns key custody, emergency access, or decryption design
   - do not make “no server keys ever” a permanent absolute rule
   - do not introduce backend decryption, browser decryption, raw server-held keys, key escrow, or key-sharing behaviour incidentally
   - explicit key custody/decryption work must update threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation

5. Deep Research maintenance:
   - Phase 1 prompt lives under `docs/reports/prompts/`
   - Phase 2 validation prompt lives under `codex/prompts/95-validate-deep-research-report.md`
   - review them together when report workflow changes
   - keep portable citation keys
   - pin repository citations to reviewed commits
   - do not allow ChatGPT internal citation tokens in public reports
   - add newly discovered recurring false positives to the Phase 2 checklist

6. Reusable prompt discipline:
   - do not add a reusable prompt for every one-off idea
   - add reusable prompts only for repeated workflows
   - one-off prompts belong in dated historical directories
   - generated local artifacts belong outside `codex/`

## Style requirements

Keep the wording concise and practical.

Do not turn this into a governance manifesto, compliance framework, AI policy thesis, or other paperwork dragon nesting habitat.

Use project terminology consistently:

- reusable prompts
- historical prompts
- one-off prompts
- Deep Research Phase 1
- Codex Phase 2 validation
- source-of-truth docs
- key custody guardrails
- public-safe report workflow

Avoid jokes in repository documentation unless the surrounding file already uses light humour and the wording remains professional enough for a public repository.

## Suggested files to edit

Primary:

- `codex/README.md`

Optional, only if needed for cross-reference consistency:

- `docs/development.md`
- `docs/reports/README.md`
- `docs/codex-change-control.md`

## Validation

After editing, run:

```bash
git diff --stat
git diff -- codex/README.md docs/development.md docs/reports/README.md docs/codex-change-control.md codex/work-orders
```

If any non-Markdown files changed, stop and explain why.

If Go code changed accidentally, revert those code changes. Go tests are not required for this documentation-only task.

Manually inspect:

- Markdown headings
- internal links
- prompt filenames
- whether the new section contradicts `AGENTS.md`
- whether key custody wording distinguishes current implementation from future explicit design
- whether Deep Research Phase 1 and Phase 2 responsibilities remain separate
- whether historical prompts remain reference-only

## Output

Summarize:

1. files changed
2. section(s) added or updated
3. whether any optional docs were touched
4. whether any reusable prompts were changed
5. whether historical prompts were left untouched, except this one-off prompt if saved
6. validation commands run
7. confirmation that no application code, CI, Docker, SQL, or runtime behaviour changed

Do not claim production readiness.
Do not claim this is a formal security audit.
Do not create GitHub issues.
