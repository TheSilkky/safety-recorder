# Codex Prompts

This directory records the Codex prompt workflow used for AI-assisted development.

Codex output is treated as maintainer-reviewed work, not as endorsement, audit, certification, security review, or maintenance by OpenAI.

## Directory Structure

Keep the Codex workflow in this structure:

```text
codex/
  README.md
  prompts/
  archive/
  features/
  refactors/
  work-orders/
```

Do not add extra prompt directories without a clear workflow reason. Generated local review artifacts belong outside `codex/`.

## Prompt Categories

Reusable prompts live in `codex/prompts/`. They are scoped workflows that can be run again against the current repository after reading current source-of-truth docs.

Historical prompts live in:

```text
codex/archive/
codex/features/
codex/refactors/
codex/work-orders/
```

Historical prompts are reference material only. Do not re-run historical prompts without checking them against the current `README.md`, `AGENTS.md`, `SECURITY.md`, relevant docs, and reusable prompts.

## Naming Conventions

Reusable prompts use this filename pattern:

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

Historical prompts use this filename pattern:

```text
YYYY-MM-DD-short-kebab-title.md
```

Rules:

- date prefix
- kebab-case title
- `.md` extension
- no numeric reusable-workflow prefix
- each file should be clearly marked historical/reference-only near the top

## Generated Artifacts

Generated local artifacts should not be placed under `codex/`.

Current generated artifact locations:

- `.backlog-drafts/YYYY-MM-DD/<branch-slug>/` or `.backlog-drafts/current/<branch-slug>/` for backlog issue drafts
- `.issue-review-drafts/YYYY-MM-DD/<branch-slug>/` or `.issue-review-drafts/current/<branch-slug>/` for open-issue review drafts
- `scripts/create-backlog-issues.sh` only when explicitly generated from reviewed backlog drafts

Backlog and issue-review drafts must not include raw tokens, secrets, private deployment details, exploit details, or user safety data. Public GitHub issues must not be created from drafts until the maintainer reviews them.

Backlog draft directories should include a `README.md` index, public issue drafts named `NNN-short-kebab-title.md`, and a `private-notes/README.md` guardrail when private notes are present or expected. Public issue drafts must include `## Priority`, `## Type`, `## Labels`, and `## Branch scope`, including the `backlog` label plus at least one existing topic/type label. Private notes must never be used for public issue creation.

## Normal reusable prompt order

Use prompts in this rough order:

### Context and readiness

1. `00-project-context-check.md`
2. `05-codex-change-control.md`

### Maintenance, review, and design

3. `10-readability-maintenance.md`
4. `15-codex-structure-and-naming-maintenance.md`
5. `20-code-review.md`
6. `30-security-review.md`
7. `35-key-custody-and-emergency-access-design.md`
8. `36-update-codex-key-custody-guardrails.md`
9. `37-browser-decryption-design-spike.md`
10. `38-break-glass-and-dead-mans-switch-key-access-design.md`
11. `40-documentation-update.md`
12. `50-mdn-web-security-header-review.md`, for web-facing changes
13. `60-simulator-maintenance.md`, for API/client-flow changes

### Issue and PR workflow

14. `70-work-on-github-issue.md`
15. `75-create-draft-pr-from-current-branch.md`
16. `76-request-codex-pr-review.md`

### Backlog workflow

17. `80-backlog-scan-issue-drafts.md`
18. `81-backlog-drafts-structure-and-hygiene.md`
19. `82-review-open-issues-for-stale-or-fixed.md`
20. `85-create-github-issues-from-drafts.md`

### Release workflow

21. `90-release-check.md`
22. `95-validate-deep-research-report.md`, for Phase 2 validation of public technical review reports

## Current project constraints

Treat `README.md`, `AGENTS.md`, `SECURITY.md`, and the `docs/` directory as the current source of truth.

Product documentation now uses the name Proofline. The repository URL is `open-proofline/server`, the root Go module path is `github.com/open-proofline/server`, release binaries use `proofline-server-*` names, and the published GHCR image is `ghcr.io/open-proofline/server`. Compatibility identifiers such as the v1 simulator encryption envelope and default SQLite filename may still use earlier `safety-recorder` names until separate protocol or data-layout migrations are explicitly performed.

Core constraints:

- Keep the backend small, boring, and testable.
- Prefer Go standard library where practical.
- Keep main `/v1` routes behind the reviewed deployment boundary and do not route `/v1/admin/...` from public viewer edges.
- Keep the main API/viewer route tree and the private `/admin` dashboard route tree on separate listener groups and muxes.
- Treat uploaded chunks as immutable.
- Evidence bundles are encrypted chunk bundles, not decrypted/playable media exports.
- Do not log raw viewer tokens, incident tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, or future token-like values.
- Use stable, documented crypto libraries only. Do not implement cryptographic primitives.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour incidentally.
- Key custody/decryption changes must be explicit security-sensitive work and update the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.
- Future product scope includes emergency incidents, non-emergency interaction records, timed safety checks, and evidence notes, but the current backend only stores generic incidents.
- First-class incident modes, capture profiles, escalation policies, sharing
  state, public account workflows, trusted-contact accounts, dead-man switch
  notifications, and public `/v1` product authentication are not implemented
  yet.
- Do not add React, Node, npm, OAuth, JWT, user accounts, SMS, Messenger, push notifications, public admin dashboards, Docker Compose, Kubernetes, or cloud integrations unless explicitly requested.
- Put newly discovered future work into issues/backlog items unless it is required for the current task.
- Backlog scanning creates draft Markdown files first, not GitHub issues directly.
- Do not create public GitHub issues from backlog drafts until the maintainer has reviewed them.
- Never put raw tokens, secrets, private deployment details, exploit details, or user safety data into public issue drafts.

## When To Update Prompts

Treat current code and source-of-truth docs as project truth. Reusable prompts are workflow helpers, Deep Research prompts are report-generation and validation helpers, and historical prompts are reference-only.

When project scope, architecture, security posture, or workflow changes, update implementation or design docs first. Then update `README.md`, `AGENTS.md`, `SECURITY.md`, and relevant `docs/` files as needed. Update reusable Codex prompts only when their assumptions, guardrails, or repeated workflow steps have changed. Update Deep Research prompts when report scope, citation policy, source policy, or recurring validation failures change. Leave historical prompts untouched unless the maintainer explicitly requests otherwise.

| Project change | Prompt/doc action |
|---|---|
| Product rename or repository/artifact namespace migration | Update `README.md`, `AGENTS.md`, `SECURITY.md`, relevant `docs/`, `codex/README.md`, and reusable prompts that mention product or artifact names. Keep docs-only renames separate from repository/module/Docker/GHCR migrations. |
| First-class incident modes, capture profiles, escalation policies, sharing state, safety checks, interaction records, or evidence notes | Update `docs/incident-modes.md`, `README.md`, API docs, security/threat docs, client prototype docs, and relevant review prompts. |
| New API routes or listener exposure | Review `AGENTS.md`, `docs/api.md`, security/threat docs, and relevant review prompts. |
| Private `/v1` exposure or authentication model changes | Review `AGENTS.md`, `docs/deployment.md`, `docs/security-model.md`, `docs/threat-model.md`, and every reusable prompt that references private/public route separation. |
| Encryption envelope changes | Update `docs/encryption.md`, `60-simulator-maintenance.md`, `30-security-review.md`, and Deep Research review scope. |
| Key custody, browser decryption, break-glass, or dead-man-switch design changes | Use or update the key-custody prompts and update threat model, security model, encryption docs, incident-mode docs, and operational guidance. |
| Bundle, storage, schema, or manifest changes | Update API docs, code-map docs, simulator docs/prompts, and Deep Research scope. |
| CI/CD, Docker, GHCR, or release workflow changes | Update release/development docs and release/report prompts. |
| New repeated Codex workflow | Add one reusable `NN-short-kebab-title.md` prompt and list it in this README. |
| One-off implementation, refactor, or work order | Add a dated historical prompt under `features/`, `refactors/`, or `work-orders/`. |
| Validated Deep Research report finds a recurring false-positive pattern | Update the Deep Research Phase 1 and/or Codex Phase 2 validation prompts so the same mistake is less likely to recur. |

Key custody guardrails need special care. Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design. Do not turn "no server keys ever" into a permanent absolute rule, and do not introduce backend decryption, browser decryption, raw server-held keys, key escrow, or key-sharing behaviour incidentally. Explicit key custody or decryption work must update the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.

For the public-safe report workflow, review Deep Research Phase 1 and Codex Phase 2 validation together when report workflow changes. Phase 1 lives in `docs/reports/prompts/phase-1-deep-research-technical-review.md`. Codex Phase 2 validation lives in `codex/prompts/95-validate-deep-research-report.md`. Keep portable citation keys, pin repository citations to reviewed commits, do not allow ChatGPT internal citation tokens in public reports, and add newly discovered recurring false positives to the Phase 2 checklist.

Do not add a reusable prompt for every one-off idea. Add reusable prompts only for repeated workflows. One-off prompts belong in dated historical directories, and generated local artifacts belong outside `codex/`.

## Issue And PR Workflow

Use `70-work-on-github-issue.md` for scoped implementation work tied to one GitHub issue.

Use `75-create-draft-pr-from-current-branch.md` when a reviewed local branch should become a draft pull request.

Use `76-request-codex-pr-review.md` for a code-review pass over an existing pull request.

## Backlog And Issue Review Workflow

Use `80-backlog-scan-issue-drafts.md` to generate timestamped branch-scoped backlog drafts under `.backlog-drafts/`.

Use `81-backlog-drafts-structure-and-hygiene.md` to review or clean up backlog draft structure. It should not create or close GitHub issues.

Use `82-review-open-issues-for-stale-or-fixed.md` to create local issue review drafts under `.issue-review-drafts/`. It should not close GitHub issues unless the maintainer explicitly asks for that follow-up action.

Only after manual review, use `85-create-github-issues-from-drafts.md` to generate a script and review summary for GitHub issue creation. Do not execute that script unless explicitly instructed. Once public issues exist, GitHub Issues become the source of truth and local drafts should be treated as historical generated artifacts.

## Key custody prompt use

Use `35-key-custody-and-emergency-access-design.md` when making the next encryption/key architecture decision.

Use `36-update-codex-key-custody-guardrails.md` when updating prompt wording, docs, or `AGENTS.md` so that "no backend decryption/no server keys" does not become a permanent absolute rule.

Use `37-browser-decryption-design-spike.md` for browser-side incident viewer decryption design.

Use `38-break-glass-and-dead-mans-switch-key-access-design.md` for server escrow, dead-man-switch, or break-glass key access design.

## Technical review report workflow

Use `docs/reports/prompts/phase-1-deep-research-technical-review.md` outside Codex to draft a source-cited public technical review report.

Use `95-validate-deep-research-report.md` in Codex to verify repository claims, remove draft-only material, pin repository citations, check public-safety constraints, and produce a cleaned report under `docs/reports/`.

## Validation

Before accepting Codex changes that touch Go code:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
```

For docs-only changes, inspect the relevant Markdown and links manually. Go tests are not required unless code changed.

For simulator/API flow changes, also run the simulator smoke test when practical:

```bash
go run ./cmd/api
```

In another terminal:

```bash
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```
