# Historical Feature Prompt: Multiple Bind Addresses

This prompt is historical. Do not re-run it without checking it against the current `README.md`, `AGENTS.md`, and code.

## Current behaviour to preserve

- `SAFE_PRIVATE_BIND_ADDRS` and `SAFE_PUBLIC_BIND_ADDRS` are comma-separated `host:port` lists.
- Empty entries are rejected.
- Plural variables take precedence over singular variables.
- Singular variables are still supported when matching plural variables are unset.
- Do not silently bind to `0.0.0.0` unless explicitly configured.
- Inside Docker, bind to `0.0.0.0` inside the container and restrict exposure using port publishing/firewall/WireGuard/reverse proxy.
- Separate bind addresses are a deployment boundary, not a complete security model.

Use current reusable review prompts for maintenance.
