# Codex Prompt: Break-Glass and Dead-Man-Switch Key Access Design

Design server escrow / break-glass / dead-man-switch key access for Proofline.

This is a **design/documentation task only**.

Do **not** implement server-side decryption.
Do **not** implement key escrow.
Do **not** change database schema.
Do **not** add API routes.
Do **not** add background jobs.
Do **not** implement dead-man-switch logic.
Do **not** add dependencies.
Do **not** add OAuth, JWT, new account-system features beyond the implemented local account/session and registration flows, SMS, Messenger, push notifications, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features.

## Goal

Design how server-assisted emergency key access could work as part of the future hybrid key custody model.

This design should evaluate whether raw server-side key access or server-side decryption is acceptable for:

- dead-man-switch triggers
- emergency escalation
- trusted contact access
- evidence preservation when the phone is destroyed or unavailable

## Source of truth

Read:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `docs/encryption.md`
- `docs/key-custody.md`, if present
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/deployment.md`
- `docs/architecture.md`
- GitHub issue `#9`, if available

## Document to create

Create:

```text
docs/break-glass-key-access.md
```

## Required sections

### 1. Summary

Explain what break-glass/dead-man-switch key access is and why it might be needed.

State clearly:

- this is not implemented
- this would increase backend/operator trust requirements
- this must be explicit and auditable if ever implemented

### 2. Availability requirement

Document that the system must assume the user's phone may be lost, damaged, powered off, taken, or destroyed.

Explain why phone-only keys are not sufficient for this product.

### 3. Candidate access models

Evaluate:

1. Server stores wrapped key material only
2. Server can unwrap keys under break-glass policy
3. Server decrypts/transcodes media under break-glass policy
4. n-of-m trusted contacts
5. maintainer/operator assisted recovery
6. external KMS/HSM/secret store
7. no server break-glass support

For each, document:

- how it works
- what the server can access
- operator trust requirements
- audit requirements
- failure modes
- usability during emergency
- fit for personal/self-hosted deployment

### 4. Trigger policy

Discuss possible triggers:

- explicit user panic/incident start
- missed check-ins
- dead-man-switch timeout
- trusted contact request
- manual maintainer action
- repeated failed uploads
- device offline threshold

Do not implement.

Document false positive and false negative risks.

### 5. Access controls

Design required controls if server-assisted key access exists:

- explicit enable/disable configuration
- per-incident policy
- trusted contact authorization
- operator authentication
- local-only/private API boundary
- audit log
- rate limits
- notification events
- revocation
- least privilege
- separation between viewer token and decryption authority

### 6. Audit and logging

Define what should be logged:

- key unwrap attempts
- successful key access
- failed key access
- who/what triggered access
- incident ID
- timestamp
- policy decision

Define what must never be logged:

- raw keys
- plaintext
- raw incident tokens
- uploaded bytes
- sensitive user safety data beyond necessary audit metadata

### 7. Deployment assumptions

Discuss:

- self-hosted local server
- Docker deployment
- WireGuard/private API
- HTTPS incident viewer
- external KMS/HSM future option
- disk encryption
- backup/restore impact
- retention/deletion impact

### 8. Threat model impacts

Cover:

- backend compromise
- database compromise
- blob storage compromise
- operator misuse
- malicious trusted contact
- stolen incident token
- stolen key escrow material
- false dead-man-switch trigger
- destroyed phone
- compromised server during browser decryption

### 9. Recommended direction

Recommend whether break-glass access should be:

- unsupported
- documented only
- optional future mode
- required part of production design

The preferred answer may be:

```text
Default model:
  contact-wrapped keys and client-side/browser-side decryption

Optional future mode:
  server escrow/break-glass access for dead-man-switch cases
  disabled by default
  explicitly configured
  audited
  heavily documented
```

### 10. Implementation prerequisites

List prerequisites before implementation, such as:

- key custody design accepted
- browser/contact decryption design accepted
- retention/backup policy accepted
- token expiry policy implemented
- `/v1` access-control story defined
- audit log design
- deployment hardening
- test plan

### 11. Open questions

List unresolved issues.

## Docs to update

Update small references in:

- `docs/README.md`
- `docs/key-custody.md`, if present
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/deployment.md`
- `CHANGELOG.md`

Do not make README large.

## Validation

Markdown-only:

```bash
git diff --stat
git diff -- docs CHANGELOG.md
```

If code changed, stop and explain why.

## Output

Summarize:

1. files changed
2. recommended break-glass direction
3. operator/server trust implications
4. required controls
5. open questions
6. whether any code changed
