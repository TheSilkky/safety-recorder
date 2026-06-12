# One-off Codex Work Order: Rename Emergency Viewer And Token Terminology

Historical/reference-only prompt.

This is a one-off implementation work order for Proofline Server. Do not treat this file as a reusable workflow prompt after the task is complete.

## Goal

Rename the current `emergency viewer` and `emergency token` terminology to the broader `incident viewer` and `incident token` terminology across the Go server backend, tests, and current documentation.

This change supports the Proofline product direction where the server handles more than emergency-only recordings. The current repository is the Go server backend only and corresponds to the planned `open-proofline/server` repository.

## Source of truth

Before editing, read the current versions of:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `docs/README.md`
- `docs/architecture.md`
- `docs/api.md`
- `docs/code-map.md`
- `docs/configuration.md`
- `docs/deployment.md`
- `docs/incident-modes.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/development.md`
- `codex/README.md`

Then inspect current implementation and tests under:

- `server/cmd/api`
- `server/cmd/simclient`
- `server/internal/httpapi`
- `server/internal/incidents`
- `server/internal/config`
- `server/migrations`
- current Go tests under `server/`

Do not rely on stale assumptions from this prompt if the repository has changed.

## Scope

This is a naming and compatibility migration, not a feature expansion.

Allowed:

- Rename current canonical terminology from `emergency viewer` to `incident viewer`.
- Rename current canonical terminology from `emergency token` to `incident token`.
- Rename Go types, functions, files, comments, test names, fixtures, docs, and user-facing text where the old wording now describes the broader incident-viewer/token behavior.
- Add canonical incident-token API names, route names, config names, response docs, and tests if the implementation currently exposes emergency-token names.
- Keep explicit legacy aliases where needed to avoid breaking existing simulator/API flows in this release.
- Add deprecation/compatibility notes for any retained legacy emergency-token route names, environment variables, database identifiers, file names, or migration names.
- Update the simulator so normal output and docs use incident-token / incident-viewer terminology.
- Update public docs and current reusable prompt guidance if they describe the canonical terminology.
- Update `CHANGELOG.md` with a concise Unreleased entry.

Not allowed:

- Do not add user accounts, OAuth, JWT, push notifications, SMS, Messenger, web-client, iOS-client, Android-client, or protocol implementation.
- Do not expose private `/v1` routes publicly.
- Do not change the current ciphertext-only backend behavior.
- Do not introduce backend decryption, browser decryption, raw server-held keys, key escrow, or key-sharing behavior.
- Do not change the encryption envelope format as part of this task.
- Do not change evidence bundle contents except for terminology in manifests if that is already part of the documented public format and compatibility is handled.
- Do not change incident-mode behavior or add first-class incident types unless explicitly requested in a separate task.
- Do not create GitHub issues or PRs unless the maintainer explicitly asks.

## Compatibility decision required

Before making implementation changes, decide and document the migration strategy in the Codex summary.

Default preference unless the maintainer says otherwise:

- Introduce canonical `incident` names.
- Preserve legacy `emergency` route/config aliases for at least one release where practical.
- Keep existing data readable.
- Do not silently break the simulator smoke test.
- Document any retained legacy names as compatibility names, not current product terminology.

Specific areas to decide:

1. API routes:
   - Preferred canonical route shape should use `incident-token` terminology.
   - Existing `emergency-token` routes may remain as deprecated aliases if removal would be breaking.

2. Environment variables:
   - Preferred canonical config should use incident-token terminology.
   - Existing `SAFE_DEFAULT_EMERGENCY_TOKEN_TTL` may remain as a fallback alias if removal would be breaking.

3. Database schema:
   - If renaming tables or columns such as `emergency_tokens`, add a migration that preserves existing data.
   - If a schema rename is too risky for this pass, leave the database name as an explicitly documented legacy internal compatibility name and rename only the Go-facing/public terminology.

4. Public viewer path:
   - Decide whether `/e/{token}` remains the stable viewer path for now or whether a new canonical incident-viewer path is added as an alias.
   - Do not remove the existing path unless the maintainer explicitly accepts a breaking route change.

## Required implementation inventory

Start by inventorying current references:

```bash
grep -Rni \
  -e "emergency viewer" \
  -e "EmergencyViewer" \
  -e "emergency_viewer" \
  -e "emergency-token" \
  -e "emergency token" \
  -e "EmergencyToken" \
  -e "emergency_token" \
  -e "emergency_tokens" \
  .
```

Review likely implementation areas:

- HTTP handlers and route registration
- repository models and methods
- migrations and schema tests
- simulator output and client flow
- API docs and examples
- security and threat model docs
- deployment and proxy examples
- tests that assert route names, JSON fields, errors, headers, or logs

## Expected changes

Implementation should make incident terminology canonical in current code and docs.

Examples of likely rename direction:

| Current wording | Preferred wording |
|---|---|
| emergency viewer | incident viewer |
| emergency token | incident token |
| emergency-token route | incident-token route |
| emergency token TTL | incident token TTL |
| emergency page/data/bundle handler names | incident viewer page/data/bundle handler names |

Keep security-sensitive meanings intact:

- tokens remain bearer credentials and must be treated as secrets
- token hashes remain stored instead of raw tokens
- expired, revoked, and invalid tokens should still collapse to the same public error unless the task explicitly changes behavior
- public viewer routes remain read-only
- private `/v1` route separation remains unchanged
- no raw tokens, request bodies, uploaded bytes, plaintext, raw keys, or Authorization headers should be logged

## Documentation expectations

Update current source-of-truth docs to reflect canonical incident terminology, including:

- `README.md`
- `SECURITY.md`
- `AGENTS.md`
- `docs/README.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/code-map.md`
- `docs/configuration.md`
- `docs/deployment.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/simulator.md`
- `server/README.md`
- relevant reusable Codex prompts if they describe current canonical terminology

Do not rewrite historical reports or dated historical work orders just to change old titles or historical wording. If historical docs mention Safety Recorder or emergency terminology for old versions, leave them historical unless they conflict with current guidance.

## Validation

After Go changes:

```bash
cd server
gofmt -w .
go test ./...
go vet ./...
```

Run the simulator smoke test when practical:

```bash
cd server
go run ./cmd/api
```

In another terminal:

```bash
cd server
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

Also run a final terminology review:

```bash
grep -Rni \
  -e "emergency viewer" \
  -e "EmergencyViewer" \
  -e "emergency_viewer" \
  -e "emergency-token" \
  -e "emergency token" \
  -e "EmergencyToken" \
  -e "emergency_token" \
  -e "emergency_tokens" \
  .
```

Any remaining matches must be one of:

- documented legacy compatibility aliases
- historical reports or historical work orders
- old changelog entries
- migration names required for existing deployed databases
- deliberately preserved public route/config aliases

## Output

Summarize:

1. files changed
2. canonical incident terminology changes made
3. legacy compatibility aliases retained or removed
4. route/config/schema compatibility decisions
5. behavior-preservation notes
6. validation commands run
7. remaining `emergency` terminology and why each remaining category is acceptable
8. follow-up work that should become an issue rather than expanding this diff
