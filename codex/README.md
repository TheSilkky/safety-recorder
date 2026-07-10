# Codex Prompts

This directory records the Codex prompt workflow used for AI-assisted
development in `open-proofline/server`.

Codex output is treated as maintainer-reviewed work, not as endorsement, audit, certification, security review, or maintenance by OpenAI.

The server repository owns server behavior, API, deployment, security, and
release workflow facts. The website repository owns project-wide public
governance posture, political alignment, public-good framing, public voice,
reusable README baseline guidance, and source-of-truth mapping:

- [`open-proofline/website/docs/governance-and-political-alignment.md`](https://github.com/open-proofline/website/blob/main/docs/governance-and-political-alignment.md)
- [`open-proofline/website/docs/repository-readme-baseline.md`](https://github.com/open-proofline/website/blob/main/docs/repository-readme-baseline.md)

Reusable server prompts that touch README structure, public-facing wording,
project-wide governance, public-good framing, or source-of-truth mapping should
inspect those website documents and link to them instead of re-declaring the
project posture inside server docs.

## Directory Structure

Keep the Codex workflow in this structure:

```text
codex/
  README.md
  prompts/
  archive/
  work-orders/
```

Do not add extra prompt directories without a clear workflow reason. Generated local review artifacts belong outside `codex/`.

## Prompt Categories

Reusable prompts live in `codex/prompts/`. They are scoped workflows that can be run again against the current repository after reading current source-of-truth docs.

Historical prompts live in:

```text
codex/archive/
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

- `.technical-review-drafts/` for source-cited Codex Phase 1 report drafts that
  still require independent Phase 2 validation before publication
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
12. `45-documentation-and-prompt-review.md`, for comprehensive documentation and reusable-prompt consistency review
13. `50-mdn-web-security-header-review.md`, for web-facing changes
14. `60-simulator-maintenance.md`, for API/client-flow changes

### Issue and PR workflow

15. `70-work-on-github-issue.md`
16. `75-create-draft-pr-from-current-branch.md`
17. `76-request-codex-pr-review.md`

### Backlog workflow

18. `80-backlog-scan-issue-drafts.md`
19. `81-backlog-drafts-structure-and-hygiene.md`
20. `82-review-open-issues-for-stale-or-fixed.md`
21. `85-create-github-issues-from-drafts.md`

### Release workflow

22. `90-release-check.md`
23. `95-validate-technical-review-report.md`, for independent Phase 2 validation of public technical review reports

For any `v1 preview`, `v1.0.0`, or real-user evidence-upload readiness claim,
run [docs/v1-preview-readiness-checklist.md](../docs/v1-preview-readiness-checklist.md)
as part of the release workflow before using preview-ready language.

## Current Project Constraints

Treat `README.md`, `AGENTS.md`, `SECURITY.md`, and the `docs/` directory as the current source of truth. For v1 preview terminology, repository roles, and current-versus-future product direction, read `docs/v1-preview-direction.md` before turning prototype gaps into backlog or implementation assumptions.
For v1 preview release claims, also read
`docs/v1-preview-readiness-checklist.md` and preserve its hard-blocker,
non-goal, optional hosted-service, and issue-hygiene boundaries.

For public governance posture, political alignment, public-good framing,
public voice, README baseline style, and source-of-truth mapping, read the two
website source documents above. Keep server-specific facts in this repository;
link project-wide posture to the website source of truth.

Product documentation now uses the name Proofline. The repository URL is `open-proofline/server`, the root Go module path is `github.com/open-proofline/server`, release binaries use `proofline-server-*` names, and the published GHCR image is `ghcr.io/open-proofline/server`. Current runtime protocol and default data-layout identifiers use Proofline names. Historical reports and archived prompts may still mention earlier `safety-recorder` identifiers.

Core constraints:

- Keep the backend small, boring, and testable.
- Prefer Go standard library where practical.
- Keep main `/v1` routes behind the reviewed deployment boundary and do not route `/v1/admin/...` from public viewer edges.
- Keep the main API/viewer route tree and the private `/admin` dashboard route tree on separate listener groups and muxes.
- Treat uploaded chunks as immutable.
- Evidence bundles are encrypted chunk bundles, not decrypted/playable media exports.
- Do not log raw viewer tokens, incident tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, wrapped-key ciphertext, private deployment details, stored paths, object keys, user safety data, or future token-like values. Logging changes should follow `docs/logging-requirements.md`.
- Use stable, documented crypto libraries only. Do not implement cryptographic primitives.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour incidentally.
- Key custody/decryption changes must be explicit security-sensitive work and update the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.
- Future product scope includes emergency incidents, non-emergency interaction records, timed safety checks, and evidence notes. The current backend stores generic incidents by default and can store optional incident-mode, capture-profile, escalation-policy, and sharing-state metadata as labels only.
- Mode-driven access, first-class capture-profile behavior, escalation
  policies, sharing-state behavior, trusted-contact accounts, dead-man switch
  notifications, public account portals, and public `/v1` product
  authentication are not implemented yet.
- Do not add React, Node, npm, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, public admin dashboards, Docker Compose, Kubernetes, or cloud integrations unless explicitly requested.
- Put newly discovered future work into issues/backlog items unless it is required for the current task.
- Backlog scanning creates draft Markdown files first, not GitHub issues directly.
- Do not create public GitHub issues from backlog drafts until the maintainer has reviewed them.
- Never put raw tokens, secrets, private deployment details, exploit details, or user safety data into public issue drafts.

## When To Update Prompts

Treat current code and source-of-truth docs as project truth. Reusable prompts
are workflow helpers, Codex technical-review prompts are report-generation and
validation helpers, and historical prompts are reference-only.

When project scope, architecture, security posture, or workflow changes, update implementation or design docs first. Then update `README.md`, `AGENTS.md`, `SECURITY.md`, and relevant `docs/` files as needed. Update reusable Codex prompts only when their assumptions, guardrails, or repeated workflow steps have changed. Update Codex technical-review prompts when report scope, citation policy, source policy, execution-evidence policy, or recurring validation failures change. Leave historical prompts untouched unless the maintainer explicitly requests otherwise.

| Project change | Prompt/doc action |
|---|---|
| README baseline, public voice, governance posture, public-good framing, or source-of-truth mapping changes | Read the website governance and README baseline docs, update `README.md`, `AGENTS.md`, `docs/`, `codex/README.md`, and reusable prompts only where they consume that project-wide source of truth. |
| Product rename or repository/artifact namespace migration | Update `README.md`, `AGENTS.md`, `SECURITY.md`, relevant `docs/`, `codex/README.md`, and reusable prompts that mention product or artifact names. Keep docs-only renames separate from repository/module/Docker/GHCR migrations. |
| First-class incident modes, capture profiles, escalation policies, sharing state, safety checks, interaction records, or evidence notes | Update `docs/incident-modes.md`, `README.md`, API docs, security/threat docs, client prototype docs, and relevant review prompts. |
| New API routes or listener exposure | Review `AGENTS.md`, `docs/api.md`, security/threat docs, and relevant review prompts. |
| Private `/v1` exposure or authentication model changes | Review `AGENTS.md`, `docs/deployment.md`, `docs/security-model.md`, `docs/threat-model.md`, and every reusable prompt that references private/public route separation. |
| Logging behavior, startup/config error logs, request logs, or worker/operator logs | Review `docs/logging-requirements.md`, `docs/security-model.md`, `docs/threat-model.md`, and relevant review prompts. |
| Encryption envelope changes | Update `docs/encryption.md`, `docs/post-quantum-envelope.md`, `docs/simulator.md`, `60-simulator-maintenance.md`, `30-security-review.md`, and the Codex technical-review scope. |
| Key custody, browser decryption, break-glass, or dead-man-switch design changes | Use or update the key-custody prompts and update threat model, security model, encryption docs, incident-mode docs, and operational guidance. |
| Bundle, storage, schema, or manifest changes | Update API docs, code-map docs, simulator docs/prompts, and the Codex technical-review scope. |
| CI/CD, Docker, GHCR, or release workflow changes | Update release/development docs and release/report prompts. |
| New repeated Codex workflow | Add one reusable `NN-short-kebab-title.md` prompt and list it in this README. |
| One-off implementation, refactor, or work order | Add a dated historical prompt under `archive/` or `work-orders/`. |
| Validated technical review finds a recurring false-positive pattern | Update the Codex Phase 1 and/or independent Phase 2 validation prompts so the same mistake is less likely to recur. |

Key custody guardrails need special care. Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design. Do not turn "no server keys ever" into a permanent absolute rule, and do not introduce backend decryption, browser decryption, raw server-held keys, key escrow, or key-sharing behaviour incidentally. Explicit key custody or decryption work must update the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.

For the public-safe report workflow, review the Codex Phase 0 preflight, Codex
Phase 1 draft, and independent Codex Phase 2 validation prompts together when
the workflow changes. Phase 0 lives in
`docs/reports/prompts/phase-0-codex-technical-review-preflight.md`, Phase 1 lives
in `docs/reports/prompts/phase-1-codex-technical-review.md`, and Phase 2 lives in
`codex/prompts/95-validate-technical-review-report.md`. Keep portable citation
keys, pin repository citations to reviewed commits, remove internal renderer or
tool citation identifiers from public reports, and add newly discovered
recurring false positives to the Phase 2 checklist.

Do not add a reusable prompt for every one-off idea. Add reusable prompts only for repeated workflows. One-off prompts belong in `archive/` or `work-orders/`, and generated local artifacts belong outside `codex/`.

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

Use `docs/reports/prompts/phase-0-codex-technical-review-preflight.md` in Codex
Max for a focused single-agent preflight, or Ultra when meaningful independent
read-only lanes justify subagent delegation. Load the governing Phase 1 prompt,
prepare the source and execution plan, and stop for maintainer approval.

After approval, use `docs/reports/prompts/phase-1-codex-technical-review.md` in
Codex Max for a focused review, or Ultra when the review has meaningful
independent research lanes. Create a source-cited draft under the ignored
`.technical-review-drafts/` directory. Phase 1 may directly execute safe
validation commands, but its Source Registry must record the exact command,
context, result, and limitations.

Use `95-validate-technical-review-report.md` as an independent Phase 2 pass to
verify repository and execution-evidence claims, remove draft-only material,
pin repository citations, check public-safety constraints, and publish the
cleaned report under `docs/reports/`.

## Validation

Before accepting Codex changes that touch Go code:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

For docs-only changes, run `git diff --check` and inspect the relevant
Markdown. Run the local Markdown link checker for documentation or reusable
prompt changes:

```bash
scripts/check-markdown-links.py
```

The checker validates local links in `README.md`, `AGENTS.md`, `SECURITY.md`,
`docs/**/*.md`, and `codex/**/*.md`, including simple GitHub-style Markdown
heading anchors. It skips external URLs and fenced code examples, uses only the
Python standard library, and does not require network access, Node/npm, Docker,
cloud services, or secrets. When changing the checker itself, also run:

```bash
scripts/check-markdown-links.py --self-test
```

Go tests are not required unless code changed.

For simulator/API flow changes, also run the simulator smoke test when
practical. Prefer an explicit TOML config for repeatable smoke tests,
especially when the test database still needs a bootstrap secret:

```toml
[auth]
bootstrap_secret_file = "/path/to/local-bootstrap-secret"
```

```bash
go run ./cmd/api --config /path/to/proofline-smoke.toml
```

In another terminal:

```bash
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```
