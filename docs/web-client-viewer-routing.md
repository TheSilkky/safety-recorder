# Web-Client Viewer Routing

Status: routing decision and implementation plan. This document does not change
runtime behavior, deploy the web client, remove current server routes, change
CORS or CSRF behavior, implement browser decryption, or approve broad public
`/v1` exposure.

## Decision

Proofline has no current public production deployments that require preserving
the built-in incident viewer or pre-rename viewer aliases as public
compatibility surfaces.

The future canonical no-account viewer link should point at the web-client
origin, not at the server-rendered built-in viewer:

```text
https://<web-client-origin>/viewer#token=<raw-viewer-token>
```

The raw viewer token stays a bearer secret. It belongs in the URL fragment
only for link handoff to the web client. The fragment is not sent to the
web-client hosting server in the HTTP request path, query string, or
`Referer` header. After the web client reads the token, it should remove the
fragment from the visible URL with browser history replacement and keep the
token only in memory for the current viewer session unless a separately
reviewed client storage design says otherwise.

If the server later creates complete viewer links instead of returning only the
raw token at creation time, those generated links should use the configured
web-client origin and the fragment shape above. The server should not generate
new `/i/{token}` or `/e/{token}` links for normal v1 preview sharing.

## Current Server Routes

The current backend still implements these token-scoped read-only routes on the
main API/viewer listener:

- `GET /i/{token}`
- `GET /i/{token}/data`
- `GET /i/{token}/viewer-payload`
- `GET /i/{token}/streams/{stream_id}/download`
- `GET /i/{token}/incident/download`
- legacy `/e/{token}` aliases for the same viewer behavior

These routes remain current implementation details until a later route-removal
or redirect issue changes runtime behavior. They are not the long-term
canonical viewer surface.

`GET /i/{token}/viewer-payload` is the backend primitive intended for the
future web-client no-account viewer. Completed encrypted ZIP download routes
may remain backend primitives for a web-client viewer, subject to reviewed
edge routing, no-store headers, rate limits, and token-redaction controls.

`GET /i/{token}` should not remain a long-lived public compatibility viewer
after the web-client viewer exists. A later implementation issue should choose
one of these explicit production behaviors:

- remove it from public deployment paths and return a safe not-found response;
- keep it only as a clearly configured local-development fallback; or
- replace it with a minimal redirect/shim to the configured web-client viewer
  origin after reviewing `Location` header handling, proxy logs, no-store
  headers, and token leakage.

The legacy `/e/{token}` aliases should not be used for new links. Keep them
only for explicit local/test compatibility until a later issue removes them.
Do not design a long-lived `/e` migration window without a concrete current
migration need.

## Web-Client Access To Backend Data

The web client should exchange the fragment token for token-scoped backend data
only through reviewed routes. The current server primitive is
`GET /i/{token}/viewer-payload`; any future token exchange route must preserve
the same public safety properties:

- read-only behavior;
- no account, admin, grant, trusted-contact, write, notification, dispatch, or
  decryption capability from the bearer token alone;
- indistinguishable public failures for missing, invalid, expired, and revoked
  tokens;
- no raw viewer token, token hash, session token, Authorization header,
  request body, uploaded bytes, plaintext, raw key, wrapped-key ciphertext,
  stored path, object key, private deployment detail, backend diagnostic, or
  user safety narrative in responses.

If the web-client origin and API origin are different, viewer payload fetches
need an explicit origin decision before deployment. Prefer a reviewed
same-origin reverse-proxy shape for the web-client origin. If CORS is used
instead, allow only exact reviewed web-client origins, keep the no-account
viewer request credentialless, and never use wildcard credentialed CORS.

Navigation, map-provider links, analytics, crash reports, monitoring labels,
and support exports must not receive viewer tokens. Map-provider links must not
include viewer tokens unless a later route design explicitly requires and
reviews that behavior.

## Browser And Edge Requirements

For any deployed no-account web-client viewer:

- serve the web-client origin and backend token routes over HTTPS;
- set `Referrer-Policy: no-referrer` on web-client viewer pages and backend
  token responses;
- set `Cache-Control: no-store` on token-bearing pages, JSON, errors, redirects
  or shims, and encrypted ZIP downloads;
- keep `X-Content-Type-Options: nosniff`, a restrictive
  `Content-Security-Policy`, clickjacking protection, and a restrictive
  `Permissions-Policy` on browser-facing viewer responses;
- set HSTS at the HTTPS edge only after TLS is reliable for the public
  hostname;
- keep app-level public viewer rate limits aligned with edge route groups;
- redact or drop raw request paths, query strings, response `Location` headers
  if redirects are used, rate-limit keys, metric labels, and analytics events
  that could contain viewer tokens;
- avoid browser-history and screenshot leakage by removing the token fragment
  from the visible URL immediately after the web client reads it.

These controls do not make broad `/v1` exposure public-ready. They apply only
to the reviewed no-account viewer route group.

## Test Expectations For Later Runtime Changes

This issue records the routing decision. A later implementation issue that
removes, redirects, or gates current built-in viewer routes should update tests
to cover:

- canonical web-client link construction, including fragment placement and no
  query-string token;
- safe behavior for `GET /i/{token}` after the built-in viewer is no longer
  canonical;
- removal or explicit local/test-only handling for `/e/{token}` aliases;
- no-store and no-referrer headers on token-bearing responses, errors, and any
  redirect/shim responses;
- public viewer rate-limit classification for retained token routes;
- log redaction for retained `/i`, `/e`, token exchange, and download routes;
- indistinguishable public errors for missing, invalid, expired, and revoked
  viewer tokens;
- no public routing for `/v1`, `/admin/api/...`, `/admin`, private diagnostics,
  admin/operator workflows, backend decryption, browser decryption, raw-key,
  escrow, or break-glass routes.
