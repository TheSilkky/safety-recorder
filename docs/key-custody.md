# Key Custody And Emergency Access

This document defines the intended production key custody direction for
Proofline. The current backend now includes account-owner contact public-key
metadata, sharing-grant metadata, and grant-bound wrapped-key record storage and
delivery, but this document remains the security boundary for production
custody. The current implementation does not add browser decryption, backend
decryption, server escrow, trusted-contact accounts, simulator envelope
changes, or production key custody behavior.

## Summary

Proofline currently keeps the backend ciphertext-only. Clients or the simulator upload already-encrypted chunk bytes, the backend validates hashes over those ciphertext bytes, and evidence bundles contain encrypted chunks plus JSON manifests. The backend does not store media keys and does not decrypt media.

That model is a good confidentiality baseline, but it is not sufficient for the future product. During a real emergency, safety check, or high-risk interaction, the user's phone may be lost, damaged, powered off, taken, destroyed, or otherwise unavailable. Production key material therefore must not exist solely on the phone.

The preferred long-term direction is a hybrid key custody model:

- clients encrypt media before upload
- the backend stores ciphertext chunks
- the backend may store wrapped or encrypted media keys
- trusted contacts can eventually decrypt authorised incident evidence without needing the phone to survive
- client-side or trusted-contact-side decryption is preferred where practical
- server escrow or server-side decryption is allowed only as an explicit break-glass or dead-man-switch mode

Server-side decryption is not forbidden forever, but it must never be introduced accidentally. Any future key custody, recovery, escrow, browser decryption, or server decryption work must be deliberate, documented, tested, and threat-modeled.

Future key custody also depends on the role and grant boundaries in
[v1-access-control.md](v1-access-control.md). Account-owner,
trusted-contact, public-link, admin/operator, and optional escrow access must
be designed separately from the encryption envelope itself.
The contact public-key lifecycle, trusted-contact grants, and wrapped-key
metadata boundary are designed in
[contact-key-sharing-grants.md](contact-key-sharing-grants.md).

## Goals

- Preserve evidence confidentiality where practical.
- Keep authorised evidence accessible when the user's phone is unavailable.
- Allow trusted contacts to access emergency or safety-check evidence when policy permits it.
- Support non-emergency interaction records without forcing emergency escalation.
- Support future live GPS and incident dashboard use.
- Support future live audio/video streaming design.
- Support future dead-man-switch flows.
- Avoid making the phone the only place where usable keys exist.
- Avoid casual or raw server access to media keys.
- Make key custody decisions auditable and documented.

## Non-Goals For This Milestone

- No iOS, Android, or web-client code.
- No browser decryption implementation.
- No server-side decryption implementation.
- No key-custody, wrapped-key, decryption, or emergency-access behavior tied to
  incident-mode, capture-profile, escalation-policy, or sharing-state metadata.
- No playable media export.
- No push, SMS, or Messenger delivery.
- No user account system.

## Incident Mode Implications

Planned incident modes change access policy expectations, not the current
ciphertext-only backend behavior. The current optional mode metadata fields keep
capture profile, escalation policy, sharing state, and key-access scope separate;
see [incident-modes.md](incident-modes.md).

| Incident mode | Key custody implication |
|---|---|
| Emergency incident | Trusted contacts may need access if the phone is unavailable. Wrapped keys or explicit break-glass policy matter most here. |
| Interaction record | The default should be private capture with no automatic escalation. Sharing/export should be deliberate. |
| Safety check | Missed check-ins may trigger trusted-contact access. False positives and cancellation behavior need explicit policy before implementation. |
| Evidence note | Usually private by default. Export, retention, and deletion policy may matter more than live access. |

Do not treat incident-mode labels as sufficient access control. Account-owner,
trusted-contact, public-link, admin/operator, and optional escrow access must be
designed separately; see [v1-access-control.md](v1-access-control.md).

## Key Custody Models Considered

### 1. Client-Only Keys

The recording client generates media keys and stores them only on the phone, for example in the iOS Keychain or Android Keystore. Uploaded chunks are encrypted before upload. The backend never receives media keys or wrapped copies.

This protects against passive backend, database, and blob-storage compromise, but it fails the core availability requirement if the phone is lost, destroyed, seized, or unavailable. It remains useful as a development starting point and confidentiality baseline, but it is not sufficient as the production model.

Tradeoffs:

- Protects against: passive server, database, blob-store, and bundle compromise
  when the attacker does not have the phone or local key backup.
- Does not protect against: phone compromise, local key extraction, or loss of
  all usable key copies.
- Availability impact: poor for emergencies because the phone is the only
  decryption path.
- Operational complexity: low for the server, higher for support because key
  loss is unrecoverable.
- Implementation complexity: moderate in clients because Keychain or platform
  key-storage behavior must be tested across lock, reboot, backup, and restore
  states.
- Emergency UX impact: weak; trusted contacts cannot use uploaded evidence if
  the phone is gone.
- Trust assumptions: the client device remains available and uncompromised.
- Fit: acceptable only as a development baseline or optional private-only mode,
  not as the production default.

### 2. Contact-Wrapped Keys

The user pre-registers trusted contacts. Each contact has a public key. For each incident or stream, the client encrypts media with a media key and wraps that media key to one or more contact public keys. The backend stores ciphertext chunks and wrapped media keys, but not raw keys.

This is the strongest default fit for Proofline. It preserves ordinary backend ciphertext-only operation while allowing trusted contacts to decrypt authorised evidence if the phone is gone. It requires careful design for contact enrollment, public-key verification, revocation, key loss, and contact-device compromise.

Tradeoffs:

- Protects against: passive backend and storage compromise, as long as contact
  private keys remain private.
- Does not protect against: compromised trusted-contact devices, malicious
  viewer code that receives private keys or plaintext, or incorrect contact-key
  enrollment.
- Availability impact: strong when contacts are pre-enrolled and can access the
  wrapped keys.
- Operational complexity: medium; enrollment, public-key verification,
  revocation, rotation, and lost-contact-key support must be understandable.
- Implementation complexity: medium to high; it needs stable public-key wrapping
  formats, metadata, access-control, and client-side unwrap flows.
- Emergency UX impact: good if setup happens before the incident; poor if the
  user tries to add contacts for the first time during an emergency.
- Trust assumptions: selected contacts and their devices are trusted for the
  incidents or modes where access is granted.
- Fit: recommended default direction.

### 3. Browser Or Client-Side Viewer Decryption

The viewer downloads encrypted evidence and wrapped keys, then decrypts in the browser or another trusted client. Key material may be delivered out of band, opened by a contact private key, or placed in a URL fragment so it is not sent in HTTP requests.

This can make trusted-contact access easier, but it does not protect against an actively compromised backend serving malicious JavaScript. Browser decryption is stronger against passive storage compromise than active server compromise. A future high-assurance design may need signed/static viewer assets, reproducible builds, or a native trusted-contact client.

See [browser-decryption.md](browser-decryption.md).

Tradeoffs:

- Protects against: passive database, blob-storage, and bundle compromise when
  raw keys are not sent to the backend.
- Does not protect against: a compromised backend serving malicious JavaScript,
  compromised browsers, extensions, or endpoint malware.
- Availability impact: good because contacts can use a normal browser, but only
  if they have the right key material or decryption capability.
- Operational complexity: medium; deployment must control CSP, static assets,
  caching, logging, and user guidance for secret material.
- Implementation complexity: high for large bundles, memory safety, ZIP parsing,
  worker behavior, and cross-browser crypto compatibility.
- Emergency UX impact: potentially strong because it avoids installing a native
  contact app during a crisis.
- Trust assumptions: the browser, delivered JavaScript, and device are trusted
  at access time.
- Fit: useful follow-up after contact-wrapped keys, not a reason to weaken the
  current backend.

### 4. Server Escrow / Break-Glass Access

The backend or deployment environment stores an encrypted or otherwise protected way to recover media keys. During an explicit emergency, dead-man-switch, or break-glass event, the server can obtain key access or perform server-side decryption according to configured policy.

This improves availability when the phone and trusted-contact keys are unavailable, but it creates serious operator, hosting, audit, and misuse risks. It is acceptable only as an optional future mode. It must be disabled by default or separately configured, clearly documented, audited, rate-limited, and treated as deliberate break-glass capability.

See [break-glass-key-access.md](break-glass-key-access.md).

Tradeoffs:

- Protects against: total loss of the phone and trusted-contact keys when a
  reviewed emergency policy permits recovery.
- Does not protect against: malicious operators, compromised hosting, weak
  policy triggers, or excessive server privilege.
- Availability impact: strongest for dead-man-switch and disaster-recovery
  cases.
- Operational complexity: high; it needs secure key storage, audit, approval,
  incident review, rate limiting, monitoring, deployment warnings, and operator
  training.
- Implementation complexity: high; server-assisted access touches API,
  storage, authorization, audit, deployment, and threat-model boundaries.
- Emergency UX impact: strong if policy is correct; harmful if false positives
  or misuse expose sensitive non-emergency evidence.
- Trust assumptions: the deployment operator, escrow mechanism, and access
  policy are trusted under explicit break-glass conditions.
- Fit: optional future mode only, never the default ordinary path.

### 5. Threshold Or Multi-Party Recovery

Key recovery requires multiple parties or shares, such as two trusted contacts, one contact plus server escrow, or another threshold arrangement. No single party can recover the media key alone.

This may reduce unilateral misuse but can hurt emergency availability if enough parties are not reachable. It may be useful later for high-risk users or escrow modes, but it should not be the first production default.

Tradeoffs:

- Protects against: unilateral compromise or misuse by one contact or one
  escrow holder, depending on the threshold design.
- Does not protect against: collusion, enough compromised parties, bad share
  backup practices, or user confusion.
- Availability impact: mixed; stronger misuse resistance can make emergency
  access slower or impossible if parties are unreachable.
- Operational complexity: high; enrollment, recovery rehearsal, share rotation,
  and support flows are difficult.
- Implementation complexity: high and should use reviewed libraries or
  protocols only.
- Emergency UX impact: risky for urgent access unless the user has rehearsed
  the contact workflow.
- Trust assumptions: enough independent parties remain reachable and honest.
- Fit: future advanced option, not an initial production default.

### 6. Hybrid Model

The client encrypts media before upload. The backend stores ciphertext chunks and wrapped media keys. Trusted contacts are the default recovery path. Browser or app-based client-side decryption is preferred where practical. Optional server escrow or break-glass access can be added as a separate explicit mode for dead-man-switch and emergency-access cases.

This is the recommended direction.

Tradeoffs:

- Protects against: passive backend and storage compromise in default mode,
  while preserving availability when the phone is unavailable.
- Does not protect against: all active-server, malicious-client, compromised
  contact, or operator-risk cases; each mode still needs scoped controls.
- Availability impact: best balance because normal access uses contact-wrapped
  keys and optional emergency access can be designed separately.
- Operational complexity: medium in default mode and high for any escrow mode.
- Implementation complexity: incremental; simulator wrapped-key metadata,
  access-control, client key storage, browser/native decrypt, and break-glass
  policy can be phased.
- Emergency UX impact: strongest practical fit because contacts can be prepared
  before the incident and escalation policy can vary by incident mode.
- Trust assumptions: clients encrypt correctly, contacts keep private keys
  secure, and any optional escrow mode is separately governed.
- Fit: recommended ultimate model.

## Recommended Ultimate Model

Default mode:

- The client creates media keys for each incident or stream.
- The client encrypts chunks before upload.
- The backend stores ciphertext chunks and metadata.
- The backend stores wrapped or encrypted copies of media keys for trusted contacts.
- The backend does not store raw media keys in the default mode.
- The incident viewer or a future trusted client performs decryption client-side where practical.

Optional future mode:

- A deployment may enable server escrow or break-glass key access for dead-man-switch and emergency-access cases.
- This mode must be disabled by default or configured separately from the normal ciphertext-only path.
- It must have explicit access policy, audit logging, rate limiting, operational warnings, and incident-review expectations.
- It may use deployment-specific key storage such as a KMS, HSM, locked local secret store, or another reviewed secret-management system.

This document decides the long-term direction: contact-wrapped keys plus client-side decryption should be the default production path, with server escrow or server-side decryption reserved for explicit break-glass modes.

The first production contact-wrapped implementation should follow
[contact-key-sharing-grants.md](contact-key-sharing-grants.md): access grants
must remain separate from decryption capability, wrapped-key records must remain
separate from viewer tokens, and the server must not store raw media keys,
contact private keys, plaintext, or unwrapped shared secrets in the default
path.

## Key Hierarchy

Future implementations should keep the hierarchy simple and auditable.

Suggested hierarchy:

- Device identity key: a long-lived client key pair controlled by the user's device or account model in a future client.
- Contact identity key: a long-lived public/private key pair controlled by each trusted contact.
- Incident key: an optional per-incident wrapping or coordination key for all streams in one incident.
- Stream media key: the symmetric key used to encrypt chunks in one media stream.
- Chunk nonce: a fresh nonce for each encrypted chunk under the relevant stream media key.
- Key ID: a non-secret identifier used to match encrypted chunks and wrapped keys with the correct decrypting key.
- Wrapped media key: an encrypted copy of an incident or stream key for a device, contact, recovery method, or escrow mode.
- Server escrow key: an optional future deployment key used only in explicit break-glass mode.

The current simulator uses one AES-256-GCM key for development/test chunks. A production client should prefer per-stream media keys so compromise or rotation can be contained to a smaller unit, especially for long-running or multi-media incidents.

Each encrypted chunk must use a unique nonce for its media key. Key IDs and non-secret envelope metadata may be stored in manifests and database rows. Raw media keys, contact private keys, escrow private keys, and plaintext must not be logged or placed in bundle manifests.

Future implementation must use stable, reviewed cryptographic libraries. Do not implement custom AEAD, block modes, padding, MACs, KDFs, random generators, public-key wrapping, threshold recovery, or secret-sharing primitives.

Initial production direction:

- Prefer one stream media key per stream.
- Use an optional incident-level key only if it simplifies rotation,
  late-contact enrollment, or multi-stream policy without widening compromise
  impact.
- Bind encrypted chunks to incident ID, stream ID, media type, and chunk index
  through authenticated metadata, as the current simulator envelope already
  does for development chunks.
- Treat key IDs, contact IDs, algorithm names, and public wrapping metadata as
  non-secret identifiers, but treat wrapped-key ciphertext as access-enabling
  metadata that must not be logged.
- Keep raw media keys and contact private keys in client or trusted-contact
  environments only, except for separately approved break-glass modes.

## Trusted Contact Access

Trusted contact access should be designed around pre-registration.

Possible flow:

1. A trusted contact generates or imports a public/private key pair.
2. The user verifies and registers the contact public key.
3. The recording client creates stream media keys during an incident.
4. The client uploads encrypted chunks and wrapped media keys for the selected trusted contacts.
5. The trusted contact receives an incident viewer token or future account-based access grant through a separate sharing path.
6. The viewer or future trusted contact app downloads ciphertext chunks, bundle manifests, and contact-wrapped key material.
7. The contact private key unwraps the media key and decrypts evidence client-side.

The viewer token should authorize read access to incident metadata and
encrypted evidence. It should not, by itself, be the only decryption capability
unless the system intentionally chooses a weaker bearer-token-only emergency
mode. Future account-owner, trusted-contact, and public-link grant rules are
tracked in [v1-access-control.md](v1-access-control.md).

Lost contact keys must be handled explicitly. If a contact loses their private key, existing media keys wrapped only to that contact may be unrecoverable by that contact. Future schema and API design should distinguish removing a contact from future incidents, stopping new key wrapping, revoking viewer tokens, rotating media keys, and marking older wrapped keys as no longer offered by the server.

## Browser Decryption Considerations

Browser decryption can help trusted contacts review evidence without installing
a native app, but it is not a complete end-to-end security answer when the same
backend serves the decrypting JavaScript.

Important constraints:

- URL fragment key delivery can keep raw key material out of normal HTTP
  requests, reverse-proxy paths, and server access logs.
- JavaScript delivered by the backend can still read URL fragments, imported
  keys, unwrapped media keys, and plaintext.
- A compromised backend can potentially serve malicious JavaScript even if the
  encrypted chunks and wrapped-key records are sound.
- Strict CSP, static assets, no-store responses, no inline script, signed or
  pinned viewer bundles, and offline verification tools can reduce risk, but
  they do not fully solve malicious-server risk.
- Browser decryption is stronger against passive storage compromise than
  active server compromise.
- Large encrypted ZIP bundles need careful parsing, streaming, cancellation,
  memory limits, and plaintext handling.

The browser path should follow [browser-decryption.md](browser-decryption.md)
and should not be implemented until the key custody, access-control, and
viewer trust model are accepted.

## Server Escrow And Break-Glass Considerations

Server-side key access may be acceptable only for explicit emergency,
dead-man-switch, or break-glass modes. It must not be introduced as an
incidental convenience for normal viewing.

Any future server-assisted mode needs:

- explicit deployment configuration, disabled by default or isolated from
  normal contact-wrapped access
- policy for dead-man-switch triggers, emergency escalation, cancellation, and
  false positives or false negatives
- authentication and authorization for account-owner, trusted-contact,
  admin/operator, and optional escrow roles
- audit logging that avoids raw tokens, raw keys, plaintext, and sensitive
  safety details
- rate limiting and abuse controls around key access and decrypted export
- operational warnings about the extra trust placed in the deployment
- incident-review expectations after break-glass access
- reviewed key storage choices such as KMS, HSM, a locked local secret store, or
  another deployment-specific secret-management system

See [break-glass-key-access.md](break-glass-key-access.md). Do not implement
server escrow until the policy, audit, deployment, and threat-model changes are
approved together.

## API And Storage Changes

The current API has owner-scoped contact public-key registration,
sharing-grant metadata routes, and grant-bound wrapped-key record storage and
delivery behind the authenticated main `/v1` boundary. It still has no
trusted-contact account model, browser decryption, backend decryption, or
server escrow path. Before iOS or production trusted-contact work starts,
future design should define:

- contact public-key registration, verification, replacement, and revocation
- device identity and recovery-key enrollment
- how clients choose, validate, and encode wrapping formats for server-stored
  records
- whether wrapped-key metadata ever appears in grant-scoped bundle manifests;
  current bundle manifests remain key-free and current delivery uses
  authenticated API responses
- how access-control grants interact with decryption capabilities
- retention, backup, restore, and deletion behavior for wrapped keys
- audit fields that are useful without exposing tokens, raw keys, plaintext, or
  sensitive safety data

Future custody, decryption, client, or bundle-manifest changes remain separate
implementation work. They should update this document,
[security-model.md](security-model.md), [threat-model.md](threat-model.md),
[encryption.md](encryption.md), and deployment guidance before or alongside
code changes.

## Metadata And Live Dashboard Implications

Media chunk encryption does not automatically protect all incident data. Incident IDs, stream IDs, media types, timestamps, byte counts, ciphertext hashes, stream state, and token-scoped summaries are visible to the backend today. Checkins can include location metadata.

Live GPS data may need a different privacy model from encrypted media chunks. A dashboard that shows current location to trusted contacts may require backend visibility, contact-side decryption, or a mixed design where coarse status is server-visible and sensitive details are encrypted.

Live audio/video streaming may also require a different key/session model than
completed chunk bundles. Long-running streams need key rotation, late contact
enrollment behavior, partial stream access, reconnect handling, and clear rules
for when wrapped keys are uploaded. The live or partial access boundary is
documented in
[live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md).

Interaction records may be non-emergency but still highly sensitive. Do not assume lower urgency means lower confidentiality.

## Threat Model Impacts

Future key custody work must consider:

- compromised backend or malicious viewer code
- compromised database or blob storage
- compromised viewer token
- malicious or compromised reverse proxy
- compromised trusted contact device
- destroyed or unavailable phone
- maintainer/operator misuse
- dead-man-switch false positives and false negatives
- accidental sharing/export of non-emergency interaction records

The hybrid model is designed to keep uploaded ciphertext useful after the phone is gone while limiting ordinary backend access to plaintext. Escrow modes increase backend trust requirements and must be explicit.

## Open Questions

- Should media encryption use per-stream media keys only, or a per-incident parent key plus per-stream keys?
- What exact public-key wrapping scheme should be used for contact-wrapped keys?
- How are trusted contact public keys verified during enrollment?
- What account model is required for web, iOS, and Android clients?
- How should contacts recover from lost private keys?
- Can contacts be added to an incident after recording has started, and which existing media keys should be wrapped for them?
- What metadata should be encrypted, and what must remain server-visible for the incident dashboard?
- Should browser decryption be the first contact UX, or should a native trusted contact app come first?
- Are signed/static viewer assets or reproducible builds needed before browser decryption is considered acceptable?
- Should server escrow be supported at all in the first production release, or deferred until after contact-wrapped keys are proven?
- What audit log fields are safe to store without leaking tokens, keys, plaintext, or sensitive safety data?
- What retention, backup, and deletion policies apply to wrapped keys?
- How do incident modes affect default sharing, retention, escalation, and access policies?

## Proposed Implementation Phases

Phase 1: design and docs.

Create this design, update the security and encryption documentation, and keep the current backend ciphertext-only.

Phase 2: protocol and incident-mode design.

Define mode-driven behavior, account/trusted-contact roles, and compatibility
expectations around the optional incident-mode, capture-profile,
escalation-policy, and sharing-state metadata fields before implementing public
client workflows.

Phase 3: contact-wrapped key prototype in the simulator.

Prototype media-key wrapping and bundle metadata in development flows only. Do
not add production server decryption. The simulator-only design is documented
in [contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md).

Phase 4: browser/client-side decrypt prototype.

Prototype viewer decryption with strict CSP, no-store behavior, and a clear explanation of malicious-server limitations, following the constraints in [browser-decryption.md](browser-decryption.md).

Phase 5: iOS/Android keychain and contact-key planning.

Design client key generation, platform key storage, contact public-key enrollment, rotation, and revocation before implementing production mobile clients.

Phase 6: emergency access and dead-man-switch key policy.

Define trigger behavior, access policy, audit expectations, notification, cancellation, and false-positive/false-negative handling. See [break-glass-key-access.md](break-glass-key-access.md).

Phase 7: optional server escrow or break-glass implementation.

Implement only if explicitly accepted. Keep it separately configured, audited/logged, rate-limited, and documented with deployment warnings.
