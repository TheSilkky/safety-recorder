# Codex Prompt: Documentation Update

Update documentation to match the current code and project scope.

Do **not** change code unless documentation reveals a clear inconsistency or broken example and the maintainer explicitly wants it fixed.
Do **not** overpromise production readiness.

## Source of truth

Before making changes, read current source-of-truth files as relevant:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
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
- Do not expose `/v1` as an unreviewed public catch-all; keep `/v1/admin/...` and `/admin` off public edges.
- Do not log raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, wrapped-key ciphertext, private deployment details, stored paths, object keys, user safety data, or future token-like values; check logging documentation against `docs/logging-requirements.md`.
- Do not add React, Node, npm, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features unless explicitly requested.
- Prefer Go standard library where practical.
- Preserve private/public listener separation.
- Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour as an incidental implementation detail.
- Any key custody/decryption change must be an explicit security-sensitive task that updates the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.

## Potential docs to update

Update only relevant files:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`, only for small consistency updates
- `docs/README.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/configuration.md`
- `docs/deployment.md`
- `docs/encryption.md`
- `docs/key-custody.md`, if present
- `docs/browser-decryption.md`, if present
- `docs/break-glass-key-access.md`, if present
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/simulator.md`
- `docs/development.md`
- `docs/codex-change-control.md`
- `docs/code-map.md`
- `.github/ISSUE_TEMPLATE/*.md`, if docs/process changes require it
- `codex/README.md`
- `codex/prompts/*.md`, if prompt workflow documentation changes require it

## Check documentation for

- project scope
- current version number, if stated
- endpoint list
- request/response examples
- environment variables
- bind address behaviour
- Docker bind caveat
- private/public listener separation
- data directory layout
- media stream lifecycle
- completed evidence bundle downloads
- encryption envelope format and simulator key-file behaviour
- current backend does not decrypt or store raw keys by default
- future hybrid key custody direction, if design docs exist
- incident viewer download buttons
- simulator commands
- security warnings
- known limitations
- test/run/build commands
- CI/GHCR notes
- issue/PR workflow
- backlog draft workflow
- Codex change-control workflow
- AI-assisted development disclosure
- next steps / roadmap

## Constraints

- Do not imply evidence bundles are playable media.
- Do not imply broad `/v1` public exposure is safe without route-level deployment review.
- Do not claim the iOS client exists.
- Do not claim production-readiness.
- Do not describe future key custody/decryption as implemented unless it is implemented.
- Keep wording clear and concise.

## Validation

If only Markdown changed:

```bash
git diff --stat
git diff -- README.md docs codex AGENTS.md SECURITY.md CHANGELOG.md .github/ISSUE_TEMPLATE
git diff --check
```

Go tests are not required unless code changed.

If code changed unexpectedly, stop and explain why.

## Output

Summarize:

1. files changed
2. docs updated
3. docs intentionally not touched
4. links/examples that should be manually checked
5. whether tests were skipped as docs-only
