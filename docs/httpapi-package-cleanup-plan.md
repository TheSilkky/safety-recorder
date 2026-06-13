# Staged internal/httpapi Cleanup Plan

Status: planning note for issue #332. This document does not move code, change
routes, change templates, change authentication, change browser headers, or
change private/public listener behavior.

## Goal

`internal/httpapi` is intentionally the route and handler boundary for the
server, but it now contains many concerns in one package: API construction,
auth, admin web, viewer pages, bundle generation, upload handling, relay
coordination, sharing metadata, deletion workflows, response helpers,
middleware, and tests.

The cleanup goal is to make future changes smaller and easier to review without
weakening the security boundary. The core rule is that the main API/viewer mux
and the private-admin mux stay separate, and any package extraction must keep
route registration decisions in `internal/httpapi` unless a later issue proves a
narrower boundary is safer.

## Current File Inventory

### API Construction And Route Wiring

- `api.go`
- `routes.go`
- `metadata_repository.go`
- `doc.go`

These files should remain in `internal/httpapi` until the `API` type and route
registration dependencies are smaller. `routes.go` is the source of truth for
the main API/viewer and private-admin mux split, so moving it too early would
increase route-boundary risk.

### Middleware, Responses, And Logging

- `response.go`
- `middleware.go`
- `logging.go`
- `auth_middleware.go`
- `auth_context.go`
- `authorization.go`
- `main_rate_limit.go`
- `public_rate_limit.go`
- related middleware, logging, authorization, and rate-limit tests

These are good candidates for early file-level cleanup and later helper
extraction, but only where the extracted code has no dependency on `API` route
wiring or repository access. Response helpers are likely the lowest-risk first
slice. Route classification, request redaction, and auth wrappers need extra
care because they protect token paths and listener separation.

### Account, Auth, And Second-Factor Handlers

- `auth_handlers.go`
- `registration_handlers.go`
- `second_factor_handlers.go`
- `totp_second_factor_handlers.go`
- `webauthn_second_factor_handlers.go`
- `web_auth.go`
- `web_cors.go`
- related auth and account tests

These handlers depend on `API`, the metadata repository, session issuance,
CSRF behavior, and second-factor state. Keep them in `internal/httpapi` until
smaller auth helper boundaries are identified. Do not use this cleanup to
change login, registration, browser-cookie, CSRF, TOTP, WebAuthn, or email
challenge semantics.

### Private Admin API And Private Admin Web

- `private_handlers.go`
- `legacy_reassignment_handlers.go`
- `deletion_handlers.go`, for admin-global deletion paths
- `admin_web.go`
- `assets.go`
- `web/templates/admin.html`
- `web/admin/static/styles.css`
- related admin web, deletion, and reassignment tests

These files must preserve the private-admin listener boundary. Admin JSON
routes and `/admin` web routes are registered only from `adminRoutes()`. Any
cleanup that changes file layout, embedded assets, or helper names must retain
`/admin`, `/admin/api/...`, and `/admin/static/...` on the private-admin mux
only.

### Incident, Stream, Upload, And Bundle Handlers

- `chunk_handlers.go`
- `incident_helpers.go`
- `streams.go`
- `upload.go`
- `upload_coordination.go`
- `idempotency.go`
- `bundles.go`
- `bundle_manifest.go`
- `bundle_zip.go`
- related upload, stream, bundle, and idempotency tests

Bundle manifest and ZIP helpers are strong extraction candidates because they
can be tested around explicit inputs and outputs. Upload and stream handlers
should stay in `internal/httpapi` until helper APIs avoid exposing stored
paths, object keys, uploaded bytes, plaintext, raw keys, or repository internals
through a wider package boundary.

### Viewer And Static Assets

- `incident_viewer.go`
- `web/templates/incident_viewer.html`
- `web/static/styles.css`
- `web/static/scripts.js`
- related viewer tests

The public viewer remains read-only and token-scoped. Static asset serving can
be organized, but viewer routes must stay separated from private-admin routes,
must keep no-store behavior for token-protected responses, and must not become
a general public web-client implementation.

### Relay And Sharing Handlers

- `relay_sessions.go`
- `relay_core.go`
- `account_recipient_key_handlers.go`
- `trusted_contact_relationship_handlers.go`
- `sharing_handlers.go`
- `wrapped_key_handlers.go`
- related relay, sharing, contact, and wrapped-key tests

These handlers sit on current main `/v1` routes and depend on auth,
authorization, repository methods, and response helpers. Keep route ownership in
`internal/httpapi`; split only pure validation or formatting helpers when doing
so does not change key custody, wrapped-key delivery, service identity, or
relay authorization behavior.

### Test Support

- `test_helpers_test.go`
- package-local `*_test.go` files
- smoke tests such as `s3_deletion_smoke_test.go`

Test fixture cleanup should happen before or alongside package extraction so
future helpers can be tested without copying large `API` setup blocks. Keep
route-boundary tests close to `routes.go`.

## Likely Dependency Risks

- `API` currently holds configuration, repository, storage, coordination,
  session, security-header, and template dependencies. Packages that import
  `internal/httpapi` helpers should not also need an `API` pointer.
- Handler helpers often rely on shared response shapes, auth principal loading,
  repository interfaces, and safe logging conventions. Moving one helper can
  create cycles if it imports handler types while handlers import the new
  package.
- Bundle and upload helpers are safer to extract when the boundary accepts
  explicit metadata structs and `io.Reader`/`io.Writer` style dependencies
  rather than handler or repository types.
- Route classification and logging helpers protect sensitive path redaction.
  Extracting them without route-surface tests can create false confidence or
  accidental raw path logging.
- Embedded template and static asset movement can silently change paths or
  headers if the new package owns HTTP handlers instead of file-system helpers.

## Route-Boundary Risks To Check

Every cleanup PR that touches route registration, middleware, web assets,
templates, auth, or handlers must verify:

- `/admin`, `/admin/...`, `/admin/api/...`, and `/admin/static/...` remain on
  the private-admin handler only.
- The main API/viewer handler does not mount private admin routes.
- Public incident viewer routes remain read-only.
- Token-bearing `/i/{token}` and legacy `/e/{token}` paths remain redacted in
  logs and no-store in responses.
- ZIP bundle routes continue to use server-controlled entry names and do not
  expose stored paths or object keys.
- No cleanup introduces React, a SPA admin app, OAuth, JWT, public admin
  dashboard behavior, backend decryption, raw key access, or key escrow.

## Proposed PR Sequence

### Phase 1: Document And Stabilize Boundaries

Document the current file concerns and route-boundary checks. This issue is
that phase. No runtime code should change.

Validation:

```bash
scripts/check-markdown-links.py
git diff --check
```

### Phase 2: Organize Private Admin Web Files

Move or split private admin web files only enough to make future admin work
clearer. Preserve Go `html/template`, embedded static assets, existing forms,
CSRF behavior, no-store headers, and private-admin-only routing.

Validation:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
scripts/check-markdown-links.py
git diff --check
```

### Phase 3: Extract Pure Bundle Helpers

Move bundle manifest and ZIP helper logic behind a small package or narrower
file boundary only if the API accepts explicit safe metadata and writer
dependencies. Keep bundle handlers in `internal/httpapi`.

Validation should include bundle tests that prove hash and size verification,
controlled ZIP entry names, no stored path exposure, and fail-closed behavior.

### Phase 4: Narrow Shared Response And Error Helpers

Extract response helpers only after confirming the result does not create
import cycles with handlers, auth middleware, or route registration. Keep safe
error codes stable.

Validation should include existing handler tests and focused response-shape
tests.

### Phase 5: Organize Tests And Fixtures

Reduce duplicated handler setup by moving package-local test helpers into
clearer files. Do not hide route-boundary setup behind abstractions that make
main/private mux mistakes harder to see in tests.

### Phase 6: Defer Handler Package Extraction Until Dependencies Are Smaller

Do not rush broad handler package extraction. Auth, admin, upload, relay,
sharing, wrapped-key, and deletion handlers should remain in `internal/httpapi`
until a later issue defines a package boundary that avoids cycles and preserves
route, logging, key custody, and listener-separation guarantees.

## Follow-Up Issue Mapping

Existing open issues already cover the next cleanup slices:

- `#333`: extract bundle manifest and verification helpers.
- `#334`: extract shared HTTP response and error helpers.
- `#335`: organize private admin web files before feature expansion.
- `#336`: tighten `internal/httpapi` test fixture organization.

Do not create duplicate issues for those topics from this plan. If later work
finds a concrete route-boundary or logging gap, create one narrow follow-up
with affected files, acceptance criteria, and validation commands.

## Review Checklist

Before merging any cleanup PR:

- [ ] Diff is smaller than the affected concern and does not mix unrelated
      handler families.
- [ ] Route registration remains readable in `routes.go`.
- [ ] Main API/viewer and private-admin mux separation is preserved.
- [ ] No raw token, request body, uploaded byte, Authorization header,
      plaintext, raw key, wrapped-key ciphertext, stored path, object key,
      private deployment detail, or user safety data is logged or rendered.
- [ ] Docs and `docs/code-map.md` still describe the implemented package
      layout accurately.
- [ ] Validation commands appropriate to the changed files passed.
