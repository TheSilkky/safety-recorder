# Codex Prompt: Update Key Custody Guardrails in Codex Prompts

Update Codex prompt wording so it no longer treats server-side decryption or server-side key storage as permanently forbidden.

This is a **prompt/documentation task only**.

Do **not** change application code.
Do **not** change backend behaviour.
Do **not** implement server-side decryption.
Do **not** implement browser decryption.
Do **not** implement key escrow.
Do **not** change the encryption envelope.
Do **not** change database schema.
Do **not** add dependencies.

## Goal

Replace overly absolute wording such as:

```text
Preserve the backend's ciphertext-only posture: no backend decryption and no server-side key storage.
```

with wording that preserves the **current implementation** while allowing future explicit key custody design.

## Required policy meaning

Use this meaning throughout reusable prompts and process docs:

- The current backend implementation is ciphertext-only.
- The current backend should not decrypt or store raw keys as an incidental change.
- Server-side decryption is not permanently off limits.
- Server-side key storage is not permanently off limits.
- Keys must not exist solely on the user's phone in the future production design.
- Any change to key custody or decryption must be explicit, documented, reviewed, and threat-modeled.
- Wrapped/encrypted server-stored keys may be acceptable.
- Raw server-held keys or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode.
- Do not introduce backend decryption, key escrow, browser decryption, or key-sharing behaviour accidentally.

## Source files to inspect

Read:

- `README.md`
- `AGENTS.md`
- `docs/encryption.md`
- `docs/key-custody.md`, if present
- `docs/security-model.md`
- `docs/threat-model.md`
- `codex/README.md`
- `codex/prompts/*.md`

## Files to update

Update only Markdown files as needed:

- `AGENTS.md`
- `codex/README.md`
- `codex/prompts/*.md`
- `docs/encryption.md`, only if a short wording clarification is needed
- `docs/security-model.md`, only if a short wording clarification is needed
- `docs/threat-model.md`, only if a short wording clarification is needed

Do not update historical prompts in `codex/archive/` or `codex/work-orders/` unless they are explicitly marked reusable.

## Recommended replacement language

Use this as the central reusable wording:

```md
Preserve the current backend ciphertext-only implementation unless the task explicitly concerns key custody, emergency access, or decryption design.

Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour as an incidental implementation detail.

Any change to key custody or decryption must be handled as an explicit security-sensitive design task. It must update the threat model, security model, encryption docs, tests, and operational guidance before or alongside implementation.

Server storage of wrapped/encrypted keys may be acceptable if explicitly designed. Raw server-side key access or server-side decryption may be acceptable only as a deliberate, documented break-glass or emergency-access mode with clear access controls, audit expectations, and deployment warnings.
```

## AGENTS.md suggested update

Replace any absolute "no server keys ever" wording with something like:

```md
- Preserve the current backend ciphertext-only implementation unless a task explicitly concerns key custody, emergency access, or decryption design.
- Do not introduce backend decryption, raw server-held decryption keys, key escrow, browser decryption, or key-sharing behaviour incidentally.
- Future production key custody should assume the user's phone may be unavailable; keys must not exist solely on the client device.
- Server storage of wrapped/encrypted keys may be acceptable if explicitly designed.
- Raw server-side key access or server-side decryption may be acceptable only as a deliberate break-glass/dead-man-switch/emergency-access mode with clear access controls, audit expectations, and deployment warnings.
```

Keep `AGENTS.md` concise.

## Prompt updates

Update reusable prompts so they distinguish:

```text
current implementation constraints
future explicit design tasks
```

For general prompts, keep the guardrail strict:

```text
Do not add backend decryption or key custody changes unless this task explicitly concerns key custody, emergency access, or decryption design.
```

For key custody design prompts, allow analysis of:

- contact-wrapped keys
- browser/client-side decryption
- server escrow
- break-glass access
- dead-man-switch access
- hybrid model

## Validation

Because this is Markdown-only:

```bash
git diff --stat
git diff -- AGENTS.md codex docs
git diff --check
```

If any non-Markdown files changed, stop and explain why.

Go tests are not required unless code changed.

## Output

Summarize:

1. files changed
2. absolute wording removed
3. new key custody guardrail wording
4. whether historical prompts were left untouched
5. whether any code changed
