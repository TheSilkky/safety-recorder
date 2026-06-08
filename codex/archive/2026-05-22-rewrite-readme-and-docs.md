# Codex Prompt: Rewrite README and Markdown Docs for GitHub-Ready Project Presentation

This prompt is historical/reference-only. Do not re-run it without checking it
against the current `README.md`, `AGENTS.md`, `SECURITY.md`, docs, and reusable
prompts.

Rewrite the repository Markdown documentation to make the project look polished, readable, and GitHub-friendly while preserving technical accuracy.

This is a documentation-only task.

Do **not** change Go code.
Do **not** change application behaviour.
Do **not** change workflows, Dockerfiles, database migrations, generated assets, or tests.
Do **not** add React, Node, npm, GitHub Pages, MkDocs, Docusaurus, Docker Compose, Kubernetes, cloud deployment files, OAuth, JWT, user accounts, SMS, Messenger, push notifications, or public admin dashboard features.

## Goal

Make the top-level `README.md` a professional open-source-style landing page.

Move detailed operational/API/development content into normal repository Markdown docs under `docs/`.

The README should be visually clearer, better structured, and badge-friendly, while clearly stating:

- this is experimental
- this is not production-ready public infrastructure
- the repository currently contains the Go backend only
- the iOS client does not exist yet
- evidence bundles are encrypted chunk bundles, not decrypted/playable media exports

## Source of truth

Use the current repository files as source of truth, especially:

- `README.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `LICENSE`
- `AGENTS.md`
- `docs/api.md`
- `docs/threat-model.md`
- `docs/code-map.md`
- `server/go.mod`
- `.github/workflows/ci.yml`

Do not invent features.

Do not claim the project is production-ready.

Do not claim the backend performs client-side encryption, decryption, recording, playable media export, key sharing, SMS, Messenger, push notifications, or iOS functionality.

## Current project facts to preserve

Safety Recorder currently includes:

- Go backend only
- private `/v1` write/admin API listener group
- public read-only emergency viewer listener group
- SQLite metadata
- local disk encrypted chunk storage
- immutable chunk uploads
- media streams that can be marked `open`, `complete`, or `failed`
- completed encrypted stream and incident ZIP evidence bundle downloads
- emergency viewer tokens
- simulator CLI for incident upload/check-in flows
- Docker image build
- GitHub Actions / GHCR publishing
- AGPL-3.0-only license
- repository security policy

Evidence bundles are ZIP files containing encrypted chunks plus JSON manifests.

Evidence bundles are not decrypted, playable, or merged media exports.

The backend does not currently implement:

- iOS app
- recording
- client-side encryption
- decryption
- playable media export
- emergency contact key sharing
- push notifications
- SMS
- Messenger integration
- user accounts
- public admin dashboard

## Badge requirements

Add a badge block near the top of `README.md`.

Use badges for:

- CI passing
- Docker/GHCR image
- latest release
- license AGPL-3.0
- Go version
- repo status
- security policy

Prefer accurate badges over decorative nonsense.

Suggested badge examples:

```md
[![CI](https://github.com/TheSilkky/safety-recorder/actions/workflows/ci.yml/badge.svg)](https://github.com/TheSilkky/safety-recorder/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/TheSilkky/safety-recorder?sort=semver)](https://github.com/TheSilkky/safety-recorder/releases)
[![License: AGPL-3.0-only](https://img.shields.io/badge/license-AGPL--3.0--only-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/TheSilkky/safety-recorder?filename=server%2Fgo.mod)](server/go.mod)
[![Status: Experimental](https://img.shields.io/badge/status-experimental-orange.svg)](#security-warning)
[![Security Policy](https://img.shields.io/badge/security-policy-blue.svg)](SECURITY.md)
[![GHCR](https://img.shields.io/badge/GHCR-ghcr.io%2Fthesilkky%2Fsafety--recorder-blue?logo=github)](https://github.com/TheSilkky/safety-recorder/pkgs/container/safety-recorder)
```

If any badge URL is wrong because of repository casing, workflow filename, or package path, correct it.

Do not add badges for tools/services that are not actually configured, such as Codecov, Go Report Card, Dependabot, OpenSSF Scorecard, or docs deployment, unless the repo already has them.

## README target structure

Rewrite `README.md` into a concise landing page.

Suggested structure:

```md
# Safety Recorder

Badge block.

Short one-paragraph description.

> Security / maturity warning block.

## What it is

Brief explanation.

## What works today

Short bullet list.

## What it is not yet

Short bullet list.

## Architecture

Short explanation plus Mermaid diagrams.

## Quick start

Minimal local run and simulator flow.

## Docker

Minimal Docker run example.

## Documentation

Link to detailed docs.

## Security

Short warning and link to `SECURITY.md` and security model docs.

## Roadmap

Short high-level roadmap.

## License
```

Keep the README readable. Avoid turning it into a giant wall of curl commands.

## Move detailed content into docs/

Keep docs as normal repo Markdown under `docs/`.

Do not set up GitHub Pages, Wiki, MkDocs, Docusaurus, or any docs generator.

You may add or update Markdown files under `docs/`.

Recommended docs structure:

```text
docs/
  README.md
  getting-started.md
  architecture.md
  configuration.md
  api.md
  deployment.md
  security-model.md
  threat-model.md
  simulator.md
  development.md
  code-map.md
```

If existing docs already cover a topic, update them rather than creating duplicates.

Do not delete useful existing docs.

If moving detailed README content into docs, preserve the information accurately.

## Diagrams

Add Mermaid diagrams that render in GitHub Markdown.

Include detailed but readable diagrams for:

1. High-level system architecture
2. Example network topology
3. Incident data flow
4. Private/public server boundary

Example network topology should show something like:

```text
iOS app
  -> WireGuard/private network
  -> private API listener
  -> SQLite + blob storage

trusted contact
  -> HTTPS/reverse proxy
  -> public emergency viewer listener
  -> token-scoped read-only access
```

Make diagrams accurate to the current project.

Do not show features that do not exist yet as implemented.

If showing planned iOS/WireGuard/decryption pieces, label them clearly as planned/future.

## Docs index

Create or update `docs/README.md` as a docs index.

It should link to:

- getting started
- architecture
- configuration
- API
- deployment
- security model
- threat model
- simulator
- development
- code map

## Security wording

Preserve strong warnings.

Make clear that:

- separate bind addresses are a deployment boundary, not a complete security model
- private `/v1` API has no public user authentication
- private `/v1` must stay behind localhost/LAN/WireGuard/firewall/strict reverse proxy
- public emergency viewer links are token-scoped and read-only
- token URLs are secrets
- public deployment still needs TLS, rate limiting, log review, retention policy, proxy hardening, and operational testing
- this project is experimental and not production-ready public infrastructure

## Style requirements

Use a professional open-source tone.

Keep language clear and direct.

Use tables where they improve readability.

Use callout-style blockquotes for warnings.

Use concise sections.

Do not add jokes or informal commentary.

Do not over-market the project.

Do not claim legal, medical, personal-safety, or production guarantees.

## Markdown-only constraint

Only modify Markdown files.

Allowed:

- `README.md`
- `docs/**/*.md`
- `CHANGELOG.md`, if needed
- `SECURITY.md`, only if links/wording need small consistency updates
- `AGENTS.md`, only if docs workflow guidance needs a small update
- `codex/**/*.md`, only if prompt/docs index references need small consistency updates

Not allowed:

- Go files
- SQL migrations
- Dockerfile
- GitHub Actions workflows
- package metadata
- generated binaries
- database files
- uploaded blob data
- non-Markdown files

## Validation

After changes:

```bash
git diff --stat
git diff -- README.md docs
```

If only Markdown files changed, Go tests are not required.

If any non-Markdown files changed, stop and explain why before proceeding.

Check:

- all README links are relative and valid where practical
- badge links use correct repository casing and paths
- docs index links point to files that exist
- Mermaid diagrams are fenced as `mermaid`
- README remains concise
- detailed curl/API content lives in docs, not mostly in README
- no production-readiness claims were added
- no unimplemented features are described as implemented

## Output after implementation

Summarize:

1. files changed
2. new docs files added
3. README structure changes
4. badges added
5. diagrams added
6. detailed content moved from README to docs
7. any links or badges that should be manually checked on GitHub
8. whether any non-Markdown files were changed
