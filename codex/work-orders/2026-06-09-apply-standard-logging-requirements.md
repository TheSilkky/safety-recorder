# Historical Work Order: Apply Standard Logging Requirements

This is a one-off Codex work order for `open-proofline/server`.

Date: 2026-06-09
Branch: `codex/logging-work-orders-2026-06-09`
Base branch: `develop`
Base commit: `892eccc097f251f2d540550addabb1c40efa4dad`

## Goal

Audit and update server logging so current code follows `docs/logging-requirements.md`, then update `codex/prompts/20-code-review.md` so future code review checks logging against that document.

This work order may change Go code, tests, and narrow documentation/prompt references. Keep the implementation small, safe, and reviewable.

## Prerequisite

Run this work order only after the logging requirements document exists and has been reviewed.

Required source document:

```text
docs/logging-requirements.md
```

If `docs/logging-requirements.md` is missing, stop and run the logging requirements work order first. Do not invent logging policy in this implementation work order.

## Reasoning

Use very-high reasoning.

Logging is security-sensitive. The implementation must improve startup/config/operator/debug usefulness without exposing secrets, raw tokens, private deployment details, object keys, filesystem paths, request bodies, uploaded bytes, plaintext, raw keys, or user safety data.

Prefer Go standard-library `log/slog` unless `docs/logging-requirements.md` explicitly justifies a different logging library and the maintainer explicitly confirms that dependency change. Do not add a third-party logging dependency by default.

## Source of truth to inspect

Before editing, inspect:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `CHANGELOG.md`
- `docs/README.md`
- `docs/logging-requirements.md`
- `docs/development.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/deployment.md`
- `docs/configuration.md`
- `docs/code-map.md`
- `cmd/api/main.go`
- `cmd/api/logging.go`
- `cmd/api/main_test.go`
- `cmd/api/operator*.go`, if present
- `internal/config/`
- `internal/httpapi/`
- `internal/storage/`
- `internal/coordination/`
- `internal/email/`
- `internal/retention/`
- existing tests for logging, config parsing, HTTP middleware, upload handling, storage, deletion, and operator commands
- `codex/prompts/20-code-review.md`
- `codex/prompts/30-security-review.md`, only if implementation reveals a security-review logging gap

Do not rely on stale assumptions from this work order if current repository files disagree.

## Scoped changes

### 1. Audit current logging against the requirements

Audit current logging and error handling for compliance with `docs/logging-requirements.md`.

At minimum, inspect:

- startup error logging in `cmd/api/main.go` and `cmd/api/logging.go`;
- configuration loading and parse errors in `internal/config/`;
- metadata backend initialization;
- blob storage initialization;
- coordination backend initialization and checks;
- email sender initialization, if it can fail during startup or send;
- HTTP middleware request logging;
- panic/recovery logging;
- rate-limit logging;
- upload, temp cleanup, deletion, retention worker, and operator command logging.

Do not produce a giant logging refactor. Fix the highest-value gaps first, especially startup/config errors that are currently too generic.

### 2. Improve startup/config error usefulness safely

Implement minimal typed or structured startup error handling so startup failures identify the safe failing stage and category.

Expected direction, adjusted to fit current code:

- add a small typed startup error wrapper if useful;
- include `startup_stage` in startup failure logs when known;
- include `error_category` using a stable low-cardinality taxonomy;
- include `safe_error_detail` only when explicitly safe;
- include `config_key` only when allowed by `docs/logging-requirements.md`;
- keep unsupported backend errors helpful by including supported values;
- keep auth-bootstrap errors helpful without exposing secrets;
- avoid raw `err.Error()` for generic startup errors unless the error type is proven safe.

Do not log raw secret values, secret file contents, private file paths, database DSNs, object-store credentials, object keys, uploaded bytes, request bodies, plaintext, raw keys, or wrapped-key ciphertext.

### 3. Preserve and extend redaction behavior

Existing tests already check that startup logs do not expose private filesystem paths and do not expose secret config parse detail. Preserve that behavior and extend it.

Add or update tests for:

- safe startup stage/category fields;
- safe non-secret config details, including unsupported backend supported values;
- secret-related config errors remaining redacted;
- filesystem errors not exposing private paths;
- coordination/backend initialization errors receiving useful safe categories;
- request/logging paths not including raw Authorization headers, request bodies, uploaded bytes, raw tokens, plaintext, raw keys, or wrapped-key ciphertext where practical.

Tests should assert positive usefulness and negative redaction.

### 4. Keep request and worker logging safe

Do not add broad request-body logging or verbose error dumping.

If request logging is updated, ensure it follows `docs/logging-requirements.md` and uses safe low-cardinality route or component fields. Do not log token-bearing paths in a way that exposes raw token values. Preserve public/private listener separation.

If worker/operator logging is updated, prefer safe counts and controlled statuses rather than raw row values, stored paths, object keys, notes, location values, or user safety data.

### 5. Update code review prompt

Update:

```text
codex/prompts/20-code-review.md
```

so future code reviews check logging against:

```text
docs/logging-requirements.md
```

The prompt should require review of:

- structured logging field consistency;
- startup/config error helpfulness;
- redaction of secrets, tokens, paths, object keys, request bodies, uploaded bytes, plaintext, raw keys, wrapped-key ciphertext, private deployment details, and user safety data;
- no casual raw `err.Error()` logging in sensitive paths;
- tests for changed logging behavior.

Keep the prompt update small and scoped. Do not rewrite the whole prompt unless necessary.

### 6. Update docs only if implementation changes visible behavior

Update documentation only if runtime logging behavior, configuration guidance, or operational troubleshooting changes in a way maintainers should know.

Likely candidates:

- `docs/logging-requirements.md`
- `docs/deployment.md`
- `docs/configuration.md`
- `docs/development.md`
- `docs/code-map.md`
- `CHANGELOG.md`

Do not turn docs into a long troubleshooting novel. The paperwork dragon is strong enough already.

## Explicit non-goals

Do not:

- replace `slog` with another logging library unless the requirements doc explicitly justifies it and the maintainer confirms;
- add broad request-body logging;
- add raw error logging in sensitive paths;
- log secrets, raw tokens, request bodies, uploaded bytes, plaintext, raw keys, wrapped-key ciphertext, private deployment details, stored paths, object keys, database DSNs, SMTP credentials, or user safety data;
- change API behavior;
- change auth/session behavior;
- change upload semantics;
- change storage semantics;
- change migrations unless absolutely required and separately justified;
- add backend decryption;
- add browser decryption;
- add key escrow;
- add notification behavior;
- add recording/capture;
- add paid registration, billing, Stripe, or payment-provider behavior;
- create GitHub issues;
- create or merge pull requests;
- edit sibling repositories.

## Sensitive-data rules

Never include raw tokens, session tokens, incident/viewer tokens, Authorization headers, request bodies, uploaded bytes, plaintext, raw keys, raw media keys, contact private keys, unwrapped secrets, wrapped-key ciphertext, verification credentials, private deployment details, exploit payloads, object-store credentials, stored paths, object keys, private filesystem paths, SMTP credentials, database DSNs, bootstrap secrets, or user safety data in logs, docs, tests, prompts, PR text, issue text, or summaries.

Synthetic test values must not look like real secrets. Avoid using real-looking tokens or private deployment details in test fixtures.

## Validation

If Go code changed, run:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

If only Markdown/prompt files changed, run:

```bash
git diff --check
git diff --stat
git diff -- README.md AGENTS.md SECURITY.md CHANGELOG.md docs codex
```

Do not claim validation passed unless commands actually ran. If any command cannot run, report the reason.

## Output

Summarize:

1. files changed;
2. logging audit findings addressed;
3. startup/config logging changes;
4. redaction guarantees preserved or added;
5. tests added or updated;
6. `20-code-review.md` updates;
7. docs updated;
8. validation commands run and results;
9. remaining logging follow-up work, if any;
10. confirmation that no unrelated runtime behavior, API behavior, migrations, issues, PRs, or sibling repositories were changed.
