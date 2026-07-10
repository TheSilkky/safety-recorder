# Reports

This directory contains public technical review artifacts for Proofline.
Reports are documentation and planning inputs, not formal audits,
certifications, or production-readiness endorsements.

Historical reports keep their original `Safety Recorder` titles, filenames, and reviewed-commit context because they describe the project before the docs-only Proofline rename.

## Published Reports

| Date | Report | Reviewed commit | Notes |
|---|---|---|---|
| 2026-06-01 | [Technical Review of Proofline v0.10.0](2026-06-01-proofline-v0.10.0-technical-review.md) | `74ec526123708b7a4904f25b8e805e9847fcfdbe` | AI-assisted public technical review after Codex Phase 2 validation. No new branch-scoped issue drafts were created because the tag-context release workflow evidence was verified during validation and no new actionable finding required a draft. |
| 2026-05-30 | [Technical Review of Proofline v0.8.0](2026-05-30-proofline-v0.8.0-technical-review.md) | `4ff318b9faecea59475794ebaaec662b3e0afa78` | AI-assisted public technical review after Codex Phase 2 validation. No new branch-scoped issue drafts were created because draft findings were removed or downgraded during validation. |
| 2026-05-28 | [Technical Review of Proofline v0.7.0](2026-05-28-proofline-v0.7.0-technical-review.md) | `12e97543953ff1ba938c128a6afec73e9643acce` | AI-assisted public technical review after Codex Phase 2 validation. Follow-up items were written as local branch-scoped drafts only. |
| 2026-05-26 | [Technical Review of Safety Recorder v0.5.0](2026-05-26-safety-recorder-v0.5.0-technical-review.md) | `fe2f8bf6e90e6f1e2086d487783fa0a03d83688c` | AI-assisted public technical review after Codex Phase 2 validation. One non-blocking CI assurance follow-up was written as a local branch-scoped draft only. |
| 2026-05-25 | [Technical Review of Safety Recorder v0.5.0-rc.1](2026-05-25-safety-recorder-v0.5.0-rc.1-technical-review.md) | `5b5a57354d6fcdbdc1ef1f440372c04b8bba2289` | AI-assisted public technical review after Codex Phase 2 validation. Follow-up items were written as local branch-scoped drafts only. |
| 2026-05-23 | [Technical Review of Safety Recorder v0.4.x](2026-05-23-safety-recorder-technical-review.md) | `89a07ff0616fe5ad13437f1b2eec93e091ec3ef6` | AI-assisted public technical review after maintainer Phase 2 validation. |

## Report Prompts

Run the
[Phase 0 preflight](prompts/phase-0-codex-technical-review-preflight.md) in Codex
Max for a focused single-agent plan, or Ultra when meaningful independent
read-only lanes justify delegation, and obtain maintainer approval before running the
[Phase 1 technical review](prompts/phase-1-codex-technical-review.md). Phase 1
writes its source-cited report under the ignored `.technical-review-drafts/`
directory and records exact evidence for commands it executes. The independent
[Phase 2 Codex validation workflow](../../codex/prompts/95-validate-technical-review-report.md)
checks the draft and publishes the cleaned report under `docs/reports/`.

Report prompts and reports must remain public-safe. Do not include raw tokens,
secrets, private deployment details, exploit payloads, raw keys, plaintext media,
or user-safety data.
