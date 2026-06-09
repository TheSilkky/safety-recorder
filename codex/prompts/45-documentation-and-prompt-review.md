# Codex Prompt: Documentation And Prompt Review

Use very-high reasoning.

Review Proofline Server documentation and reusable Codex prompts for accuracy,
consistency, safety, and maintainability. Default mode is review-only: inspect
files and produce a report without editing files.

Use edit mode only when the maintainer explicitly requests it. In edit mode,
make small, scoped documentation or prompt edits after producing or following
review findings. Do not rewrite large sections without need. Do not perform
unrelated implementation work, add features, or create public GitHub issues.
Backlog findings should become local draft Markdown only if a backlog
prompt/workflow is explicitly requested.

## Source Files To Inspect

Start with current source-of-truth files:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `CHANGELOG.md`, if present
- `docs/README.md`
- `docs/v1-preview-direction.md`
- every current source-of-truth file under `docs/`
- `codex/README.md`
- every reusable prompt under `codex/prompts/`, including this prompt
- `docs/reports/` and report prompts, if present; treat dated published
  reports as historical reviewed-commit artifacts unless they conflict with
  current guidance

Use implementation files only as needed to verify documentation claims:

- `cmd/`
- `internal/`
- `migrations/` docs/comments, if relevant
- tests

Do not rely on stale assumptions from this prompt when current docs or source
code disagree.

## Review Scope

Review:

- all documentation
- all README, AGENTS, SECURITY, and CHANGELOG files
- all docs directories
- all reusable Codex prompt files
- all public-facing project claims
- source-of-truth alignment
- technical accuracy
- linguistic coherence
- readability and approachability
- missing, stale, or contradictory documentation
- overbroad or stale Codex prompt instructions
- validation instructions and safety/security wording

## Server Boundaries

Preserve these server-specific boundaries:

- Keep the backend small, boring, and testable.
- Preserve the main `/v1` and private `/admin` listener separation.
- Do not route `/admin`, `/v1/admin/...`, private diagnostics, raw backend
  errors, or operator-only routes from public edges.
- Do not add backend decryption, browser decryption, raw server-held keys, key
  escrow, break-glass access, playable export, recording/capture,
  trusted-contact decryption, OAuth/JWT, public admin dashboards,
  notifications, emergency-services integration, or key-sharing behavior unless
  explicitly scoped, documented, threat-modeled, and tested.
- Keep implemented behavior, partial/experimental behavior, and planned future
  behavior clearly separated.
- Do not describe current encrypted evidence bundles as decrypted or playable
  media exports.
- Do not imply the backend is production-ready public emergency
  infrastructure.

## Review Checks

Check source-of-truth consistency:

- Do docs agree with current `README.md`, `AGENTS.md`, `SECURITY.md`, and
  `docs/`?
- Do Codex prompts agree with current repo rules?
- Are public claims supported by implementation or source docs?

Check technical accuracy:

- Are API, architecture, route, deployment, auth, storage, security,
  encryption, key-custody, and validation claims accurate?
- Are planned/future features clearly separated from implemented behavior?
- Are deprecated or legacy names explained rather than silently mixed?
- Are `/v1`, `/v1/admin/...`, `/admin`, public viewer, bundle, deletion,
  retention, and account/session boundaries current?

Check documentation completeness:

- Are important setup, development, simulator, validation, release,
  deployment, security, and contribution workflows documented?
- Are missing docs or stale sections identified?
- Are docs/report workflows and public technical review workflows represented
  accurately?

Check prompt quality:

- Are Codex prompts scoped?
- Do prompts tell Codex to inspect current source-of-truth files?
- Do prompts include allowed edit paths and non-goals where needed?
- Do prompts include current validation commands?
- Do prompts avoid stale assumptions?
- Do prompts avoid creating public issues or exposing sensitive material by
  default?
- Do prompts fit the naming and order conventions in `codex/README.md`?

Check readability and approachability:

- Are docs readable to a new contributor or reviewer?
- Are public-facing docs understandable without internal context?
- Are technical docs precise without being needlessly dense?
- Is wording direct, humane, and clear?
- Are acronyms and project-specific terms explained where needed?
- Are there sections that sound like internal notes, legal fog, or startup
  hype?

Check safety and security wording:

- Are emergency-service limitations clear?
- Are production-readiness limitations clear?
- Are reporting instructions safe?
- Are sensitive-data warnings present where needed?
- Are key custody/decryption boundaries accurate and not accidentally
  absolutist where future design remains open?

Check validation and maintenance:

- Are validation commands current?
- Are CI, docs, and check instructions aligned with module reality?
- Are recurring workflows represented by reusable prompts?
- Are one-off or historical prompts clearly separate from reusable prompts?

## Sensitive-Data Rules

Do not include raw sensitive material in public artifacts. Never include:

- raw tokens
- session tokens
- incident/viewer tokens
- Authorization headers
- request bodies
- uploaded bytes
- plaintext
- raw keys
- raw media keys
- contact private keys
- unwrapped secrets
- wrapped-key ciphertext
- verification credentials
- private deployment details
- exploit payloads or proof-of-concept details
- object-store credentials
- stored paths
- object keys
- user safety data

If sensitive material is discovered, describe only the category and affected
file path, not the secret value.

## Review-Only Mode

Review-only mode is the default.

In review-only mode:

- do not modify files
- report findings with file paths and section names
- do not create issues, commits, branches, or pull requests
- do not run full application test suites unless the maintainer requests them
- recommend validation commands instead of claiming they passed

## Edit Mode

Use edit mode only when the maintainer explicitly requests edits.

In edit mode:

- keep edits limited to documentation and Codex prompts unless separately
  scoped
- do not rewrite large sections when targeted edits are enough
- do not perform unrelated implementation work
- do not add features
- preserve all server boundaries above
- update validation instructions when workflow reality changes
- summarize what changed after edits

For docs-only edits, run:

```bash
git diff --check
```

and any documented Markdown/report validation that exists and applies.

If code or migration files are changed because the maintainer explicitly scoped
that work, run:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

Do not claim validation passed unless commands actually ran.

## Required Report

Produce a structured report with:

1. Executive summary.
2. Source files inspected.
3. Documentation consistency findings.
4. Technical accuracy findings.
5. Readability and approachability findings.
6. Codex prompt findings.
7. Missing documentation/gaps.
8. Safety/security wording findings.
9. Recommended edits, grouped by priority:
   - High
   - Medium
   - Low
10. Suggested follow-up backlog draft titles, if any.
11. Validation commands recommended or run.
12. Clear statement whether files were changed.

Findings must include file paths and section names. If edit mode is used,
summarize what changed.
