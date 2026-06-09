# Codex Prompt: Project Context Check

Read current project context before making changes.

Do **not** change files.
Do **not** add features.

## Goal

Summarize the current repo state and the likely impact of the requested task before any implementation work begins.

## Source of truth

Before making changes, read current source-of-truth files as relevant:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `SECURITY.md`
- `docs/README.md`
- `docs/v1-preview-direction.md`
- `docs/key-custody.md`, if present
- relevant files in `docs/`
- relevant source files
- relevant tests
- relevant issue or PR, if this is issue/PR work

Do not rely on stale assumptions from this prompt if the repository has changed.

## Additional sources for issue/PR tasks

If the task references an issue or pull request, inspect it first:

```bash
gh issue view <ISSUE_NUMBER> --repo open-proofline/server
gh pr view <PR_NUMBER> --repo open-proofline/server
```

Use whichever command is relevant.

If GitHub CLI is unavailable, say so and continue from local files.

## Output

Return:

1. Current project scope
2. Current backend surfaces
3. Private/public listener split
4. Current security boundaries
5. Current known exclusions / out-of-scope features
6. Current key custody / encryption posture, distinguishing implemented ciphertext-only behaviour from future explicit key custody design
7. Files likely affected by the requested task
8. Files or areas that must not change
9. Likely validation commands
10. Recommended next reusable prompt to use
11. Any clarifying questions, only if required to avoid a bad change
