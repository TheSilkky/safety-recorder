# Codex Prompt: Add SECURITY.md and AGPL-3.0 License

This prompt is historical/reference-only. Do not re-run it without checking it
against the current `README.md`, `AGENTS.md`, `SECURITY.md`, docs, and reusable
prompts.

Add a repository security policy and GNU AGPLv3 license metadata.

Do not change application behaviour.
Do not modify Go source code unless required only for license metadata comments, and do not add source headers unless explicitly requested.
Do not add dependencies.
Do not add React, Node, npm, OAuth, JWT, user accounts, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features.

## Goal

Add:

- `LICENSE`
- `SECURITY.md`
- README license/security sections
- any small documentation updates needed to make the project licensing and vulnerability reporting clear

The project should be licensed as GNU Affero General Public License v3.0.

Use SPDX identifier:

```text
AGPL-3.0-only
```

Do not use `AGPL-3.0-or-later` unless the repository already clearly says “or later” somewhere.

## Important project context

Safety Recorder is a Go backend for a private personal-safety recording system.

It currently includes:

- private `/v1` write/admin API listener group
- public read-only emergency viewer listener group
- SQLite metadata
- local disk encrypted chunk storage
- immutable chunk uploads
- media streams that can be marked `open`, `complete`, or `failed`
- completed encrypted stream and incident ZIP evidence bundle downloads
- emergency viewer tokens
- simulator CLI
- Docker image build
- GitHub Actions / GHCR publishing

Evidence bundles are encrypted chunk bundles, not decrypted or playable media exports.

## Add LICENSE

Add a top-level file:

```text
LICENSE
```

The license file must contain the exact, unmodified GNU Affero General Public License version 3 text.

Preferred source:

- official GNU AGPLv3 text, or
- GitHub’s standard AGPL-3.0 license template if available locally through GitHub tooling

Do not paraphrase the license.
Do not create a custom license.
Do not add extra terms.

If you cannot reliably obtain the exact AGPLv3 text, stop and explain what is needed instead of inventing license text.

## Add SECURITY.md

Add a top-level file:

```text
SECURITY.md
```

The policy should include:

1. Security Policy title
2. Supported Versions
3. Reporting a Vulnerability
4. Vulnerability handling expectations
5. Security scope
6. Out-of-scope reports
7. Public disclosure guidance

Use current repo version information from `README.md`, `CHANGELOG.md`, and tags if available.

Suggested supported versions:

```md
## Supported Versions

| Version | Supported |
|---|---|
| 0.2.x | Yes |
| < 0.2 | No |
```

If the current version differs, use the current minor version as supported.

## SECURITY.md reporting section

Do not invent a private email address.

Use this wording unless the repository already has a clear security contact:

```md
## Reporting a Vulnerability

Please do not report security vulnerabilities through public GitHub issues.

For now, report vulnerabilities privately to the repository maintainer.

Before this repository is made public or deployed for real-world use, configure one of:

- GitHub private vulnerability reporting, or
- a dedicated security contact email.

Include:

- a description of the vulnerability
- affected version or commit
- steps to reproduce
- expected impact
- any suggested fix, if known
```

If GitHub private vulnerability reporting is already enabled or documented in the repo, reference it instead.

## SECURITY.md scope

Include security areas relevant to this project:

- private `/v1` route exposure
- public emergency viewer read-only access
- emergency token leakage
- raw token logging
- request body logging
- uploaded file byte logging
- Authorization header logging
- upload size limits
- SHA-256 verification
- immutable chunk storage
- media stream completion validation
- ZIP bundle path traversal
- ZIP entry name safety
- filesystem path disclosure
- Docker bind exposure
- reverse proxy/TLS deployment
- evidence retention/deletion policy

Make clear that this project is not production-ready public infrastructure.

## README updates

Update `README.md` with concise sections:

```md
## License

This project is licensed under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). See [LICENSE](LICENSE).

## Security

Please see [SECURITY.md](SECURITY.md) for supported versions and vulnerability reporting guidance.

Do not report security vulnerabilities through public GitHub issues.
```

Preserve the existing README’s security warnings about:

- private `/v1` API exposure
- separate bind addresses being a deployment boundary, not a complete security model
- public deployment requiring TLS, rate limiting, logging review, retention policy, and proxy hardening

## Optional docs updates

If present, update:

- `docs/security-model.md`
- `docs/threat-model.md`
- `CHANGELOG.md`
- `codex/prompts/90-release-check.md`

Only make small, relevant updates.

For `CHANGELOG.md`, add an entry such as:

```md
- Added AGPL-3.0-only license.
- Added repository security policy.
```

For release-check prompt/docs, add checks for:

- `LICENSE` exists
- `SECURITY.md` exists
- README links to both
- security policy does not promise production readiness

## Constraints

Do not:

- change backend routes
- change API behaviour
- change database schema
- change Docker build behaviour
- change GitHub Actions workflows unless required for documentation links
- add source-code license headers unless explicitly requested
- add a Contributor License Agreement
- add a Code of Conduct unless explicitly requested
- add legal advice
- claim the project is production-ready

## Validation

After changes:

```bash
git diff --stat
git diff -- LICENSE SECURITY.md README.md
```

If only docs/license files changed, Go tests are not required.

If any application code changed, run:

```bash
cd server
gofmt -w .
go test ./...
```

## Output after implementation

Summarize:

1. files added
2. files updated
3. selected SPDX identifier
4. supported versions listed in `SECURITY.md`
5. reporting path used in `SECURITY.md`
6. whether any placeholders remain
7. whether tests were run or not needed
