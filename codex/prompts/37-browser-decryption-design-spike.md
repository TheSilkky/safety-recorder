# Codex Prompt: Browser-Side Incident Viewer Decryption Design Spike

Design browser/client-side decryption for the incident viewer.

This is a **design/documentation task only**.

Do **not** implement browser decryption.
Do **not** add JavaScript crypto code.
Do **not** change backend behaviour.
Do **not** add API routes.
Do **not** change database schema.
Do **not** add React, Node, npm, Vite, frontend build tooling, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, Docker Compose, Kubernetes, or cloud integrations.

## Goal

Explore how the incident viewer might decrypt completed evidence bundles or live stream chunks in the browser while keeping the backend from seeing raw media keys in normal operation.

This design should support the broader hybrid key custody goal:

- client-side encryption by default
- keys not solely stored on the user's phone
- trusted contacts can access emergency evidence
- browser-side decryption is one possible access path
- server escrow/break-glass remains a separate explicit design path

## Source of truth

Read:

- `README.md`
- `AGENTS.md`
- `docs/v1-preview-direction.md`
- `docs/encryption.md`
- `docs/key-custody.md`, if present
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/architecture.md`
- `docs/api.md`
- current incident viewer files:
  - `internal/httpapi/incident_viewer.go`
  - `internal/httpapi/web/templates/incident_viewer.html`
  - `internal/httpapi/web/static/scripts.js`
  - `internal/httpapi/assets.go`

## Document to create or update

Create or update the relevant design document. If the document already exists,
update it rather than creating a duplicate.

Path:

```text
docs/browser-decryption.md
```

## Required sections

### 1. Summary

Explain the purpose of browser-side decryption and how it fits the hybrid key custody goal.

### 2. Current viewer behaviour

Summarize current incident viewer behaviour:

- token-scoped read-only page
- completed encrypted bundle downloads
- static CSS/JS
- no browser decryption today

### 3. Candidate approaches

Evaluate:

1. URL fragment decryption capability, such as `#key=...`
2. Contact private key stored/imported in browser
3. One-time recovery phrase or file import
4. Separate trusted contact app
5. Static/signed viewer bundle
6. Browser decryption deferred in favour of non-browser client

For each, document:

- how it works
- what the backend sees
- what the browser sees
- UX strengths
- security weaknesses
- implementation complexity
- fit for emergency use

### 4. URL fragment model

If considering URL fragments, explain:

- fragments are not sent in normal HTTP requests
- JavaScript can read fragments locally
- fragments may still leak through screenshots, browser history, extensions, local compromise, or malicious served JS
- fragment keys are not a complete solution against a compromised server

### 5. Malicious server limitation

State clearly:

Browser decryption served by the same backend is useful against passive storage/database/blob compromise, but it is not full end-to-end protection against a malicious or compromised server that can serve modified JavaScript.

List mitigations:

- strict CSP
- static assets
- no inline script
- subresource integrity where applicable
- signed/static viewer bundle
- separate trusted viewer app
- pinned viewer release
- offline verification/decryption tool

Do not overstate these mitigations.

### 6. Web Crypto considerations

Document expected use of stable browser crypto APIs, not custom primitives.

Do not implement.

Discuss:

- PQ default envelope and explicit v1 AES-GCM simulator compatibility limits
- associated data requirements
- nonce/header parsing
- streaming limitations for large bundles
- memory usage risks
- worker-based decryption as future work
- no plaintext logging or DOM leaks

### 7. Emergency UX

Discuss:

- trusted contact opens emergency link
- contact obtains or already has decryption capability
- dashboard shows live status and location
- completed bundles can be decrypted/downloaded
- live streaming may need different session key handling
- what happens if decryption fails

### 8. Threat model impacts

Cover:

- stolen incident token
- stolen decryption fragment/capability
- compromised browser
- compromised backend
- malicious JavaScript
- compromised trusted contact device
- network attacker with HTTPS
- logs and referrers
- screenshots/browser history

### 9. Recommended direction

Recommend a phased approach.

Example:

```text
Phase 1: document browser decryption constraints
Phase 2: simulator/browser-compatible envelope verification
Phase 3: static proof-of-concept viewer for downloaded bundles
Phase 4: trusted contact key wrapping
Phase 5: live stream/session key model
```

### 10. Open questions

List unresolved decisions.

## Docs to update

Update small references in:

- `docs/README.md`
- `docs/key-custody.md`, if present
- `docs/encryption.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `CHANGELOG.md`

Do not make README large.

## Validation

Markdown-only:

```bash
git diff --stat
git diff -- docs CHANGELOG.md
git diff --check
```

If code changed, stop and explain why.

## Output

Summarize:

1. files changed
2. recommended browser decryption direction
3. major risks
4. mitigations
5. open questions
6. whether any code changed
