# Codex Prompt: Validate Technical Review Report

Validate, clean, and public-harden a Codex technical review report for this repository.

This is an independent Phase 2 workflow after a source-cited Codex Phase 1
draft. Phase 1 produces a broad report and portable source registry and may
directly execute validation commands when it records exact evidence. Phase 2
independently checks the draft against the reviewed repository commit,
converts citations into public-safe Markdown, separates future design from
implemented behavior, and scopes any generated draft issues to the current
branch. Do not accept Phase 1 execution claims without checking the recorded
command, context, and result.

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
.technical-review-drafts/<YYYY-MM-DD>-proofline-<TARGET_RELEASE_OR_VERSION>-technical-review-draft.md
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
- `create_issues`: create sanitized non-vulnerability GitHub issues only when
  the maintainer explicitly requested it
- `none`: do not create issue drafts or GitHub issues

## Product Context

Product documentation now uses the name Proofline. Repository paths, the Go module path, Docker image names, GHCR package names, release binary names, runtime protocol identifiers, and default data-layout identifiers use the `open-proofline/server` repository namespace and Proofline names. Historical reports, archived prompts, legacy `/e/{token}` aliases, and historical migration names may still mention `safety-recorder` or `emergency`.

The website repository is the project-level source of truth for public
governance posture, political alignment, public-good framing, public voice,
reusable README baseline guidance, and source-of-truth mapping. Report wording
that summarizes project-wide public posture should use those website source
documents rather than inventing a server-local governance claim.

Proofline's planned product scope includes emergency incidents, non-emergency interaction records, timed safety checks, and evidence notes. The current backend stores generic incidents by default; optional incident-mode, capture-profile, escalation-policy, and sharing-state metadata are labels only unless the reviewed tree explicitly implements first-class behavior for them. Recipient-key, trusted-contact, sharing-grant, wrapped-key, and post-quantum envelope status is commit-specific: preserve implemented server or simulator behavior that code and tests prove without overstating it as completed production key custody, browser/client decryption, or cross-repository conformance. Any report claim that Proofline is ready for `v1 preview`, `v1.0.0`, or real-user evidence upload must be checked against `docs/v1-preview-readiness-checklist.md`.

## Rules

- Use the current checked-out branch as the Phase 2 editing workspace, not as
  automatic evidence for `<REVIEWED_COMMIT_SHA>`.
- Confirm `<REVIEWED_BRANCH_OR_REF>` resolves to `<REVIEWED_COMMIT_SHA>`. Ground
  reviewed-tree facts in commit-addressed reads or a clean isolated copy of that
  commit. Never attribute dirty working-tree or different-`HEAD` results to the
  reviewed commit.
- Pin repository citations and report metadata to `<REVIEWED_COMMIT_SHA>`, not to a moving branch name.
- Keep changes scoped to report validation, citation cleanup, and branch-scoped draft issues if requested.
- Do not change application code, CI behavior, repository settings, or GitHub issues unless explicitly requested.
- Keep the report and any issue drafts public-safe according to `SECURITY.md`.
- Remove raw viewer, incident, session, or future token-like values; secrets;
  Authorization headers; request bodies; uploaded file bytes; plaintext; raw
  keys; wrapped-key ciphertext; private deployment details; stored paths;
  object keys; exploit payloads; and user-safety data.
- Do not weaken security warnings.
- Do not claim production readiness, platform-store approval, legal review, compliance certification, penetration test, or formal audit.
- Do not treat absence of future-design features as a defect when source-of-truth docs mark them out of scope.
- Preserve the current backend ciphertext-only implementation boundary unless the report identifies implemented behavior that contradicts it.
- Preserve separate main API/viewer and private-admin listener groups and muxes,
  read-only public viewer paths, and the rule that public edges do not route
  private write or admin surfaces.
- Preserve that completed evidence bundles are encrypted chunk bundles with
  server-controlled ZIP entry names, not decrypted or playable media exports.
- Treat future incident-mode, web/iOS/Android client, key-custody, browser-decryption, break-glass, and post-quantum envelope documents as planning or future preview requirements unless implementation files exist in the reviewed tree.
- Treat Phase 1 command output as review evidence, not as a substitute for
  independent Phase 2 verification. Record which commands Phase 2 reruns and
  which Phase 1 evidence it only inspects.
- Do not claim that Phase 1 or Phase 2 ran a command unless the report or current
  validation record includes the exact command, relevant ref or environment,
  and result.
- Independently safety-review every command from the Phase 1 draft before
  execution. Do not access private or production services, inherit deployment
  credentials or endpoints, or run secret-dependent checks. Run
  reviewed-commit tests/builds only in an approved clean isolated temporary
  copy; treat dirty-workspace results as current-workspace evidence only.
- Route suspected or unresolved vulnerabilities through `SECURITY.md` and
  GitHub private vulnerability reporting. Do not create public issue drafts,
  public GitHub issues, or public report details that disclose them.

## First Steps

Check repository state:

```bash
git status --short --branch --untracked-files=all
git branch --show-current
git rev-parse HEAD
git rev-parse '<REVIEWED_BRANCH_OR_REF>^{commit}'
git cat-file -e '<REVIEWED_COMMIT_SHA>^{commit}'
git log --oneline -5
```

Read source-of-truth docs:

```bash
sed -n '1,240p' README.md
sed -n '1,220p' SECURITY.md
sed -n '1,260p' AGENTS.md
sed -n '1,280p' docs/README.md
sed -n '1,260p' ../website/docs/governance-and-political-alignment.md
sed -n '1,260p' ../website/docs/repository-readme-baseline.md
cat docs/v1-preview-direction.md
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
test -f docs/post-quantum-envelope.md && cat docs/post-quantum-envelope.md
test -f docs/browser-decryption.md && sed -n '1,360p' docs/browser-decryption.md
test -f docs/break-glass-key-access.md && sed -n '1,360p' docs/break-glass-key-access.md
test -f docs/ios-local-recorder-prototype.md && sed -n '1,360p' docs/ios-local-recorder-prototype.md
```

Do not replace the `docs/v1-preview-direction.md` or
`docs/post-quantum-envelope.md` full-file reads with fixed line caps unless the
replacement reports overflow visibly. These source-of-truth documents can grow
between report-validation runs, and silent truncation can miss new v1 direction
or post-quantum envelope requirements.

The working-tree reads above establish current publication guidance. Inspect
every material claim about the reviewed tree with a commit-addressed read such
as `git show <REVIEWED_COMMIT_SHA>:<path>`, or in an approved clean isolated
copy of the reviewed commit. If current guidance and reviewed-commit behavior
differ, record that distinction instead of pinning a working-tree observation
to the reviewed SHA.

Read the report:

```bash
cat <REPORT_PATH>
```

Review the Phase 1 validation evidence in the Source Registry. For each command
reported as executed, identify the exact command, reviewed ref or commit,
relevant environment, result, and any retained output or summary. Re-run safe,
relevant validation when needed to verify a material report claim, and record
whether Phase 2 independently executed the command or inspected Phase 1
evidence only.

Before editing, summarize:

1. reviewed branch/ref and reviewed commit SHA
2. current branch and current `HEAD`
3. target release/version
4. whether citations are portable and commit-pinned
5. whether a source registry exists and supports material claims
6. whether future-planning docs are separated from implemented behavior
7. whether Proofline naming and compatibility-name notes are represented correctly
8. whether incident-mode planning is represented as planning unless implemented
9. whether Phase 1 execution claims have exact evidence and which checks Phase 2 should rerun
10. whether issue drafts should be created and what branch scope they should use
11. likely files to update and docs-review checks

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

Do not create public issue drafts or public GitHub issues for suspected or
unresolved vulnerabilities. Follow `SECURITY.md` and GitHub private
vulnerability reporting. Public issue handling is limited to sanitized
non-vulnerability hardening or documentation follow-up.

## Report Validation Checklist

Check and fix, if needed:

- Product name is Proofline where describing current docs/product direction.
- Compatibility names remain when describing current artifacts, APIs, routes, config, or packages.
- Repository facts are pinned to `<REVIEWED_COMMIT_SHA>`.
- Mode-driven behavior is marked as planning unless implemented. Optional
  incident-mode, capture-profile, escalation-policy, and sharing-state metadata
  may be described as implemented labels only when the reviewed tree supports
  them.
- Recipient-key, trusted-contact, sharing-grant, and wrapped-key metadata or API
  behavior proved at the reviewed commit is not removed as wholly future, and
  is not overstated as completed production key custody or client decryption.
- Post-quantum behavior is classified component by component from code and tests
  at the reviewed commit; implemented server or simulator behavior is neither
  erased nor treated as proof of complete cross-repository preview readiness.
- Current `/v1` private boundary and public incident-viewer separation remain clear.
- Main API/viewer and private-admin routes remain on separate listener groups
  and muxes; public viewer paths remain read-only; public edges do not route
  private write or admin surfaces.
- Current backend ciphertext-only behavior is represented accurately.
- Completed evidence bundles remain encrypted chunk bundles with
  server-controlled entry names and are not described as decrypted or playable
  exports.
- V1 preview, v1.0.0, and real-user evidence-upload readiness claims are
  checked against `docs/v1-preview-readiness-checklist.md`; if any hard
  blocker remains incomplete, the report must use pre-v1 or experimental
  language instead of preview-ready language.
- Historical report names are not rewritten as if they used the new product name at the time.
- Internal renderer or tool citation identifiers are removed or converted to portable citation keys.
- The report and issue drafts contain none of the prohibited sensitive-data
  categories listed in Rules.
- Remove informal or conversational draft language, including humour, mascot references, assistant/meta commentary, and chat-only tone, unless it is quoted as reviewed evidence.
- Source Registry entries support material claims.
- External-source omissions are disclosed and affected claims are marked not independently verified.
- Phase 1 execution claims identify the exact command, relevant ref or
  environment, result, and limitations; Phase 2 separately records whether it
  reran the command or inspected the supplied evidence only.
- The report does not invent a no-network, no-web, or no-external-source constraint unless the maintainer explicitly imposed one.
- Missing external sources are treated as verification limitations, not as supporting evidence for findings.

## Common False Positives To Remove Or Downgrade

- Missing public product API authentication when docs state `/v1` is private
  and protected by local account sessions.
- Missing iOS/Android/protocol clients when docs mark them as planned future
  repositories.
- Missing production web-client behavior when docs mark `open-proofline/web-client`
  as a separate current experimental prototype rather than a production client.
- Missing first-class incident-mode behavior, capture-profile behavior,
  escalation policies, sharing-state behavior, or dead-man switch when docs mark
  them as future work.
- Missing browser decryption, production key custody, or break-glass behavior when docs mark them as future work.
- Implemented recipient-key, trusted-contact, sharing-grant, or wrapped-key
  metadata and API surfaces treated as wholly absent or future, or overstated as
  completed production key custody or client-side review.
- Implemented post-quantum server or simulator behavior treated as wholly
  future, or partial behavior overstated as complete web-client, key-custody,
  decryption, and cross-repository conformance.
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
scripts/check-markdown-links.py
git diff --check
```

If this prompt or the link checker changed, manually confirm the
`docs/v1-preview-direction.md` and `docs/post-quantum-envelope.md` reads remain
uncapped or fail visibly on overflow. If the Markdown link checker itself
changed, also run `scripts/check-markdown-links.py --self-test`.

If Go code changed unexpectedly, stop and report scope creep.

## Output

Summarize:

1. current branch
2. reviewed branch/ref and commit SHA
3. report path updated
4. issue drafts created, if any
5. citation and public-safety cleanup performed
6. Proofline naming and incident-mode boundary corrections made
7. Phase 1 execution evidence reviewed and Phase 2 checks independently run
8. validation/docs-review commands run
9. follow-up work
