# One-off Codex Context: Issue #4 Rate-Limiting Guidance Preflight

Historical/reference-only context prompt.

Use this prompt as context before running `codex/prompts/70-work-on-github-issue.md` for GitHub issue #4.

This prompt is a preflight/context pass only.

Do **not** change files.
Do **not** create commits.
Do **not** create a pull request.
Do **not** create or close GitHub issues.
Do **not** draft final documentation yet.
Do **not** add rate-limiting examples yet.
Do **not** modify Traefik configuration snippets yet.

## Goal

Prepare a concise implementation context packet for issue #4:

```text
Add Rate Limiting Guidance And Proxy Examples
```

The next prompt, `codex/prompts/70-work-on-github-issue.md`, will use this context packet to perform the actual scoped documentation work.

This preflight should clarify how issue #4 should build on the issue #3 deployment examples before any edits are made.

## Repository and issue

Repository:

```text
TheSilkky/safety-recorder
```

Issue:

```text
#4 Add Rate Limiting Guidance And Proxy Examples
```

Issue intent:

- document deployment-edge rate-limiting guidance
- build on the reverse-proxy deployment shape from issue #3
- use Traefik as the only concrete proxy example
- describe route groups that should have different rate limits
- warn against logging raw `/e/{token}` paths while implementing limits
- explain that app-level rate limiting is still absent
- keep security-sensitive details high-level and suitable for a public issue
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
gh issue view 4 --repo TheSilkky/safety-recorder
```

If GitHub CLI is unavailable, say so and continue from local files.

Read current source-of-truth files:

```bash
sed -n '1,220p' README.md
sed -n '1,240p' AGENTS.md
sed -n '1,220p' SECURITY.md
sed -n '1,260p' docs/README.md
sed -n '1,320p' docs/deployment.md
sed -n '1,260p' docs/configuration.md
sed -n '1,320p' docs/security-model.md
sed -n '1,320p' docs/threat-model.md
sed -n '1,320p' docs/api.md
sed -n '1,260p' codex/README.md
```

Do not rely on stale assumptions from this prompt if current repository files disagree.

## Prerequisite check: issue #3

Issue #4 should build on the deployment shape from issue #3.

Before recommending issue #4 implementation work, check whether the current branch already includes documentation for:

- localhost-only Docker deployment
- WireGuard or private-network `/v1` access
- Traefik HTTPS exposure for the public emergency viewer
- explicit warning that `/v1` remains private and unauthenticated
- token-path logging warnings for `/e/{token}`
- timeout coordination for encrypted ZIP bundle downloads
- a note that rate-limiting and abuse-control examples are tracked separately in issue #4

If those issue #3 docs are not present in the current branch, report that issue #3 appears to be a prerequisite and recommend merging or rebasing onto the issue #3 documentation before implementing issue #4. Do not edit files during this preflight.

## Current non-negotiable constraints

Preserve these constraints unless current source-of-truth docs say otherwise:

- Safety Recorder is experimental and not production-ready public infrastructure.
- The private `/v1` API has no public user authentication.
- Do not expose private `/v1` publicly.
- Private `/v1` routes and public emergency viewer routes must remain on separate listener groups and muxes.
- Public emergency viewer routes must remain read-only.
- Evidence bundles are encrypted chunk bundles, not decrypted, playable, or merged media exports.
- Emergency viewer URLs contain bearer tokens and must be treated as secrets.
- Rate limiting in issue #4 is a deployment-edge control, not an application feature.
- Do not implement app-level rate limiting in this issue.
- Do not log raw emergency tokens, request bodies, uploaded bytes, Authorization headers, plaintext, raw keys, or future token-like values.
- Do not introduce backend decryption, browser decryption, raw server-held keys, key escrow, key sharing, OAuth, JWT, user accounts, CAPTCHA, SMS, Messenger, push notifications, public admin dashboards, Docker Compose, Kubernetes, or cloud integrations.
- Do not claim production readiness.
- Do not weaken existing security warnings.
- New future work discovered during this context pass should become a note for the maintainer or a future backlog item, not an implementation change.

## Reverse proxy and rate-limiting policy for issue #4

Use **Traefik** as the only concrete reverse-proxy and rate-limiting example for issue #4.

Do not add concrete examples for:

- NGINX
- Caddy
- HAProxy
- cloud load balancers
- Kubernetes ingress
- Docker Compose

It is acceptable to mention that equivalent controls can be adapted to other reverse proxies, but this repository should not maintain examples for them in this issue.

The maintainer has sysadmin and Traefik experience and will manually review all Traefik configuration details before accepting the PR. Treat Traefik rate-limiting examples as draft documentation until maintainer review.

If a Traefik setting, middleware option, or route pattern is uncertain, do not invent a confident configuration. Add a maintainer-review note instead.

## Docker example policy

Issue #4 may refer to the Docker deployment shape from issue #3, but must not add Docker Compose.

Prefer examples based on:

- the existing bare-metal or host-level Traefik/file-provider shape from issue #3
- Traefik file-provider dynamic configuration snippets
- route-group examples that are readable and reviewable
- placeholders that are clearly marked as examples, not production defaults

Do not add repository deployment files such as:

- `docker-compose.yml`
- Kubernetes manifests
- cloud deployment templates
- production Traefik config files intended to be used as-is

Documentation examples must not include real domains, real tokens, secrets, private deployment details, user safety data, raw keys, or environment-specific infrastructure.

## Issue #4 scope boundary

For issue #4, document rate-limiting guidance only.

In scope for issue #4:

- explain that the Go app still has no built-in app-level rate limiter
- explain that rate limiting should be applied at the deployment edge for now
- build on the issue #3 Traefik reverse-proxy shape
- describe separate route groups that may need different limits
- document Traefik-only example middleware or pseudo-config, subject to maintainer review
- warn that token-bearing `/e/{token}` paths must not be logged while implementing rate limiting
- keep all examples public-safe and high-level
- update docs consistently if the known gaps or deployment guidance changes

Out of scope for issue #4:

- implementing Go middleware or app-level rate limiting
- changing HTTP handlers
- changing route behaviour
- adding authentication
- adding OAuth
- adding JWT
- adding user accounts
- adding CAPTCHA
- adding SMS or push notifications
- adding public admin dashboards
- adding Docker Compose
- adding Kubernetes
- adding cloud-specific infrastructure
- adding backend decryption
- adding browser decryption
- adding key escrow
- adding key sharing
- adding playable media export
- claiming production readiness

## Route groups to identify during preflight

Inspect `docs/api.md`, current HTTP handlers if needed, and `docs/deployment.md` before making recommendations.

Identify the current route groups by documented behaviour, not guesswork.

Expected groups may include:

- public emergency viewer HTML pages
- public emergency viewer JSON polling/data routes
- public encrypted stream or incident bundle download routes
- public static assets
- private `/v1` chunk upload routes
- private `/v1` incident, stream, check-in, token, and admin-style routes

Do not assume exact route names if the current docs or handlers use different names. Report the route groups using the names and paths that exist in the current repository.

## Rate-limit guidance shape

The later issue #4 implementation should prefer guidance like:

- separate limits by route group
- stricter limits for token lookup and metadata polling than static assets
- download-aware limits for encrypted ZIP bundle routes so legitimate emergency downloads are not cut off
- upload-aware limits for private `/v1` chunk upload routes if they are routed through a private Traefik boundary
- explicit note that private `/v1` should remain private even when rate limited
- explicit note that rate limiting does not replace authentication or proper deployment boundaries
- explicit note that exact numbers must be tuned to deployment needs

This preflight should not choose final numeric limits. It may recommend that the later documentation use illustrative placeholder values only if they are clearly marked for maintainer review.

## Token-path logging warning

Issue #4 must preserve the logging warning from issue #3.

Rate-limiting examples must not make token logging worse.

The later implementation should warn that:

- `/e/{token}` paths contain bearer tokens
- reverse proxies may log raw paths before requests reach the Go server
- redacting headers is not enough because the token is in the URL path
- access logs for token-bearing routers should drop, sanitize, or avoid raw request path fields
- rate-limiting middleware, metrics, dashboards, and logs should not expose raw tokens

Do not add real tokens, synthetic token-looking values, or private deployment details to docs.

## Expected documentation shape

The actual implementation prompt may decide the final structure after reading current docs, but the likely shape is:

- update `docs/deployment.md` as the primary document
- update `docs/security-model.md` if the known rate-limiting gap wording needs to distinguish app-level absence from deployment-edge guidance
- update `docs/threat-model.md` if current next-step wording becomes stale
- update `docs/api.md` only if route grouping references need a small cross-reference
- update `README.md` only if top-level known limitation or deployment wording becomes stale
- update `CHANGELOG.md` only when the actual documentation change is made

Do not edit any of those files during this preflight.

## Context packet output

Return a concise context packet for the next prompt with these sections:

1. `Issue summary`
2. `Issue #3 prerequisite status`
3. `Current deployment/security facts from source-of-truth docs`
4. `Recommended issue #4 scope`
5. `Traefik-only rate-limiting policy`
6. `Docker/no-Compose boundary`
7. `Route groups to document later`
8. `Token-path logging warning to preserve`
9. `Rate-limit guidance shape`
10. `Files likely affected by the later implementation`
11. `Files/areas that must not change`
12. `Validation expected for the later implementation`
13. `Recommended next prompt`

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
