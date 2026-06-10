# Codex Prompt: Key Custody and Emergency Access Design

Design the production key custody and emergency access model for Proofline.

This is a **design/documentation task only**.

Do **not** implement backend decryption.
Do **not** implement browser decryption.
Do **not** implement iOS code.
Do **not** change encryption code.
Do **not** change API behaviour.
Do **not** change database schema.
Do **not** add new dependencies.
Do **not** add React, Node, npm, OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features.

## Goal

Create or update the relevant design document for the future production key
custody model. If the document already exists, update it rather than creating a
duplicate.

The ultimate target should be a **hybrid key custody model**:

- chunks/streams are encrypted client-side
- private keys belong to accounts, devices, and trusted contacts
- content-encryption keys belong to incidents, streams, or bounded chunk groups
- wrapped access records connect recipient key versions to content keys
- keys are not stored solely on the user's phone
- trusted contacts can eventually access emergency evidence without needing the phone to survive
- the backend may store wrapped/encrypted keys
- current runtime behavior remains ciphertext-only unless separately implemented
- browser/client-side decryption may be supported
- server escrow or server-side decryption may be supported only as an explicit break-glass/dead-man-switch mode
- all key custody and decryption changes must be deliberate, documented, tested, and threat-modeled

## Key product requirement

The system must assume the client device may be:

- lost
- damaged
- powered off
- taken
- destroyed
- unavailable during a dead-man-switch event

Therefore, production key material must not exist solely on the client device.

## Current implementation baseline

The current repository implementation is simulator/development only:

- backend stores opaque encrypted chunk bytes
- backend validates SHA-256 over ciphertext bytes
- backend creates encrypted ZIP evidence bundles
- server upload validation and simulator defaults use the accepted PQ payload
  envelope when the current branch implements it; the older v1 AES-256-GCM
  envelope is explicit compatibility only
- simulator can decrypt-verify downloaded bundles locally
- backend can store grant-bound wrapped-key metadata for explicitly designed
  owner-scoped sharing flows
- backend does not currently store raw keys
- backend does not currently decrypt chunks
- evidence bundles are not playable media exports

Do not weaken this current implementation in this task.

## Source of truth

Read current files before drafting:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
- `docs/v1-preview-direction.md`
- `docs/encryption.md`
- `docs/contacts-and-viewer-replacement.md`, if present
- `docs/post-quantum-envelope.md`, if present
- `docs/browser-decryption.md`, if present
- `docs/break-glass-key-access.md`, if present
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/architecture.md`
- `docs/deployment.md`
- `docs/api.md`
- `docs/simulator.md`, if present
- `docs/code-map.md`
- `internal/envelope`
- `cmd/simclient`
- GitHub issue `#9`, if GitHub CLI is available:

```bash
gh issue view 9 --repo open-proofline/server
```

If GitHub CLI is unavailable, continue from local docs.

## Design document

Create or update:

```text
docs/key-custody.md
```

If `docs/key-custody.md` already exists, update it rather than creating a
duplicate fixed-path design document.

The document should be professional, explicit, and security-focused.

## Required sections

### 1. Summary

Explain the future key custody goal in plain language.

State clearly:

- current backend remains ciphertext-only for now
- future product requires keys to be recoverable/usable without the user's phone
- the preferred long-term direction is a hybrid model
- server-side decryption is not forbidden, but it must never be introduced accidentally

### 2. Goals

Include:

- preserve evidence confidentiality where practical
- keep evidence accessible during emergencies
- allow trusted contacts to access emergency evidence
- support future live GPS/emergency dashboard use
- support future live audio/video streaming design
- support future dead-man-switch flows
- avoid single point of failure on the user's phone
- avoid casual/raw server access to media keys
- make key custody decisions auditable and documented

### 3. Non-goals for this milestone

Include:

- no implementation
- no iOS code
- no browser decryption implementation
- no server-side decryption implementation
- no new API routes
- no database schema changes
- no playable media export
- no push/SMS/Messenger delivery
- no new account-system implementation

### 4. Key custody models considered

Evaluate:

1. Client-only keys
2. Contact-wrapped keys
3. Browser/client-side incident viewer decryption
4. Server escrow / break-glass access
5. Threshold or multi-party recovery
6. Hybrid model

For each, document:

- how it works
- what it protects against
- what it does not protect against
- availability impact
- operational complexity
- implementation complexity
- emergency UX impact
- trust assumptions
- whether it fits this project

### 5. Recommended ultimate model

Recommend a hybrid model.

Suggested structure:

```text
Default:
  client encrypts per-incident or per-stream media keys
  backend stores ciphertext chunks
  backend stores wrapped/encrypted media keys for trusted contacts
  incident viewer or future trusted client performs decryption client-side where possible

Optional future mode:
  server escrow or break-glass key access for dead-man-switch/emergency cases
  disabled by default or separately configured
  heavily documented
  audited/logged
  protected by explicit access policy
```

The design should decide whether this is the recommended direction or list open questions blocking the decision.

### 6. Key hierarchy

Propose a key hierarchy.

Consider:

- account, device, and trusted-contact recipient keys
- recipient key versions
- per-incident media key
- per-stream media key
- content-encryption keys for incidents, streams, or bounded chunk groups
- per-chunk nonce
- key IDs
- wrapped media keys
- wrapped access records that connect recipient key versions to content keys
- contact public keys
- server escrow keys
- future rotation/revocation implications

Do not invent cryptographic primitives.

State that future implementation must use stable, reviewed libraries.

### 7. Emergency contact access

Define how trusted contacts might gain access.

Consider:

- pre-registered trusted contacts
- public/private key pairs
- contact-wrapped media keys
- incident viewer token plus decryption capability
- browser-side decryption tradeoffs
- future app-based contact decryption
- lost contact key scenarios
- removing/revoking contacts

### 8. Browser decryption considerations

Document that browser decryption can be useful but has limits.

Include:

- URL fragment key delivery may keep key material out of HTTP requests
- JavaScript served by the backend can still be a trust problem
- a compromised server can potentially serve malicious JavaScript
- strict CSP and static assets help but do not fully solve malicious-server risk
- browser decryption is stronger against passive storage compromise than against active server compromise

### 9. Server escrow / break-glass considerations

Document when server-side key access may be acceptable.

Include:

- dead-man-switch trigger
- emergency access escalation
- audit logging
- access policy
- explicit configuration
- rate limiting
- operational warnings
- incident review
- key storage options such as KMS/HSM/locked local secret store as future deployment choices

Do not implement this.

### 10. Metadata and live dashboard implications

Address:

- live GPS data may be visible to the backend/incident viewer depending on design
- metadata may not be encrypted in the same way as media chunks
- live streaming may require a different key/session model than completed chunk bundles
- emergency dashboard usability may trade off against strict confidentiality

### 11. Threat model impacts

Discuss impacts on:

- compromised backend
- compromised database
- stolen blob storage
- compromised incident viewer token
- malicious/compromised reverse proxy
- compromised trusted contact device
- destroyed phone
- maintainer/operator misuse
- dead-man-switch false positive/false negative

### 12. Open questions

List unresolved decisions.

### 13. Proposed implementation phases

Define phases without implementing them.

Example:

```text
Phase 1: design and docs
Phase 2: contact-wrapped key prototype in simulator
Phase 3: browser/client-side decrypt prototype
Phase 4: iOS Keychain and contact-key planning
Phase 5: emergency access/dead-man-switch key policy
Phase 6: optional server escrow/break-glass implementation
```

## Docs to update

After creating or updating `docs/key-custody.md`, update only small references in:

- `docs/README.md`
- `docs/encryption.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/architecture.md`, if needed
- `README.md`, only with a concise link or roadmap wording
- `CHANGELOG.md`

Do not make the README huge.

## Validation

Because this is a documentation-only task:

```bash
git diff --stat
git diff -- docs README.md CHANGELOG.md
git diff --check
```

If any code changed, stop and explain why.

Go tests are not required unless code changed.

## Output

Summarize:

1. files changed
2. recommended key custody model
3. key design decisions made
4. unresolved open questions
5. docs updated
6. implementation phases proposed
7. whether any code was changed
