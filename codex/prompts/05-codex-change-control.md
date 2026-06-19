# Codex Prompt: Change-Control Check

Review the requested Codex task before making changes.

Do **not** change code.
Do **not** change documentation unless explicitly requested.
Do **not** add features.

## Goal

Decide whether the task is scoped enough to begin, should become a backlog item, or needs one or two specific clarifications.

## Source of truth

Before making changes, read current source-of-truth files as relevant:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
- `docs/v1-preview-direction.md`
- `open-proofline/website/docs/governance-and-political-alignment.md`, when
  public governance posture, political alignment, or public-good framing is in
  scope
- `open-proofline/website/docs/repository-readme-baseline.md`, when README
  structure, public voice, or source-of-truth mapping is in scope
- relevant files in `docs/`
- relevant source files
- relevant tests
- relevant issue or PR, if this is issue/PR work

Do not rely on stale assumptions from this prompt if the repository has changed.
## Global constraints

- Keep changes scoped to the task.
- Do not add unrelated features.
- Do not weaken security warnings.
- Do not claim production readiness.
- Do not expose `/v1` as an unreviewed public catch-all; keep `/admin/api/...` and `/admin` off public edges.
- Do not log raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, wrapped-key ciphertext, private deployment details, stored paths, object keys, user safety data, or future token-like values; check logging changes against `docs/logging-requirements.md`.
- Do not add React, Node, npm, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features unless explicitly requested.
- Prefer Go standard library where practical.
- Preserve private/public listener separation.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour as an incidental implementation detail.
- Any key custody/decryption change must be an explicit security-sensitive task that updates the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.

## Check

Assess whether the task has:

- a clear goal
- likely affected files/areas
- files/areas that must not change
- validation commands
- explicit out-of-scope items
- a rollback/checkpoint point or clean enough working tree
- a clear distinction between required work and future work
- clear key custody/decryption scope, if relevant

## Backlog gate

If the request introduces future work that is not necessary for the current task, recommend creating an issue/backlog item instead of implementing it now.

Security vulnerabilities should follow `SECURITY.md`, not a public issue template. Non-sensitive security hardening can become a normal backlog item.

## Output

Return a short readiness assessment:

- `Ready`: the task is scoped enough to start.
- `Needs clarification`: one or two specific details are missing.
- `Create backlog item`: the request is future work or too broad for the current task.
- `Sensitive security handling`: the request may involve private vulnerability details and should not become a public issue.
- `Key custody design required`: the request would change key custody/decryption and needs explicit design first.

Also include:

- likely validation commands
- recommended prompt to use next
- any suggested issue/backlog title, if applicable
