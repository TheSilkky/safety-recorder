# Historical Refactor Prompt: Separate Private API and Emergency Viewer Servers

This prompt is historical. The project now has separate private/public listener groups.

## Current behaviour to preserve

- Private `/v1` routes run on private API listeners.
- Public emergency viewer routes run on public viewer listeners.
- Private write/admin routes must not be mounted on the public viewer server.
- Emergency viewer routes are read-only.
- Both listener groups can bind to multiple addresses using:
  - `SAFE_PRIVATE_BIND_ADDRS`
  - `SAFE_PUBLIC_BIND_ADDRS`

Use current reusable review prompts for future maintenance.
