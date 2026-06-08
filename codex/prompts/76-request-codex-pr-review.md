# Codex Prompt: Request Codex Review on Pull Request

Use this when a draft pull request exists and should be reviewed by Codex.

## Inputs

Pull request number: `<PR_NUMBER>`

Repository:

```text
open-proofline/server
```

Expected base branch:

```text
<EXPECTED_BASE_BRANCH>
```

Examples:

```text
main
develop
release/v0.5.0-prep
```

If the expected base branch is unknown, inspect the PR metadata first and report the actual base branch before reviewing.

## First steps

Fetch PR metadata and confirm base/head branches:

```bash
gh pr view <PR_NUMBER> --repo open-proofline/server --json number,title,state,isDraft,baseRefName,headRefName,headRepositoryOwner,mergeStateStatus,statusCheckRollup,url
```

Before posting a review request or reviewing locally, summarize:

1. PR number
2. actual base branch
3. actual head branch
4. whether actual base matches `<EXPECTED_BASE_BRANCH>`, if supplied
5. whether the PR is draft
6. linked issue(s), if visible
7. CI/check status, if visible

If the actual base branch does not match the expected base branch, stop and ask the maintainer whether to continue, update the PR base, or recreate the PR. Do not assume `main` is the correct base.

## Option A: Comment directly on the PR

Post this PR comment only after confirming the base branch is correct:

```md
@codex review

Please review this PR for correctness, security, scope control, and consistency with README.md, AGENTS.md, SECURITY.md, and relevant docs.

Base branch: `<ACTUAL_BASE_BRANCH>`
Head branch: `<ACTUAL_HEAD_BRANCH>`

Focus on:
- whether the change satisfies the linked issue acceptance criteria
- whether the diff is appropriate for the PR base branch
- whether unrelated scope was added
- tests and validation
- private/public listener separation
- logging consistency with `docs/logging-requirements.md`
- raw token / request body / uploaded byte / Authorization header logging
- plaintext/key, wrapped-key ciphertext, stored path, object key, private deployment detail, and user safety data logging
- backend decryption, server-side key access, or key custody changes accidentally introduced or introduced without explicit design scope
- whether explicit key custody/decryption changes update threat model, security model, encryption docs, tests, and operational guidance
- ZIP bundle path safety, if relevant
- encryption/key handling, if relevant
- documentation accuracy
- production-readiness claims

Do not suggest unrelated features.
Do not include sensitive vulnerability details in a public comment.
```

## Option B: Ask Codex locally to review a PR

Use this in the IDE if reviewing locally:

```md
Review pull request #<PR_NUMBER> in `open-proofline/server`.

First fetch PR metadata and diff.

Confirm:
- actual base branch
- actual head branch
- whether the actual base branch matches the expected base branch, if supplied
- whether the change is appropriate for that base branch

Then review for:

1. correctness
2. security
3. scope control
4. test coverage
5. documentation accuracy
6. consistency with README.md and AGENTS.md
7. whether it satisfies the linked issue acceptance criteria
8. whether it should remain draft
9. whether it changes key custody/decryption assumptions, and whether those changes are explicitly designed and documented
10. whether branch-scoped issue/report findings were revalidated against the PR base branch

Do not modify files unless explicitly requested.

Return:
- actual base branch and head branch
- whether the base branch is correct
- blocking issues
- non-blocking issues
- suggested minimal fixes
- whether the PR should remain draft
- whether the PR appears ready for human review
```

## Constraints

- Do not merge the PR.
- Do not mark the PR ready for review.
- Do not approve your own changes.
- Do not request changes unless the review is genuinely blocking.
- Do not include sensitive vulnerability details in public comments.
- Do not assume `main` is the base branch.
