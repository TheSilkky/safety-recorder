# Compose Smoke Secrets

These files contain fake local smoke-test values only.

They exist so `docker compose config` works without first running
`compose/smoke-test.sh`. The smoke runner creates a temporary ignored secrets
directory from the current `PROOFLINE_SMOKE_*` environment variables before it
starts the stack.

Do not put real deployment credentials in this directory.
