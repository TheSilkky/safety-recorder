# One-off Codex Context: Issue #3 Deployment Examples Preflight

Historical/reference-only context prompt.

Use this prompt as context before running `codex/prompts/70-work-on-github-issue.md` for GitHub issue #3.

This prompt is a preflight/context pass only.

Do **not** change files.
Do **not** create commits.
Do **not** create a pull request.
Do **not** create or close GitHub issues.
Do **not** draft final documentation yet.
Do **not** add deployment examples yet.

## Goal

Prepare a concise implementation context packet for issue #3:

```text
Add Reverse Proxy And WireGuard Deployment Examples
```

The next prompt, `codex/prompts/70-work-on-github-issue.md`, will use this context packet to perform the actual scoped documentation work.

This preflight should clarify the intended deployment documentation shape before any edits are made.

## Repository and issue

Repository:

```text
TheSilkky/safety-recorder
```

Issue:

```text
#3 Add Reverse Proxy And WireGuard Deployment Examples
```

Issue intent:

- add practical deployment documentation
- preserve the private/public listener split
- show how to expose only the public emergency viewer through HTTPS
- show how to keep private `/v1` reachable only through localhost, LAN, WireGuard, firewall rules, or another private boundary
- include token-path logging warnings for `/e/{token}`
- include timeout coordination notes for bundle downloads
- avoid production-readiness claims

## First steps

Check repository state and branch:

```bash
git status --short --branch --untracked-files=all
git branch --show-current
git log --oneline -5
```

Read the issue:

```bash
gh issue view 3 --repo TheSilkky/safety-recorder
```

If GitHub CLI is unavailable, say so and continue from local files.

Read current source-of-truth files:

```bash
sed -n '1,220p' README.md
sed -n '1,240p' AGENTS.md
sed -n '1,220p' SECURITY.md
sed -n '1,260p' docs/README.md
sed -n '1,260p' docs/deployment.md
sed -n '1,260p' docs/configuration.md
sed -n '1,260p' docs/security-model.md
sed -n '1,260p' docs/threat-model.md
sed -n '1,220p' docs/api.md
sed -n '1,260p' codex/README.md
```

Do not rely on stale assumptions from this prompt if current repository files disagree.

## Current non-negotiable constraints

Preserve these constraints unless current source-of-truth docs say otherwise:

- Safety Recorder is experimental and not production-ready public infrastructure.
- The private `/v1` API has no public user authentication.
- Do not expose private `/v1` publicly.
- Private `/v1` routes and public emergency viewer routes must remain on separate listener groups and muxes.
- Public emergency viewer routes must remain read-only.
- Evidence bundles are encrypted chunk bundles, not decrypted, playable, or merged media exports.
- Emergency viewer URLs contain bearer tokens and must be treated as secrets.
- Do not log raw emergency tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, or future token-like values.
- Do not introduce backend decryption, browser decryption, raw server-held keys, key escrow, key sharing, OAuth, JWT, user accounts, SMS, Messenger, push notifications, public admin dashboards, Docker Compose, Kubernetes, or cloud integrations.
- Do not claim production readiness.
- Do not weaken existing security warnings.
- New future work discovered during this context pass should become a note for the maintainer or a future backlog item, not an implementation change.

## Reverse proxy example policy for issue #3

Use **Traefik** as the only concrete reverse-proxy example for issue #3.

Do not add concrete examples for:

- NGINX
- Caddy
- HAProxy
- cloud load balancers
- Kubernetes ingress
- Docker Compose

It is acceptable to mention that equivalent controls can be adapted to other reverse proxies, but this repository should not maintain examples for them in this issue.

The maintainer has sysadmin and Traefik experience and will manually review all Traefik configuration details before accepting the PR. Treat Traefik examples as draft documentation until maintainer review.

If a Traefik setting is uncertain, do not invent a confident configuration. Add a maintainer-review note instead.

## Docker example policy

Issue #3 may include Docker examples, but must not add Docker Compose.

Prefer examples based on:

- `docker build`
- `docker run`
- localhost-only port publishing
- Docker networks, if needed
- Traefik Docker labels only if they remain readable and do not require sensitive data in labels
- static/dynamic Traefik configuration files only if they are documentation snippets, not committed deployment infrastructure

Do not add repository deployment files such as:

- `docker-compose.yml`
- Kubernetes manifests
- cloud deployment templates
- production Traefik config files intended to be used as-is

Documentation examples must not include real domains, real tokens, secrets, private deployment details, user safety data, raw keys, or environment-specific infrastructure.

## Issue #3 scope boundary

For issue #3, document deployment shape only.

In scope for issue #3:

- localhost-only Docker run pattern
- private `/v1` bound or published only to localhost/private network
- public emergency viewer exposed through HTTPS via Traefik
- WireGuard/private-network guidance for private API access
- token-path access-log warning for `/e/{token}`
- timeout coordination notes for large encrypted ZIP bundle downloads
- clear statement that `/v1` remains private and unauthenticated unless a separate authenticated control plane is added in the future
- clear statement that these examples do not make the project production-ready

Out of scope for issue #3:

- rate-limiting configuration, except a short forward reference to issue #4
- app-level authentication
- app-level rate limiting
- OAuth
- JWT
- user accounts
- SMS
- push notifications
- public admin dashboard
- Docker Compose
- Kubernetes
- cloud-specific infrastructure
- backend decryption
- browser decryption
- key escrow
- key sharing
- playable media export

## Relationship to issue #4

Issue #3 should come before issue #4 because issue #3 defines the deployment shape that issue #4 can later attach rate-limiting guidance to.

For issue #3, include at most a short note such as:

```text
Rate-limiting and abuse-control examples are tracked separately in issue #4.
```

Do not add Traefik rate-limit middleware configuration in issue #3.

Issue #4 should later build on the route groups and Traefik deployment shape documented by issue #3.

## Expected documentation shape

The actual implementation prompt may decide the final structure after reading current docs, but the likely shape is:

- update `docs/deployment.md` as the primary document
- update `docs/configuration.md` only if bind-address or timeout examples need clarification
- update `docs/security-model.md` or `docs/threat-model.md` only if current security assumptions or known gaps need a small consistency update
- update `README.md` only if top-level links or summary wording become stale
- update `CHANGELOG.md` only when the actual documentation change is made

Do not edit any of those files during this preflight.

## Context packet output

Return a concise context packet for the next prompt with these sections:

1. `Issue summary`
2. `Current deployment/security facts from source-of-truth docs`
3. `Recommended issue #3 scope`
4. `Traefik-only example policy`
5. `Docker/no-Compose boundary`
6. `WireGuard/private-network guidance to preserve`
7. `Token-path logging warning to preserve`
8. `Timeout coordination guidance to preserve`
9. `Files likely affected by the later implementation`
10. `Files/areas that must not change`
11. `Validation expected for the later implementation`
12. `Recommended next prompt`

For `Recommended next prompt`, say:

```text
codex/prompts/70-work-on-github-issue.md
```

Do not produce final docs. Do not patch files. Do not write the implementation diff.

## Validation for this preflight

Because this is context-only:

- no Go tests are required
- no Markdown files should be changed
- no validation commands need to be run beyond reading files and checking repository state
- if any file changes occurred accidentally, stop and report that as an error
