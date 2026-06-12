# v1 Preview Readiness Checklist

This checklist is the release gate for any Proofline `v1 preview`, `v1.0.0`,
or real-user evidence-upload readiness claim. It is documentation and process
guidance only. It does not change runtime behavior, run `gh issue create`,
create deployment automation, add CI enforcement, implement client features,
or approve public production deployment.

Read this checklist with [v1 preview direction](v1-preview-direction.md).
That document explains the product direction and current-versus-future
boundaries. This checklist answers a narrower release question: whether the
current repository state can be described as ready for v1 preview use.

Proofline is not production-ready public infrastructure until deployment
hardening, route exposure review, operational validation, logging review, and
release validation actually land. Passing ordinary build and test commands is
not enough to make that claim.

## When To Run This Gate

Run this checklist before:

- tagging or publishing a release that says `v1 preview` or `v1.0.0`
- publishing docs, reports, release notes, or PR text that say real users can
  upload sensitive evidence to a preview deployment
- enabling broad public routes or web-client deployment language for preview
- closing a v1-preview release-prep PR as ready

If any hard blocker below is incomplete, do not claim v1 preview readiness.
Use language such as `pre-v1`, `experimental`, `planning`, or `internal test`
instead.

## Hard Blockers

Every hard blocker must be satisfied before a v1 preview readiness claim.
Open issues listed here are pointers to existing tracked work, not permission
to implement them inside an unrelated release-gate task.

| Area | Required before v1 preview | Source | Tracked issue |
|---|---|---|---|
| Post-quantum default runtime support | The accepted post-quantum envelope is implemented, documented, tested, and default for preview evidence upload, wrapped-key delivery, and review paths. Legacy or simulator envelopes are clearly non-preview defaults. | [post-quantum envelope](post-quantum-envelope.md), [encryption](encryption.md) | [#246](https://github.com/open-proofline/server/issues/246), [#234](https://github.com/open-proofline/server/issues/234) |
| Web-client viewer replacement | The built-in server-rendered viewer is no longer the v1 preview review surface. The web-client replacement and backend token/data primitives have a reviewed contract. | [v1 preview direction](v1-preview-direction.md), [contacts and viewer replacement](contacts-and-viewer-replacement.md) | [#223](https://github.com/open-proofline/server/issues/223), [#221](https://github.com/open-proofline/server/issues/221), [#222](https://github.com/open-proofline/server/issues/222) |
| Browser crypto and decryption boundary | Browser encryption/decryption trust limits, static asset integrity, plaintext handling, key handling, CSP/cache behavior, and malicious-server caveats are accepted before any browser decrypt preview claim. | [browser decryption](browser-decryption.md), [key custody](key-custody.md) | [#233](https://github.com/open-proofline/server/issues/233) |
| Browser recording scope | Browser capture is explicitly scoped as secondary/fallback capture with permission, staging, retry, backgrounding, crash, and token-storage limits documented. It is not described as guaranteed background mobile capture. | [v1 preview direction](v1-preview-direction.md) | Not yet represented by a dedicated server issue; do not auto-file a duplicate from this checklist. |
| Recipient keys and trusted-contact access | Account, device, and trusted-contact recipient keys, invite/accept flows, grant-scoped delivery, and CEK rewrapping behavior are implemented or explicitly scoped for the preview surface being claimed. | [contacts and viewer replacement](contacts-and-viewer-replacement.md), [contact key sharing](contact-key-sharing-grants.md), [key custody](key-custody.md) | [#230](https://github.com/open-proofline/server/issues/230), [#224](https://github.com/open-proofline/server/issues/224), [#225](https://github.com/open-proofline/server/issues/225), [#226](https://github.com/open-proofline/server/issues/226) |
| Capture stream variants and evidence preservation | Capture stream group, concrete variant, source-timeline, encrypted context, and supersession behavior needed for preview evidence claims are implemented or clearly excluded from the specific preview scope. | [capture stream variants](capture-stream-variants.md), [v1 preview direction](v1-preview-direction.md) | No open implementation issue was confirmed during the #249 gate work; re-check before filing anything new. |
| Encrypted location and context | GPS/location/context behavior is encrypted or minimized according to the accepted design. Full GPS, private notes, route history, and safety state are not exposed in server, edge, log, issue, or public viewer metadata by accident. | [encrypted location context](encrypted-location-context.md), [v1 preview direction](v1-preview-direction.md), [security model](security-model.md) | [#231](https://github.com/open-proofline/server/issues/231) |
| Account quota and abuse controls | Public preview deployments have account-scoped committed blob quota, upload staging controls where needed, rate limits, and deployment-edge abuse controls. Quota is treated as abuse and cost control, not billing. | [v1 preview direction](v1-preview-direction.md), [deployment](deployment.md) | [#236](https://github.com/open-proofline/server/issues/236), [#247](https://github.com/open-proofline/server/issues/247) |
| Deployment exposure review | The exact public route set, TLS/HSTS edge, reverse-proxy rules, token-path redaction, credentialed CORS/CSRF behavior, WAF/rate limits, backups, restore drills, and monitoring have been reviewed for the preview deployment. Public edges must not route `/admin`, `/admin/...`, `/admin/api/...`, operator diagnostics, raw-key, escrow, decryption, or private support routes. | [public web-client deployment boundary](public-web-client-deployment-boundary.md), [deployment](deployment.md), [public API listener split](public-api-listener-split.md), [security model](security-model.md) | [#248](https://github.com/open-proofline/server/issues/248), [#251](https://github.com/open-proofline/server/issues/251) |
| Logging review | Runtime, proxy, edge, worker, operator, email, storage, upload, token, and browser/decrypt-adjacent logs have been reviewed against the logging requirements. Raw tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, wrapped-key ciphertext, private deployment details, stored paths, object keys, location detail, and user safety data are not logged. | [logging requirements](logging-requirements.md), [security model](security-model.md) | [#252](https://github.com/open-proofline/server/issues/252) |
| Private-admin listener separation | Main API/viewer and private-admin listener separation is preserved in code, docs, deployment examples, and reverse-proxy guidance. Public viewer routes remain read-only. | [public API listener split](public-api-listener-split.md), [architecture](architecture.md), [security model](security-model.md) | Covered by release review; create a narrow issue only if a concrete gap is found. |

## Non-Goals For This Gate

These items must not be added merely because this checklist is being run:

- backend decryption in the normal path
- raw server-held CEKs, raw media keys, recipient private keys, or plaintext
- default key escrow
- emergency-services integration or guaranteed emergency response
- public admin dashboard
- broad unreviewed public `/v1` exposure
- treating Cloudflare or another edge provider as a confidentiality boundary
- plaintext GPS/location relay metadata
- playable decrypted media exports
- payment providers, subscriptions, account plans, or hosted entitlements as
  default v1 preview requirements
- SMS, Messenger, push notifications, or notification delivery as incidental
  behavior
- Docker Compose, Kubernetes, Terraform, provider-specific deployment
  automation, or cloud integrations as incidental server work

## Optional Hosted-Service Work

Hosted-service work can be useful for a future official Proofline service, but
it is not a default v1 preview blocker unless a release explicitly claims a
hosted public service. Keep these separate from the default preview gate:

- Stripe or other billing-provider integration
- hosted entitlements, plans, subscriptions, or payment webhooks
- provider-specific deployment automation
- hosted support workflows
- community-service edge patterns that do not apply to sensitive evidence
  capture

If hosted-service work becomes part of a specific preview, require a separate
issue, design, threat-model review, deployment review, and release note.

## Issue Hygiene

This checklist points to open issues where blockers are already tracked. Do not
create duplicate issues just because a blocker appears here.

Before creating any new public issue from this gate:

1. Search open and closed issues for the blocker.
2. Check whether the item belongs in `open-proofline/server` or a companion
   repository such as `open-proofline/web-client`.
3. Confirm the issue can be public under [SECURITY](../SECURITY.md).
4. Keep sensitive vulnerability details, exploit details, raw tokens, secrets,
   private deployment details, and user safety data out of public issue text.
5. Prefer one narrow issue with clear acceptance criteria over a broad
   "finish v1" umbrella issue.

Backlog scanning and draft issue creation remain local-first. This checklist
must not run `gh issue create` or execute a generated issue-creation script.

## Release Decision

Use one of these outcomes after running the gate:

- `Ready for v1 preview claim`: every hard blocker is satisfied, relevant
  release validation passed, and deployment/operational review is documented.
- `Not ready for v1 preview claim`: at least one hard blocker is incomplete.
  Release notes and PR text must say `pre-v1`, `experimental`, `planning`, or
  equivalent limited language.
- `Ready for ordinary pre-v1 release`: ordinary release validation passed, but
  one or more v1 preview blockers remain incomplete. Do not use v1 preview
  readiness language.

Record the decision in release notes, PR body, or the release-check output.
