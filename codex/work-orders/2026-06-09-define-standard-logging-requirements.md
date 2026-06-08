# Historical Work Order: Define Standard Logging Requirements

This is a one-off Codex work order for `open-proofline/server`.

Date: 2026-06-09
Branch: `codex/logging-work-orders-2026-06-09`
Base branch: `develop`
Base commit: `892eccc097f251f2d540550addabb1c40efa4dad`

## Goal

Create a strict, helpful, and safe logging requirements document for the server repository.

This work order defines logging policy and documentation only. It must not audit or change runtime logging implementation beyond small documentation references. A separate implementation work order should apply these requirements to code.

## Reasoning

Use very-high reasoning.

Logging in Proofline is security-sensitive because startup errors, request handling, storage, object deletion, configuration, account/session flows, upload handling, token handling, and future contact/key flows can easily expose sensitive data if logs become too helpful in the wrong way.

The requirements document should make logs useful for maintainers while preserving Proofline's strict sensitive-data boundaries.

## Source of truth to inspect

Before editing, inspect:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `CHANGELOG.md`
- `docs/README.md`
- `docs/development.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/deployment.md`
- `docs/configuration.md`
- `docs/code-map.md`
- `cmd/api/main.go`
- `cmd/api/logging.go`
- `cmd/api/main_test.go`
- `internal/config/`
- `internal/httpapi/`
- `internal/storage/`
- `internal/coordination/`
- `internal/email/`
- `internal/retention/`
- reusable prompts under `codex/prompts/`, only for references that need to point to the new requirements document

Do not rely on stale assumptions from this work order if current repository files disagree.

## Scoped changes

### 1. Create logging requirements documentation

Create:

```text
docs/logging-requirements.md
```

The document should define the server's required logging posture. It should be clear enough that future code reviews, security reviews, and Codex work can evaluate whether logging changes are safe and useful.

Required sections:

1. Summary
2. Goals
3. Non-goals
4. Standard structured log fields
5. Startup logging requirements
6. Configuration error logging requirements
7. Request and handler logging requirements
8. Operator command logging requirements
9. Background worker logging requirements
10. Rate-limit, auth, token, upload, deletion, and storage logging rules
11. Error category taxonomy
12. Safe detail taxonomy
13. Sensitive data that must never be logged
14. Filesystem path, object key, and private deployment detail redaction rules
15. When raw `err.Error()` is forbidden
16. When config key names may be logged
17. When supported values may be logged
18. Test requirements
19. Review checklist
20. Open questions / future work

### 2. Define standard log fields

The document should define common structured fields such as:

- `component`
- `operation`
- `startup_stage`, for startup-only logs
- `listener`, for main/admin listener context where safe
- `route_class`, for handler/rate-limit class context where safe
- `error_category`
- `safe_error_detail`
- `config_key`, only when safe
- `backend`, only when safe
- `status`, for worker/operator summaries
- safe counts such as `eligible`, `removed`, `failed`, or `skipped`

The document should state which fields are required, optional, startup-only, request-only, worker-only, or operator-only.

### 3. Define startup stage names

The document should standardize startup stage names so startup failures point to the failing subsystem without leaking secrets.

Candidate stage names include:

- `args_parse`
- `config_load`
- `config_validate`
- `coordination_init`
- `coordination_check`
- `metadata_open`
- `auth_bootstrap_check`
- `blob_store_open`
- `temp_upload_cleanup`
- `email_sender_init`
- `http_server_config`
- `http_listen`
- `shutdown`

Codex may adjust these names after inspecting current code, but the document should avoid vague stage names that do not help a maintainer diagnose a startup failure.

### 4. Define error category taxonomy

The document should standardize error categories such as:

- `config`
- `unsupported_backend`
- `missing_required_config`
- `invalid_config_value`
- `permission`
- `filesystem`
- `network`
- `timeout`
- `dependency_unavailable`
- `coordination_unavailable`
- `storage`
- `metadata`
- `auth_bootstrap_required`
- `rate_limit_unavailable`
- `shutdown`
- `unknown`

Codex may adjust the list to match current implementation, but the document should require stable, finite, low-cardinality categories.

### 5. Define safe configuration logging rules

The document should distinguish safe non-secret configuration details from unsafe sensitive details.

Safe examples:

```text
startup_stage=config_load
error_category=config
config_key=SAFE_METADATA_BACKEND
safe_error_detail="unsupported backend; supported values: sqlite, postgresql"
```

Unsafe examples:

```text
raw DSNs
secret values
secret file contents
private filesystem paths
object store access keys
raw SMTP passwords
raw bootstrap secrets
raw tokens
wrapped-key ciphertext
```

The document should specify how secret-adjacent config keys should be handled. It should allow a safe category such as `secret_config` or `secret_file_config` without logging the raw secret, secret path, or file content.

### 6. Define raw-error handling rules

The document must state that raw `err.Error()` must not be logged by default for startup, request, upload, storage, token, key, auth, object-store, and user-safety paths.

If raw error strings are ever allowed, the document must require a local justification that the error type is known safe and cannot contain secrets, paths, object keys, request data, plaintext, raw keys, token values, private deployment details, or user safety data.

### 7. Define test requirements

The document should require tests for logging behavior when code changes affect logging.

At minimum, future implementation tests should cover:

- startup logs include useful safe stage/category fields;
- non-secret config errors include safe config key and supported values where appropriate;
- secret-related config errors do not expose raw key names if the name itself is sensitive, raw secret values, secret file contents, or private paths;
- filesystem errors do not expose private paths;
- object-store/storage errors do not expose object keys or credentials;
- request logs do not expose Authorization headers, request bodies, uploaded bytes, raw tokens, plaintext, raw keys, or wrapped-key ciphertext;
- worker/operator logs include useful safe counts without exposing sensitive row data.

### 8. Link or cross-reference the requirements

Update only small documentation references if useful:

- `docs/README.md`
- `docs/development.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/code-map.md`, only if needed
- `codex/README.md`, only if needed

Do not perform the full code audit in this work order.

## Explicit non-goals

Do not:

- change Go runtime code;
- change startup logging implementation;
- change request logging implementation;
- change tests except documentation-only references if absolutely needed;
- add a third-party logging dependency;
- change API behavior;
- change configuration behavior;
- change migrations;
- change storage behavior;
- add backend decryption;
- add browser decryption;
- add key escrow;
- add notification behavior;
- add recording/capture;
- create GitHub issues;
- create or merge pull requests;
- edit sibling repositories.

## Sensitive-data rules

Never include raw tokens, session tokens, incident/viewer tokens, Authorization headers, request bodies, uploaded bytes, plaintext, raw keys, raw media keys, contact private keys, unwrapped secrets, wrapped-key ciphertext, verification credentials, private deployment details, exploit payloads, object-store credentials, stored paths, object keys, private filesystem paths, SMTP credentials, database DSNs, bootstrap secrets, or user safety data in public docs, prompts, examples, logs, tests, or summaries.

Examples in the new document must be synthetic and safe.

## Validation

For docs-only changes, run:

```bash
git diff --check
git diff --stat
git diff -- README.md AGENTS.md SECURITY.md CHANGELOG.md docs codex
```

If any Go code changes unexpectedly, stop and explain why. Do not run Go validation unless code changed or the maintainer explicitly requests it.

## Output

Summarize:

1. files changed;
2. logging requirements document created;
3. standard fields and taxonomies defined;
4. sensitive-data exclusions added;
5. cross-references updated;
6. open questions left in the document;
7. validation commands run and results;
8. confirmation that no runtime code, migrations, API behavior, issues, PRs, or sibling repositories were changed.
