# Historical Work Order: Create v1 Preview Direction Document

This is a one-off Codex work order for `open-proofline/server`.

Date: 2026-06-09
Branch: `codex/v1-preview-direction-work-order-2026-06-09`
Base branch: `develop`
Base commit: `2c46583bb5758aeb619c2ba1a501e103d1ec655d`

## Goal

Create a source-of-truth draft document that defines Proofline's v1 preview direction, terminology, repository boundaries, and implementation boundaries.

The document should be written from the repository's current documentation after local review. The maintainer direction in this work order is context, not final copy. Do not paste this work order into the final document with light edits. Synthesize the final document from the current docs and resolve terminology carefully.

Suggested target path:

```text
docs/v1-preview-direction.md
```

If current docs reveal a better path/name, use that and explain why in the final summary.

## Reasoning

Use very-high reasoning.

This document is meant to guide future Codex work and maintainer decisions. It should make the true project direction clear enough that future work does not misread current prototype limitations as permanent product non-goals, and does not treat future direction as permission to implement unrelated behavior without an issue.

The document should be a direction-setting source of truth, not a release checklist. It may include release-readiness implications, but its main job is to explain:

- what Proofline is becoming;
- which terms are canonical;
- what is implemented now;
- what is required for v1 preview;
- what is deliberately out of default v1 scope;
- which repo owns which responsibilities;
- how Codex should reason about current docs versus future direction.

## Required local documentation review

Before writing, inspect the repository documentation locally.

At minimum, review:

- `README.md`
- `AGENTS.md`, if present
- `SECURITY.md`
- `CHANGELOG.md`
- `docs/README.md`
- every Markdown file under `docs/` that discusses security, deployment, API, capture, viewer behavior, key custody, contacts, grants, wrapped keys, browser decryption, relay, incident modes, retention, deletion, logging, production cluster scope, and public exposure
- `codex/README.md`
- relevant prompts under `codex/prompts/`, especially prompts that define source-of-truth behavior, code review, security review, issue work, and documentation update behavior

Use a local file listing command such as:

```bash
find docs codex -type f -name '*.md' | sort
```

Then inspect the relevant files. Do not rely only on the file list above if additional docs exist.

## Direction context from maintainer discussion

Use this section as context. Do not copy it verbatim into the final document.

### Product direction

Proofline v1 preview should mean the intended encrypted capture and review flow works end-to-end. It does not mean only that the server builds, users can register, or a metadata UI exists.

Proofline is broader than an emergency-only recorder. It should support private encrypted capture for:

- emergency incidents;
- interaction records;
- safety checks;
- evidence notes.

Escalation, sharing, key release, notification, retention, and public links are separate explicit systems and must not be silently inferred from incident labels.

### Current API posture

Do not describe the Go API as inherently unsafe or not public-ready merely because deployment hardening is required.

The Go API is responsible for application security boundaries: authentication, authorization, route separation, admin/private listener separation, request-size limits, route-class limits, safe logging, safe errors, storage invariants, token hashing, upload validation, and ciphertext-only evidence handling.

The system administrator is responsible for secure deployment: TLS termination, Cloudflare or reverse-proxy configuration, WAF rules, edge rate limits, tunnel configuration, firewalling, private-admin reachability, monitoring, backups, host/network hardening, and operational response.

The release process is responsible for review and validation.

The API can be secure when properly deployed. A deployment can still be unsafe if the operator routes the wrong listener, exposes private admin surfaces, disables required controls, or violates documented boundaries.

### Server versus web-client responsibilities

The server repository owns backend primitives and server behavior.

The web-client repository owns the user-facing app, account portal, no-account viewer replacement, full incident review UX, browser recording/capture, browser-held private keys, browser-side encryption/decryption, trusted-contact UX, and device key-sharing UX.

The server should not carry the built-in server-rendered incident viewer into v1 preview as a fallback or compatibility surface. This is an experimental pre-v1 project. Removing that viewer before v1 is acceptable when documented in release notes. The server should keep the backend token-scoped viewer/data primitives the web client needs.

### Browser crypto is in scope for v1 preview

Do not list browser encryption/decryption as a permanent non-goal for v1 preview.

Current docs may say the prototype does not implement browser decryption or recording. That remains true today, but v1 direction expects the web client to be a real cryptographic client:

- browser recording/capture is in scope;
- browser-side encryption is in scope;
- authorized browser-side decryption is in scope;
- browser-held private keys are in scope;
- full incident review is in scope.

Backend decryption, default server key escrow, raw server-held keys, operator plaintext access, and emergency-services integration remain out of default v1 scope unless separately designed, threat-modeled, documented, and tested.

### Trusted contacts and device key sharing are in scope for v1 preview

Trusted-contact behavior, trusted-contact access, wrapped-key delivery, and full incident review are v1 direction.

Device key sharing should not copy private keys between devices. Each device should have its own key material. New devices should be approved by an existing trusted device or a separately designed recovery flow. Existing devices should rewrap relevant CEKs to the new device recipient public key. The server should store public keys, roster metadata, signatures, grants, and wrapped-key ciphertext, not private keys or raw CEKs.

### Capture terminology

Use the terminology already established in server docs unless current docs have evolved:

- `incident_mode`: user-visible reason for capture;
- `capture_profile`: what the client intends to capture;
- `escalation_policy`: if/when trusted contacts or other future workflows are triggered;
- `sharing_state`: access/export/revocation state;
- capture stream group: one logical capture source/session;
- concrete stream variant: one upload lane in a capture stream group;
- source timeline: source-capture time range shared across variants;
- supersession: evidence-selection relationship, not deletion.

Do not use `capture_modes` if current docs use `incident_mode` for the product-level mode. If a web-client or protocol doc uses different wording, call out the inconsistency and align to the accepted server terminology unless there is a strong reason not to.

### Capture stream variant direction

Future capture should support multiple encrypted stream variants for the same source session:

- `live_preview`;
- `evidence_master`;
- `audio_priority`;
- `location_context`;
- `metadata_context`.

Reduced-quality near-live chunks are preserved evidence, not disposable previews. Full-quality evidence-master chunks may supersede lower-quality chunks only after backend confirmation and source-time coverage validation. Supersession is not deletion.

### GPS and context direction

GPS/location context can be evidence. Plaintext GPS, speed, heading, route history, or detailed safety state must not become server, relay, edge, log, or public metadata by accident.

The final direction document should explain that GPS/context handling belongs in encrypted context designs and must coordinate with capture stream variants and supersession.

### Cloudflare and edge direction

Cloudflare can be used as an edge provider, WAF, tunnel provider, and abuse-control layer. For Proofline API and capture paths, Cloudflare should be treated as an untrusted metadata-observing edge, not as a confidentiality boundary.

Sensitive evidence, GPS/location context, safety metadata, and key material must be encrypted before reaching Cloudflare.

Community services such as Redlib have a separate threat model. Challenge-first Cloudflare/Bot Fight style protection may be appropriate for Redlib to protect a residential egress IP from bot abuse and Reddit blocking. That posture must not be copied blindly onto Proofline API/capture paths.

### Public registration and quota direction

Public open registration is allowed only as explicitly configured preview deployment mode. It should not be default.

It requires email verification, rate limits, deployment controls, and account-scoped committed blob quota. A 10GB account blob storage limit is an acceptable preview default after quota enforcement is implemented.

Quota is an abuse and cost-control mechanism, not billing. Do not introduce payment providers, paid registration, subscriptions, account plans, or hosted-account entitlements in this document as v1 preview requirements unless current docs already scope them.

### Public web-client deployment integrity

The web client does not deploy publicly anywhere today. Deploy CI should not be configured until the web-client is at or near v1 preview.

Cloudflare Worker deployed asset integrity validation is a future public web-client deployment requirement, not a current implementation blocker.

When public web-client deployment exists, deployment integrity should include built asset manifests, root dist hashes, post-deploy verification that served bytes match built bytes, non-sensitive build provenance, SRI where practical, CSP, and documentation that this detects deployment drift but does not fully prevent targeted same-origin edge tampering.

## Desired document style

The final document should:

- be concise enough to be used by Codex before implementation work;
- be explicit about source-of-truth terminology;
- separate current behavior from future v1 preview direction;
- avoid becoming a long issue checklist;
- avoid marketing language;
- avoid production-readiness claims;
- explain what is in v1 direction versus out of default v1 scope;
- point to authoritative docs for details instead of duplicating every design doc;
- use future tense where behavior is not implemented;
- use present tense only for behavior verified in current docs;
- identify documentation conflicts or stale current docs that need follow-up.

## Suggested structure

Codex may adjust this structure after local review, but the final document should cover these themes:

1. Purpose and use by Codex
2. Repository roles
3. Current server posture
4. Current web-client posture, as known from cross-repo context if available
5. Product direction
6. Canonical terminology
7. Incident mode direction
8. Server v1 direction
9. Web-client v1 direction
10. Built-in server incident viewer removal direction
11. Browser recording direction
12. Browser cryptography and browser trust direction
13. Device key sharing direction
14. Trusted-contact direction
15. Full incident review direction
16. Capture stream groups, variants, and evidence preservation direction
17. GPS/location/context direction
18. Regional relay direction
19. Cloudflare and edge deployment direction
20. Community services are separate from Proofline product surfaces
21. Public registration and account quota direction
22. Deployment responsibility split
23. Admin/operator boundary
24. Logging direction
25. Documentation/source-of-truth map
26. Codex guidance
27. Actual v1 non-goals versus current prototype non-goals

## Expected documentation updates

Create the main document and update cross-links where appropriate.

Likely files to update:

- `docs/README.md`
- `docs/code-map.md`, if source-of-truth docs are listed there
- `README.md`, only if a short pointer to the new direction doc is helpful
- `SECURITY.md`, only if needed and only with very careful wording
- `codex/README.md`, if Codex source-of-truth reading order should reference this doc
- relevant `codex/prompts/*.md` only if they already maintain source-of-truth reading lists and should include the new document

Do not broaden the PR by rewriting every design doc to match the new direction. If existing docs need content changes beyond small cross-links or obvious stale wording, list them as follow-up work in the final summary.

## Explicit non-goals for this work order

Do not:

- implement server code;
- implement web-client code;
- edit sibling repositories;
- create GitHub issues;
- create or merge a PR;
- add deployment CI;
- remove the built-in server viewer;
- add browser recording;
- add browser decryption;
- add trusted-contact decryption;
- add device key-sharing behavior;
- add account quota implementation;
- add Cloudflare configuration;
- add payment, billing, subscriptions, OAuth, or JWT;
- add backend decryption, key escrow, raw server-held keys, emergency-services integration, playable export, or production-readiness claims.

This is documentation-source-of-truth work only.

## Sensitive-data rules

Never include raw tokens, session tokens, viewer tokens, incident tokens, Authorization headers, request bodies, uploaded bytes, plaintext, raw keys, raw media keys, recipient private keys, unwrapped secrets, wrapped-key ciphertext, verification credentials, private deployment details, exploit payloads, object-store credentials, stored paths, object keys, private filesystem paths, SMTP credentials, database DSNs, bootstrap secrets, or user safety data in docs, examples, prompts, tests, issue text, PR text, logs, or summaries.

Examples must be synthetic and safe.

## Validation

For docs-only changes, run:

```bash
git diff --check
git diff --stat
git diff -- README.md AGENTS.md SECURITY.md CHANGELOG.md docs codex
```

If the documentation update changes Markdown links, inspect affected links manually where practical.

If Go code changes unexpectedly, stop and explain why. Do not run Go validation unless code changed or the maintainer explicitly requests it.

## Output

Summarize:

1. files changed;
2. docs inspected;
3. main direction document path;
4. key terminology decisions;
5. current-versus-future wording decisions;
6. any documentation conflicts or stale wording discovered;
7. cross-links updated;
8. validation commands run and results;
9. follow-up issues or docs work recommended, without creating issues;
10. confirmation that no runtime code, migrations, deployment config, sibling repositories, GitHub issues, or PRs were changed.
