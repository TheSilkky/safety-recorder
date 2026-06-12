# One-off Codex Work Order: Create Release If Missing And Upload Linux amd64 Binary

Historical/reference-only work order.

Use this prompt as context before running `codex/prompts/70-work-on-github-issue.md` or equivalent scoped implementation work.

## Goal

Automate attaching the Linux amd64 binary produced by the CI workflow to the matching GitHub Release for `v*` tag pushes.

Use **Option B-lite**:

1. On a `v*` tag workflow, check whether a GitHub Release already exists for the tag.
2. If the Release does not exist, create a minimal GitHub Release for the already-pushed tag.
3. Use `--verify-tag` so CI never creates a tag from the default branch by accident.
4. Mark release-candidate tags such as `v0.5.0-rc.2` as prereleases.
5. Upload `safety-recorder-linux-amd64` as a GitHub Release asset.
6. Do not use `--clobber` by default.

This should replace the current manual step:

```bash
gh release upload <tag> ./safety-recorder-linux-amd64 --repo TheSilkky/safety-recorder
```

with a narrowly scoped CI job.

## Repository

```text
TheSilkky/safety-recorder
```

## Branch / target

Current release-prep branch:

```text
release/v0.5.0-prep
```

Target release:

```text
v0.5.0
```

Test first with a release-candidate tag such as:

```text
v0.5.0-rc.2
```

## Current workflow facts to revalidate

Before editing, re-read:

```bash
sed -n '1,280p' .github/workflows/ci.yml
sed -n '80,240p' docs/development.md
sed -n '1,120p' CHANGELOG.md
```

Expected current behavior on `release/v0.5.0-prep`:

- CI runs on pull requests, branch pushes, and `v*` tag pushes.
- Top-level workflow permissions are read-only.
- `build-binary` builds `server/dist/safety-recorder-linux-amd64`.
- `build-binary` uploads an Actions artifact named `safety-recorder-linux-amd64`.
- `attest-binary` runs only on `v*` tag pushes and attests the downloaded binary artifact.
- `docker-publish` runs on trusted pushes to `main` or `v*` tags.
- `docker-publish` publishes GHCR images and generates Docker image attestations.
- `docs/development.md` documents artifact attestation verification.
- The workflow pins external actions to full commit SHAs with same-line version comments.

## Problem

The Linux amd64 binary exists as a GitHub Actions artifact, but it is not automatically attached as a GitHub Release asset.

GitHub Actions artifacts and GitHub Release assets are different things. A release user should be able to download:

```text
safety-recorder-linux-amd64
```

directly from the GitHub Release page for the tag.

## Required behavior: Option B-lite

On `v*` tag pushes, after the Linux binary is built and attested:

1. Download the `safety-recorder-linux-amd64` Actions artifact.
2. Determine the tag name from `github.ref_name`.
3. Check whether a GitHub Release exists for the tag.
4. If missing, create a minimal Release for the existing tag using `gh release create --verify-tag`.
5. If the tag name contains an RC pre-release marker, mark the Release as a prerelease.
6. Set `--latest=false` for pre-release tags so RCs are not promoted as latest.
7. Upload `dist/safety-recorder-linux-amd64` to the Release.
8. Do not use `--clobber` by default.
9. Keep top-level workflow permissions read-only.
10. Grant release-write permissions only to the release asset upload job.
11. Do not change application behavior.
12. Do not change Docker image behavior.
13. Do not weaken attestation behavior.
14. Do not upload raw tokens, secrets, request bodies, private deployment details, or user-safety data.

## Recommended implementation shape

Prefer a new tag-only job in `.github/workflows/ci.yml`, for example:

```yaml
upload-release-binary:
  name: Upload release binary
  runs-on: ubuntu-latest
  needs:
    - attest-binary
  if: ${{ github.event_name == 'push' && startsWith(github.ref, 'refs/tags/v') }}
  permissions:
    contents: write
  steps:
    - name: Download binary artifact
      uses: actions/download-artifact@<PINNED_SHA> # v7
      with:
        name: safety-recorder-linux-amd64
        path: dist

    - name: Create release if missing and upload binary
      env:
        GH_TOKEN: ${{ github.token }}
        TAG_NAME: ${{ github.ref_name }}
        REPO: ${{ github.repository }}
      run: |
        set -euo pipefail

        prerelease_args=()
        if [[ "$TAG_NAME" == *"-rc."* || "$TAG_NAME" == *"-rc"* || "$TAG_NAME" == *"-alpha"* || "$TAG_NAME" == *"-beta"* ]]; then
          prerelease_args+=(--prerelease --latest=false)
        fi

        if ! gh release view "$TAG_NAME" --repo "$REPO" >/dev/null 2>&1; then
          gh release create "$TAG_NAME" \
            --repo "$REPO" \
            --verify-tag \
            --title "$TAG_NAME" \
            --notes "Automated release shell for $TAG_NAME. Maintainers should review and replace these notes before relying on this release." \
            "${prerelease_args[@]}"
        fi

        gh release upload "$TAG_NAME" dist/safety-recorder-linux-amd64 --repo "$REPO"
```

Codex should adapt the exact YAML to the current workflow style.

## Notes on the shell logic

Use `gh release view "$TAG_NAME"` to detect whether the Release exists.

Use `gh release create "$TAG_NAME" --verify-tag` to create a Release only for an already-pushed tag.

Do not use `--generate-notes` by default.

Reason:

- Auto-generated notes may be useful later, but the project currently uses careful release notes and public-readiness wording.
- CI-created release notes should be minimal and conservative.
- Maintainers can edit the Release after the workflow attaches the asset.

Use `--prerelease --latest=false` for release-candidate tags.

Suggested prerelease detection:

```bash
if [[ "$TAG_NAME" == *"-rc."* || "$TAG_NAME" == *"-rc"* || "$TAG_NAME" == *"-alpha"* || "$TAG_NAME" == *"-beta"* ]]; then
  prerelease_args+=(--prerelease --latest=false)
fi
```

This should mark tags such as:

```text
v0.5.0-rc.2
v0.5.0-rc2
v0.5.0-beta.1
v0.5.0-alpha.1
```

as prereleases.

Final tags such as:

```text
v0.5.0
```

should not be marked prerelease.

## Asset overwrite policy

Do not silently overwrite release assets.

Default behavior:

- If the asset does not exist, upload it.
- If the asset already exists, `gh release upload` should fail.
- The failure should tell the maintainer that the asset already exists or that manual cleanup is needed.

Do not use:

```bash
--clobber
```

by default.

Reason:

- `gh release upload --clobber` deletes existing assets before re-uploading them.
- If the upload fails after deletion, the original asset can be lost.
- Final release assets should not be silently replaced.

If the maintainer later wants clobber behavior for release-candidate reruns, add it in a separate scoped change and document the risk.

## Permissions policy

Keep workflow-level permissions:

```yaml
permissions:
  contents: read
```

The release asset upload job may use:

```yaml
permissions:
  contents: write
```

Only add broader permissions if required and documented.

Do not add any of these to the release-asset upload job unless they are actually needed:

```yaml
packages: write
attestations: write
id-token: write
```

Do not broaden permissions for:

- `test`
- `build-binary`
- `attest-binary`
- `docker`
- `docker-publish`
- unrelated jobs

## Action pinning policy

If reusing existing actions already present in the workflow, keep the current pinned full commit SHA and same-line version comment.

If adding a new external action, pin it to a full 40-character commit SHA and add a same-line version comment, consistent with the repository's pinned-action policy.

Prefer the built-in GitHub CLI on `ubuntu-latest` over adding a new third-party action, unless there is a strong reason to add an action.

## Documentation updates

Update `docs/development.md` to explain:

- the workflow uploads `safety-recorder-linux-amd64` as a GitHub Release asset for `v*` tag releases
- the workflow creates a minimal GitHub Release if the Release does not exist yet
- the workflow uses `--verify-tag` so it only creates a Release for an already-pushed tag
- release-candidate tags are marked as prereleases
- RC tags should not be treated as final stable releases
- the workflow does not use `--clobber` by default
- how to verify the release asset exists
- how release asset upload relates to binary attestation verification
- that attestations prove provenance, not production readiness or vulnerability freedom

Example wording:

```md
For `v*` tag workflows, CI creates a minimal GitHub Release if one does not
already exist, verifies that the tag exists remotely, and uploads
`safety-recorder-linux-amd64` as a Release asset. Release-candidate tags are
marked as prereleases. The workflow does not overwrite existing Release assets by
default.
```

Update `CHANGELOG.md` under `Unreleased` or the current release-candidate section with a concise entry such as:

```md
- Automated creating a minimal GitHub Release when needed and uploading the Linux amd64 binary as a Release asset for `v*` tag workflows.
```

Do not overstate release maturity.

## Suggested files to change

Expected:

```text
.github/workflows/ci.yml
docs/development.md
CHANGELOG.md
```

Do not change:

```text
server/
server/Dockerfile
server/migrations/
docs/security-model.md
docs/threat-model.md
docs/key-custody.md
```

unless a specific validation problem proves they are affected.

## Validation

For this workflow/documentation change:

```bash
git diff --stat
git diff -- .github/workflows/ci.yml docs/development.md CHANGELOG.md
```

If only workflow/docs changed, Go tests are not required locally.

The real validation is a trusted tag workflow run.

For release-candidate validation, use:

```text
v0.5.0-rc.2
```

After the tag workflow:

1. Confirm `Go tests` passed.
2. Confirm `Build Linux binary` passed.
3. Confirm `Attest Linux binary` passed.
4. Confirm `Upload release binary` passed.
5. Confirm `Build Docker image` passed.
6. Confirm `Publish Docker image` passed.
7. Confirm the GitHub Release exists.
8. Confirm the GitHub Release is marked prerelease for `v0.5.0-rc.2`.
9. Confirm the GitHub Release contains `safety-recorder-linux-amd64`.
10. Confirm binary attestation verification still works.
11. Confirm GHCR image attestation verification still works.
12. Confirm rerunning the upload job fails rather than silently replacing an existing release asset, unless clobber behavior was explicitly implemented.

## Verification commands after release

After the GitHub Release exists and the workflow uploads the asset:

```bash
gh release view v0.5.0-rc.2 --repo TheSilkky/safety-recorder
```

If testing final release later:

```bash
gh release view v0.5.0 --repo TheSilkky/safety-recorder
```

Verify the binary attestation for the downloaded asset:

```bash
gh attestation verify ./safety-recorder-linux-amd64 \
  -R TheSilkky/safety-recorder \
  --signer-workflow TheSilkky/safety-recorder/.github/workflows/ci.yml \
  --source-ref refs/tags/v0.5.0-rc.2
```

Adjust the tag for final `v0.5.0`.

## Acceptance criteria

- [ ] CI creates a minimal GitHub Release for `v*` tag workflows when no Release exists.
- [ ] CI uses `gh release create --verify-tag` so it never creates a tag from the default branch.
- [ ] CI marks release-candidate tags as prereleases.
- [ ] CI sets `--latest=false` for prerelease tags.
- [ ] CI uploads `safety-recorder-linux-amd64` as a GitHub Release asset for `v*` tag workflows.
- [ ] The release asset upload job runs only for `v*` tag pushes.
- [ ] The release asset upload job depends on the binary build and, preferably, binary attestation.
- [ ] Workflow-level permissions remain read-only.
- [ ] `contents: write` is scoped only to the release asset upload job.
- [ ] The workflow does not broaden Docker publish permissions.
- [ ] The workflow does not change Docker build/publish behavior.
- [ ] The workflow does not use unpinned third-party actions.
- [ ] The workflow does not use `--clobber` by default.
- [ ] Docs explain that CI creates a minimal Release if missing.
- [ ] Docs explain that RC tags are marked as prereleases.
- [ ] Docs explain how to verify the release asset and its attestation.
- [ ] Changelog notes the release asset upload automation.
- [ ] No application code changes are included.

## Out of scope

- Do not change application behavior.
- Do not change the Go binary build command unless required for the upload path.
- Do not add checksums/signatures unless explicitly requested in a separate issue.
- Do not add package managers or installers.
- Do not create Docker Compose, Kubernetes, cloud deployment, or public production deployment workflows.
- Do not claim production readiness.
- Do not change key custody, backend decryption, browser decryption, OAuth/JWT, user accounts, SMS, push notifications, or public admin behavior.
- Do not automate final release prose beyond the minimal Release shell.

## Prompt sequence

Recommended prompt flow:

```text
00-project-context-check.md
05-codex-change-control.md
70-work-on-github-issue.md
30-security-review.md
40-documentation-update.md
90-release-check.md
75-create-draft-pr-from-current-branch.md
76-request-codex-pr-review.md
```

When using `75-create-draft-pr-from-current-branch.md`, set the target base branch explicitly.

If this work is being done on `release/v0.5.0-prep`, use:

```text
Target base branch: release/v0.5.0-prep
```

If doing it on a separate issue branch off the release-prep branch, open the PR back into:

```text
release/v0.5.0-prep
```

## Codex output requirements

Return:

1. implementation summary
2. files changed
3. confirmation that Option B-lite was implemented
4. release creation behavior
5. prerelease detection behavior
6. latest-release behavior for RC tags
7. permissions changes
8. release asset overwrite/clobber policy
9. documentation updates
10. validation commands run
11. expected tag-workflow validation steps
12. manual follow-up required before final `v0.5.0`
