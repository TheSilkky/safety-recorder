# Codex Prompt: Add Codex Change-Control and Issue Backlog Workflow

This prompt is historical/reference-only. Do not re-run it without checking it
against the current `README.md`, `AGENTS.md`, `SECURITY.md`, docs, and reusable
prompts.

Add documentation and reusable templates for two process guardrails:

1. Codex work should start from a clear rollback/checkpoint point.
2. Future work should be tracked as issues/backlog items instead of being implemented immediately during unrelated tasks.

This is a documentation/process task.

Do **not** change Go code.
Do **not** change application behaviour.
Do **not** change database migrations.
Do **not** change Dockerfiles.
Do **not** change GitHub Actions workflows.
Do **not** add dependencies.
Do **not** add React, Node, npm, OAuth, JWT, user accounts, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features.

## Goal

Make the repository safer to work on with Codex by adding a clear maintainer workflow:

- before every Codex task, create or confirm a rollback point
- keep Codex tasks narrow and scoped
- review diffs before accepting changes
- run the correct validation commands
- commit accepted changes deliberately
- create issues/backlog items for future ideas instead of letting tasks expand uncontrollably

This should feel like sysadmin-style change control, not enterprise paperwork sludge.

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
- documented v1 AES-256-GCM simulator encryption envelope
- Docker image build
- GitHub Actions / GHCR publishing
- AGPL-3.0-only license
- repository security policy
- Codex prompt library under `codex/`

Evidence bundles are encrypted chunk bundles, not decrypted/playable media exports.

The project is experimental and not production-ready public infrastructure.

## Files to add or update

Update existing Markdown docs where appropriate:

- `docs/development.md`
- `codex/README.md`
- `AGENTS.md`, only if a short rule update is useful
- `docs/README.md`, only if adding a link to new docs

Add a new Markdown doc if helpful:

```text
docs/codex-change-control.md
```

Add reusable Codex prompts if helpful:

```text
codex/prompts/00-project-context-check.md
codex/prompts/05-codex-change-control.md
```

Add GitHub issue templates as Markdown files if they do not already exist:

```text
.github/ISSUE_TEMPLATE/bug-report.md
.github/ISSUE_TEMPLATE/feature-request.md
.github/ISSUE_TEMPLATE/maintenance-task.md
.github/ISSUE_TEMPLATE/security-hardening.md
```

Use Markdown issue templates only. Do not add YAML issue forms unless explicitly requested.

Do not create actual GitHub issues in this task. Only add templates/documentation.

## Required content: Codex rollback/checkpoint workflow

Add a clear workflow for Codex-assisted changes.

Suggested section title:

```md
## Codex-assisted change workflow
```

Include a pre-task checklist:

```md
### Before running Codex

1. Check the working tree:

   ```bash
   git status
   ```

2. If there are useful uncommitted changes, commit them or intentionally stash them before asking Codex to modify files.

3. Create a rollback point for larger tasks:

   ```bash
   git add .
   git commit -m "checkpoint before <task>"
   ```

4. Read the current source-of-truth docs:
   - `README.md`
   - `AGENTS.md`
   - relevant files in `docs/`
   - relevant prompt in `codex/prompts/`

5. Define:
   - goal
   - files likely affected
   - files that must not change
   - tests/validation commands
   - explicit out-of-scope items
```

Include a post-task checklist:

```md
### After Codex makes changes

1. Review the changed files:

   ```bash
   git diff --stat
   git diff
   ```

2. If code changed, run:

   ```bash
   cd server
   gofmt -w .
   go test ./...
   go vet ./...
   ```

3. If only Markdown changed, inspect docs and links manually. Go tests are not required unless code changed.

4. For behaviour changes, run the simulator smoke test:

   ```bash
   cd server
   go run ./cmd/api
   ```

   In another terminal:

   ```bash
   cd server
   go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
   ```

5. If the change is good, commit it deliberately.

6. If the change is bad, revert or reset to the checkpoint. Do not try to rescue a sprawling bad diff by asking Codex for more sprawling changes.
```

Include rollback guidance:

```md
### Rollback

For an uncommitted bad change:

```bash
git restore .
```

For a bad change after a checkpoint commit, inspect history first:

```bash
git log --oneline -5
```

Then either revert with a new commit:

```bash
git revert <commit>
```

or reset only if you understand the consequence and have not pushed:

```bash
git reset --hard <checkpoint>
```

Prefer `git revert` for pushed history.
```

Keep this practical and not overbearing.

## Required content: issue/backlog workflow

Add guidance that new ideas discovered during Codex work should become issues or backlog items unless they are directly in scope.

Suggested section title:

```md
## Issue-first future work
```

Include rules:

- If Codex or the maintainer discovers a new idea during a task, do not implement it unless it is necessary for the current task.
- Create an issue/backlog item instead.
- Issues should include context, acceptance criteria, tests, docs updates, and explicit out-of-scope notes.
- Security vulnerabilities should follow `SECURITY.md`, not public issue templates.
- Security hardening work that is not a private vulnerability can use a normal issue template.

Suggested issue format:

```md
## Summary

One or two sentences.

## Context

Why this matters.

## Proposed change

What should change.

## Acceptance criteria

- [ ] ...
- [ ] ...
- [ ] ...

## Tests / validation

- [ ] `go test ./...`
- [ ] simulator smoke test, if relevant
- [ ] docs updated, if relevant

## Out of scope

What this issue should not include.
```

## GitHub issue templates

If adding templates, make them concise and project-specific.

### bug-report.md

Include:

- summary
- affected version/commit
- expected behaviour
- actual behaviour
- reproduction steps
- logs/output
- tests tried
- environment
- whether this might be security-sensitive

Warn that security vulnerabilities should not be reported publicly and should follow `SECURITY.md`.

### feature-request.md

Include:

- summary
- problem / motivation
- proposed solution
- alternatives
- acceptance criteria
- out of scope
- docs/tests impact

### maintenance-task.md

Include:

- summary
- maintenance category
- affected files/areas
- reason
- acceptance criteria
- validation commands
- out of scope

### security-hardening.md

This is for non-sensitive hardening tasks, not private vulnerability reports.

Include:

- summary
- hardening area
- current risk/limitation
- proposed control
- acceptance criteria
- tests/docs
- public-safety warning: do not include exploit details, raw tokens, secrets, private deployment info, or user safety data

## Reusable Codex prompt: project context check

If adding `codex/prompts/00-project-context-check.md`, make it instruct Codex to:

- read `README.md`, `AGENTS.md`, and relevant docs
- summarize current project scope
- summarize current backend surfaces
- summarize current security boundaries
- identify likely affected files
- identify out-of-scope items
- make no changes yet

## Reusable Codex prompt: change-control check

If adding `codex/prompts/05-codex-change-control.md`, make it instruct Codex to:

- check whether the task has a clear goal
- check whether the prompt defines files allowed/disallowed
- check whether validation commands are defined
- check whether the task should instead create an issue/backlog item
- make no code changes
- output a short readiness assessment

## Update AGENTS.md

If updating `AGENTS.md`, keep it short.

Add rules like:

```md
- Treat Codex prompts as scoped change requests, not open-ended permission to expand the project.
- Do not implement newly discovered future work during an unrelated task; document it as an issue/backlog item instead.
- For larger changes, start from a clean working tree or an explicit checkpoint commit.
```

Do not turn `AGENTS.md` into a giant manual.

## Constraints

Only Markdown files may be changed.

Allowed:

- `docs/**/*.md`
- `codex/**/*.md`
- `AGENTS.md`
- `.github/ISSUE_TEMPLATE/*.md`

Not allowed:

- Go files
- SQL files
- Dockerfiles
- GitHub Actions workflows
- non-Markdown issue forms
- generated artifacts
- binaries
- database files
- uploaded blob data

## Validation

After changes:

```bash
git diff --stat
git diff -- docs codex AGENTS.md .github/ISSUE_TEMPLATE
```

If only Markdown files changed, Go tests are not required.

If any non-Markdown files changed, stop and explain why.

Check:

- docs links are valid where practical
- issue templates mention `SECURITY.md` for vulnerabilities
- the workflow does not encourage destructive git commands without warnings
- the workflow does not imply Codex can approve its own changes
- the docs remain concise and usable
- no production-readiness claims were added

## Output after implementation

Summarize:

1. files changed
2. new docs added
3. issue templates added
4. Codex prompts added/updated
5. AGENTS.md changes, if any
6. whether only Markdown files changed
7. whether tests were skipped as docs-only
8. any follow-up recommendations
