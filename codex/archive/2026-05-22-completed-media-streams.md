# Historical Feature Prompt: Completed Media Streams and Downloadable Evidence Bundles

This prompt is historical. Do not re-run it without checking it against the current `README.md`, `AGENTS.md`, and code.

## Current reminder

The project now includes media streams and completed encrypted stream/incident ZIP evidence bundles.

Important current behaviour to preserve:

- chunks are immutable
- new clients should provide `stream_id`
- legacy chunks without `stream_id` are still stored/listed but are not included in completed-stream bundle downloads
- current chunk identity remains `(incident_id, media_type, chunk_index)`
- evidence bundles are encrypted chunk bundles, not decrypted/playable media
- public emergency viewer download routes are read-only and token-scoped

Use `codex/prompts/code-review.md`, `codex/prompts/security-review.md`, or `codex/prompts/documentation-update.md` for current maintenance work.
