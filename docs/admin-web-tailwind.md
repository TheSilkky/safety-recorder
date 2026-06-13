# Private Admin Tailwind CSS

Proofline keeps the private admin web surface server-rendered and embedded in
the Go binary. Tailwind is only a local maintainer build tool for the checked-in
admin CSS; the server runtime does not require Node, npm, Vite, React, or a
public asset pipeline.

## Files

- `tailwind.admin.config.js` defines the private admin Tailwind content paths
  and Proofline admin design tokens.
- `internal/httpapi/web/admin/tailwind.css` is the Tailwind input source for
  reusable admin layout, card, navigation, form, notice, warning, badge, and
  action-group patterns.
- `internal/httpapi/web/admin/static/styles.css` is the generated CSS embedded
  by `internal/httpapi/admin_web.go` and served only from `/admin/static/...` on
  the private-admin listener.

## Build Command

Regenerate the checked-in admin CSS from the repository root with:

```bash
npx --yes tailwindcss@3.4.17 -c tailwind.admin.config.js -i internal/httpapi/web/admin/tailwind.css -o internal/httpapi/web/admin/static/styles.css --minify
```

Do not add a repo-local `package.json`, Vite app, React app, Catalyst source
copy, or server runtime dependency for this build path. Review the generated CSS
before committing it.
