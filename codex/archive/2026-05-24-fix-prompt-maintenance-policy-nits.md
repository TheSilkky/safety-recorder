# One-off Codex Work Order: Fix Prompt Maintenance Policy Review Nits

Historical/reference-only prompt.

This is a one-off documentation/process maintenance task for Safety Recorder. Do not treat this file as a reusable workflow prompt after the task is complete.

## Goal

Address two non-blocking review nits on the `prompt-maintenance-policy` branch:

1. Strengthen the `/v1` exposure/authentication row in `codex/README.md` so it explicitly mentions the deployment, security model, and threat model docs.
2. Add one concise cross-reference from `docs/codex-change-control.md` back to the prompt-maintenance policy section in `codex/README.md`.

Keep the change boring, scoped, reviewable, and documentation-only. This should be a tiny follow-up diff, not a paperwork dragon nesting habitat.

## Branch

Work on the existing branch:

```text
prompt-maintenance-policy
```

Before editing, check:

```bash
git status --short --branch --untracked-files=all
git branch --show-current
```

If the current branch is not `prompt-maintenance-policy`, stop and ask the maintainer before changing files.

## Source of truth

Before editing, read the current versions of:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `codex/README.md`
- `docs/codex-change-control.md`
- `docs/deployment.md`
- `docs/security-model.md`
- `docs/threat-model.md`

Do not rely on stale assumptions from this prompt if current repository files disagree.

## Scope

Allowed:

- Update `codex/README.md`.
- Update `docs/codex-change-control.md`.
- Optionally update this one-off prompt only if the maintainer has asked it to be committed under `codex/work-orders/`.

Not allowed:

- Do not change Go code.
- Do not change SQL migrations.
- Do not change Docker files.
- Do not change CI workflows.
- Do not create or modify reusable prompts.
- Do not move or rename existing prompts.
- Do not edit historical prompts in `codex/archive/`, `codex/features/`, `codex/refactors/`, or `codex/work-orders/` except this one-off file if explicitly being saved.
- Do not create GitHub issues.
- Do not claim production readiness.
- Do not weaken security warnings.
- Do not expose private `/v1` APIs publicly.
- Do not introduce or imply implementation of backend decryption, browser decryption, raw server-held keys, key escrow, key sharing, SMS, push notifications, OAuth, JWT, user accounts, public admin dashboards, Docker Compose, Kubernetes, or cloud integrations.

## Required changes

### 1. Strengthen the `/v1` row in `codex/README.md`

In `codex/README.md`, find the `## When To Update Prompts` trigger matrix.

Update the row for private `/v1` exposure or authentication model changes so it explicitly names the relevant docs as well as reusable prompts.

Suggested replacement wording:

```md
| Private `/v1` exposure or authentication model changes | Review `AGENTS.md`, `docs/deployment.md`, `docs/security-model.md`, `docs/threat-model.md`, and every reusable prompt that references private/public route separation. |
```

If the surrounding table wording has changed, adapt the wording while preserving the intent:

- `/v1` exposure/auth changes must trigger review of deployment guidance.
- `/v1` exposure/auth changes must trigger review of the security model and threat model.
- `/v1` exposure/auth changes must trigger review of reusable prompts that mention private/public route separation.
- Do not imply `/v1` is safe for public exposure unless the actual authentication/deployment model has changed and been documented.

### 2. Add a concise cross-reference in `docs/codex-change-control.md`

Add one short cross-reference to the prompt-maintenance policy section in `codex/README.md`.

Suggested placement: near the end of the `Before running Codex` checklist, after the step that says to read current source-of-truth docs and relevant prompts.

Suggested wording:

```md
For prompt-maintenance triggers, see [Codex prompt maintenance](../codex/README.md#when-to-update-prompts).
```

If another nearby location reads better, use it. Keep it short. Do not duplicate the trigger matrix in `docs/codex-change-control.md`.

## Style requirements

- Keep wording concise and practical.
- Preserve the existing tone of the surrounding docs.
- Do not add jokes to public docs.
- Do not turn this into a governance framework, policy thesis, or other paperwork dragon habitat.
- Keep `codex/README.md` as the canonical home for the trigger matrix.
- Keep `docs/codex-change-control.md` as a change-control workflow doc with only a cross-reference.

## Validation

After editing, run:

```bash
git diff --stat
git diff -- codex/README.md docs/codex-change-control.md
```

If any non-Markdown files changed, stop and explain why.

If Go code changed accidentally, revert those code changes. Go tests are not required for this documentation-only task.

Manually inspect:

- Markdown table formatting in `codex/README.md`
- the anchor link to `../codex/README.md#when-to-update-prompts`
- consistency with `AGENTS.md`
- no weakened `/v1` exposure warning
- no new production-readiness claims
- no duplicated trigger matrix in `docs/codex-change-control.md`
- no reusable prompt changes

## Expected result

Expected changed files:

```text
codex/README.md
docs/codex-change-control.md
```

Optional changed file only if this prompt is being committed:

```text
codex/work-orders/2026-05-24-fix-prompt-maintenance-policy-nits.md
```

## Output

Summarize:

1. files changed
2. exact `/v1` trigger row wording used
3. where the cross-reference was added
4. validation commands run
5. confirmation that no application code, CI, Docker, SQL, reusable prompts, or runtime behaviour changed

Do not claim production readiness.
Do not claim this is a formal security audit.
Do not create GitHub issues.
