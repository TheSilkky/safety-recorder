# Codex Prompt: Validate Deep Research Technical Review Report

Validate, clean, and public-harden a Deep Research technical review report for this repository.

This is the Phase 2 workflow after a source-cited Deep Research draft. Phase 1 produces a broad report and portable source registry. Phase 2 checks the report against the reviewed repository commit, converts citations into public-safe Markdown, separates future design from implemented behavior, and scopes any generated draft issues to the current branch.

## Inputs

Repository:

```text
open-proofline/server
```

Reviewed branch or ref:

```text
<REVIEWED_BRANCH_OR_REF>
```

Reviewed commit SHA:

```text
<REVIEWED_COMMIT_SHA>
```

Report path:

```text
<REPORT_PATH>
```

Target release / version:

```text
<TARGET_RELEASE_OR_VERSION>
```

Output report path:

```text
docs/reports/<YYYY-MM-DD>-proofline-<TARGET_RELEASE_OR_VERSION>-technical-review.md
```

Issue handling mode:

```text
drafts_only
```

Allowed values:

- `drafts_only`: create or update local branch-scoped issue drafts only
- `create_issues`: create GitHub issues only when the maintainer explicitly requested it
- `none`: do not create issue drafts or GitHub issues

## Product Context

Product documentation now uses the name Proofline. Repository paths, the Go module path, Docker image names, GHCR package names, release binary names, runtime protocol identifiers, and default data-layout identifiers use the `open-proofline/server` repository namespace and Proofline names. Historical reports, archived prompts, legacy `/e/{token}` aliases, and historical migration names may still mention `safety-recorder` or `emergency`.

Proofline's planned product scope includes emergency incidents, non-emergency interaction records, timed safety checks, and evidence notes. The current backend stores generic incidents by default; optional incident-mode, capture-profile, escalation-policy, and sharing-state metadata are labels only unless the reviewed tree explicitly implements first-class behavior for them. The post-quantum envelope is a v1 preview requirement, but it is not current runtime behavior unless implementation files prove it. Any report claim that Proofline is ready for `v1 preview`, `v1.0.0`, or real-user evidence upload must be checked against `docs/v1-preview-readiness-checklist.md`.

## Rules

- Use the current checked-out branch.
- Pin repository citations and report metadata to `<REVIEWED_COMMIT_SHA>`, not to a moving branch name.
- Keep changes scoped to report validation, citation cleanup, and branch-scoped draft issues if requested.
- Do not change application code, CI behavior, repository settings, or GitHub issues unless explicitly requested.
- Keep the report and any issue drafts public-safe according to `SECURITY.md`.
- Do not weaken security warnings.
- Do not claim production readiness, platform-store approval, legal review, compliance certification, penetration test, or formal audit.
- Do not treat absence of future-design features as a defect when source-of-truth docs mark them out of scope.
- Preserve the current backend ciphertext-only implementation boundary unless the report identifies implemented behavior that contradicts it.
- Treat future incident-mode, web/iOS/Android client, key-custody, browser-decryption, break-glass, and post-quantum envelope documents as planning or future preview requirements unless implementation files exist in the reviewed tree.

## First Steps

Check repository state:

```bash
git status --short --branch --untracked-files=all
git branch --show-current
git rev-parse HEAD
git log --oneline -5
```

Read source-of-truth docs:

```bash
sed -n '1,240p' README.md
sed -n '1,220p' SECURITY.md
sed -n '1,260p' AGENTS.md
sed -n '1,280p' docs/README.md
sed -n '1,700p' docs/v1-preview-direction.md
test -f docs/v1-preview-readiness-checklist.md && sed -n '1,260p' docs/v1-preview-readiness-checklist.md
sed -n '1,260p' docs/incident-modes.md
sed -n '1,320p' docs/security-model.md
sed -n '1,340p' docs/threat-model.md
sed -n '1,360p' docs/api.md
sed -n '1,340p' docs/encryption.md
sed -n '1,360p' docs/deployment.md
```

Read future-design docs when present:

```bash
test -f docs/key-custody.md && sed -n '1,360p' docs/key-custody.md
test -f docs/post-quantum-envelope.md && sed -n '1,720p' docs/post-quantum-envelope.md
test -f docs/browser-decryption.md && sed -n '1,360p' docs/browser-decryption.md
test -f docs/break-glass-key-access.md && sed -n '1,360p' docs/break-glass-key-access.md
test -f docs/ios-local-recorder-prototype.md && sed -n '1,360p' docs/ios-local-recorder-prototype.md
```

Read the report:

```bash
sed -n '1,420p' <REPORT_PATH>
```

Before editing, summarize:

1. reviewed branch/ref and reviewed commit SHA
2. current branch and current `HEAD`
3. target release/version
4. whether citations are portable and commit-pinned
5. whether a source registry exists and supports material claims
6. whether future-planning docs are separated from implemented behavior
7. whether Proofline naming and compatibility-name notes are represented correctly
8. whether incident-mode planning is represented as planning unless implemented
9. whether issue drafts should be created and what branch scope they should use
10. likely files to update and docs-review checks

## Branch-Scoped Issue Drafts

When `Issue handling mode` is `drafts_only`, issue drafts must be scoped to the current branch.

Drafts belong under:

```text
.backlog-drafts/<YYYY-MM-DD>/<branch-slug>/
```

If the date is unavailable:

```text
.backlog-drafts/current/<branch-slug>/
```

Every public issue draft should include priority, type, labels, branch scope, summary, context, proposed change, acceptance criteria, tests/validation, out-of-scope notes, and report reference notes.

Use only existing labels. If a good topic label does not exist, use the closest existing label and note the mismatch.

## Report Validation Checklist

Check and fix, if needed:

- Product name is Proofline where describing current docs/product direction.
- Compatibility names remain when describing current artifacts, APIs, routes, config, or packages.
- Repository facts are pinned to `<REVIEWED_COMMIT_SHA>`.
- Mode-driven behavior is marked as planning unless implemented. Optional
  incident-mode, capture-profile, escalation-policy, and sharing-state metadata
  may be described as implemented labels only when the reviewed tree supports
  them.
- Current `/v1` private boundary and public incident-viewer separation remain clear.
- Current backend ciphertext-only behavior is represented accurately.
- V1 preview, v1.0.0, and real-user evidence-upload readiness claims are
  checked against `docs/v1-preview-readiness-checklist.md`; if any hard
  blocker remains incomplete, the report must use pre-v1 or experimental
  language instead of preview-ready language.
- Historical report names are not rewritten as if they used the new product name at the time.
- ChatGPT internal citation tokens are removed or converted to portable citation keys.
- Remove informal or conversational draft language, including humour, mascot references, assistant/meta commentary, and chat-only tone, unless it is quoted as reviewed evidence.
- Source Registry entries support material claims.
- External-source omissions are disclosed and affected claims are marked not independently verified.
- The report does not invent a no-network, no-web, or no-external-source constraint when Phase 1 only restricted claims about executing local commands, tests, builds, containers, or simulator smoke tests.
- Missing external sources are treated as verification limitations, not as supporting evidence for findings.

## Common False Positives To Remove Or Downgrade

- Missing public product API authentication when docs state `/v1` is private
  and protected by local account sessions.
- Missing web/iOS/Android clients when docs mark them as future work.
- Missing first-class incident-mode behavior, capture-profile behavior,
  escalation policies, sharing-state behavior, or dead-man switch when docs mark
  them as future work.
- Missing browser decryption, production key custody, or break-glass behavior when docs mark them as future work.
- Preserved protocol, data-layout, route-alias, or migration compatibility names treated as stale after the repository/module/artifact rename.
- Interaction-record planning treated as current implementation.
- Backend decryption or server-held keys assumed from future design docs.
- V1 preview readiness claimed only because ordinary tests passed, without
  checking the v1 preview readiness checklist hard blockers.
- Wording that says review constraints prohibited network calls, web access, or external source consultation unless the maintainer explicitly imposed that constraint.
- Findings supported by the absence of external sources instead of by repository evidence plus authoritative sources.

## Validation

For report/docs-only cleanup:

```bash
git diff --stat
git diff --check
```

If Go code changed unexpectedly, stop and report scope creep.

## Output

Summarize:

1. current branch
2. reviewed branch/ref and commit SHA
3. report path updated
4. issue drafts created, if any
5. citation and public-safety cleanup performed
6. Proofline naming and incident-mode boundary corrections made
7. validation/docs-review commands run
8. follow-up work
