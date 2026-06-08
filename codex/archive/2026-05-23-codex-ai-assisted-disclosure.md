# Codex Prompt: Add AI-Assisted Development Disclosure

This prompt is historical/reference-only. Do not re-run it without checking it
against the current `README.md`, `AGENTS.md`, `SECURITY.md`, docs, and reusable
prompts.

Add a concise AI-assisted development disclosure to the repository documentation.

This is a Markdown-only task.

Do **not** change Go code.
Do **not** change application behaviour.
Do **not** change workflows, Dockerfiles, database migrations, generated assets, or tests.
Do **not** add React, Node, npm, OAuth, JWT, user accounts, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, public admin dashboard features, or unrelated project policy documents.

## Goal

Add a transparent disclosure that this project has been developed with substantial assistance from OpenAI Codex.

The disclosure should make clear:

- Codex was used for code and documentation assistance
- Codex was used to draft, refactor, test, document, and review parts of the repository
- accepted changes are reviewed, tested, and committed by the maintainer
- the maintainer remains responsible for correctness, security, licensing, releases, deployment choices, and real-world use
- Codex use does not imply endorsement, audit, certification, or maintenance by OpenAI

Keep the disclosure professional, concise, and non-apologetic.

## Project context

Safety Recorder is an experimental Go backend for a private personal-safety recording system.

The current project includes:

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
- AGPL-3.0-only license
- repository security policy

The project is experimental and not production-ready public infrastructure.

Do not weaken existing security warnings.

## Files to update

Update:

- `README.md`
- `docs/development.md`, if it exists
- `docs/README.md`, only if it has a relevant development/documentation section

Optional:

- `codex/README.md`, only if it helps explain how Codex prompts are used in the repo

Do not create a large new policy document unless clearly necessary.

## Suggested README section

Add a section similar to this:

```md
## AI-assisted development

This project has been developed with substantial assistance from OpenAI Codex.

Codex has been used to draft, refactor, test, document, and review parts of the Go backend and Markdown documentation. All accepted changes are reviewed, tested, and committed by the maintainer.

AI assistance does not replace human responsibility. The maintainer remains responsible for:

- code correctness
- security review
- licensing decisions
- release decisions
- deployment choices
- any real-world use of the software

Use of Codex does not imply endorsement, audit, certification, or maintenance by OpenAI.
```

You may lightly adjust the wording to fit the current README style, but preserve the meaning.

## Suggested docs/development.md wording

If `docs/development.md` exists, add a shorter note such as:

```md
## AI assistance

This repository uses OpenAI Codex as an AI-assisted development tool. Codex may generate or modify code and documentation, but changes are accepted only after maintainer review and testing.

The maintainer remains responsible for correctness, security, licensing, releases, deployment decisions, and real-world use.
```

## Constraints

- Keep wording professional and concise.
- Do not over-explain.
- Do not add jokes.
- Do not make legal claims.
- Do not imply OpenAI endorses, audits, certifies, maintains, or secures this project.
- Do not claim the project is production-ready.
- Do not imply Codex replaces human review.
- Do not alter non-Markdown files.
- Do not change project features, routes, APIs, schema, workflows, Docker setup, or tests.

## Validation

After changes:

```bash
git diff --stat
git diff -- README.md docs codex
```

If only Markdown files changed, Go tests are not required.

If any non-Markdown files changed, stop and explain why before proceeding.

## Output after implementation

Summarize:

1. files changed
2. disclosure wording added
3. whether any optional docs were updated
4. whether any non-Markdown files were changed
5. whether tests were skipped as docs-only
