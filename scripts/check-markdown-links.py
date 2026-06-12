#!/usr/bin/env python3
"""Check local Markdown links without network access.

The checker intentionally ignores external URLs. It validates local inline and
reference-style Markdown links, plus simple GitHub-style heading anchors in
Markdown targets.
"""

from __future__ import annotations

import argparse
import os
import re
import shlex
import sys
import tempfile
from pathlib import Path
from urllib.parse import unquote, urlsplit


DEFAULT_ROOT_FILES = ("README.md", "AGENTS.md", "SECURITY.md")
DEFAULT_TREE_DIRS = ("docs", "codex")
EXTERNAL_SCHEMES = {
    "http",
    "https",
    "mailto",
    "tel",
    "ftp",
    "ftps",
    "data",
}

INLINE_LINK_RE = re.compile(r"!?\[[^\]\n]*\]\(([^)\n]+)\)")
REFERENCE_LINK_RE = re.compile(r"^\s*\[[^\]\n]+\]:\s+(.+?)\s*$")
HEADING_RE = re.compile(r"^(#{1,6})\s+(.+?)\s*#*\s*$")
HTML_ID_RE = re.compile(r"\bid=[\"']([^\"']+)[\"']")


def main() -> int:
    parser = argparse.ArgumentParser(description="Check local Markdown links.")
    parser.add_argument(
        "paths",
        nargs="*",
        help="Markdown files or directories to check. Defaults to README, AGENTS, SECURITY, docs, and codex.",
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Run a temporary known-bad fixture to prove failure behavior.",
    )
    args = parser.parse_args()

    if args.self_test:
        return run_self_test()

    root = Path.cwd().resolve()
    files = collect_markdown_files(root, args.paths)
    failures = check_files(root, files)
    if failures:
        for failure in failures:
            print(failure, file=sys.stderr)
        print(f"markdown link check failed: {len(failures)} issue(s)", file=sys.stderr)
        return 1
    print(f"markdown link check passed: {len(files)} file(s)")
    return 0


def collect_markdown_files(root: Path, paths: list[str]) -> list[Path]:
    if paths:
        candidates = [root / path for path in paths]
    else:
        candidates = [root / path for path in DEFAULT_ROOT_FILES]
        candidates.extend(root / path for path in DEFAULT_TREE_DIRS)

    files: list[Path] = []
    seen: set[Path] = set()
    for candidate in candidates:
        if candidate.is_file() and candidate.suffix.lower() == ".md":
            add_file(candidate, seen, files)
            continue
        if candidate.is_dir():
            for path in sorted(candidate.rglob("*.md")):
                add_file(path, seen, files)
            continue
        if paths:
            print(f"warning: skipped missing or non-Markdown path: {candidate}", file=sys.stderr)
    return files


def add_file(path: Path, seen: set[Path], files: list[Path]) -> None:
    resolved = path.resolve()
    if resolved in seen:
        return
    seen.add(resolved)
    files.append(resolved)


def check_files(root: Path, files: list[Path]) -> list[str]:
    failures: list[str] = []
    anchor_cache: dict[Path, set[str]] = {}
    for source in files:
        text = source.read_text(encoding="utf-8")
        for line_number, raw_target in iter_markdown_link_targets(text):
            target = parse_link_target(raw_target)
            if not target or should_ignore_target(target):
                continue
            failure = check_target(root, source, line_number, target, anchor_cache)
            if failure:
                failures.append(failure)
    return failures


def iter_markdown_link_targets(text: str) -> list[tuple[int, str]]:
    targets: list[tuple[int, str]] = []
    fence: str | None = None
    for line_number, line in enumerate(text.splitlines(), start=1):
        stripped = line.lstrip()
        if fence:
            if stripped.startswith(fence):
                fence = None
            continue
        if stripped.startswith("```") or stripped.startswith("~~~"):
            fence = stripped[:3]
            continue
        for match in INLINE_LINK_RE.finditer(line):
            targets.append((line_number, match.group(1)))
        reference_match = REFERENCE_LINK_RE.match(line)
        if reference_match:
            targets.append((line_number, reference_match.group(1)))
    return targets


def parse_link_target(raw_target: str) -> str:
    value = raw_target.strip()
    if value.startswith("<"):
        end = value.find(">")
        if end != -1:
            return value[1:end].strip()
    try:
        parts = shlex.split(value)
    except ValueError:
        parts = value.split()
    if not parts:
        return ""
    return parts[0].strip()


def should_ignore_target(target: str) -> bool:
    split = urlsplit(target)
    if split.scheme.lower() in EXTERNAL_SCHEMES:
        return True
    if target.startswith("//"):
        return True
    return False


def check_target(
    root: Path,
    source: Path,
    line_number: int,
    target: str,
    anchor_cache: dict[Path, set[str]],
) -> str | None:
    split = urlsplit(target)
    target_path_text = unquote(split.path)
    anchor = unquote(split.fragment)

    if not target_path_text:
        target_path = source
    else:
        raw_path = Path(target_path_text)
        if raw_path.is_absolute():
            target_path = (root / target_path_text.lstrip("/")).resolve()
        else:
            target_path = (source.parent / raw_path).resolve()

    if not is_within_root(root, target_path):
        return format_failure(source, line_number, target, "target escapes repository root")
    if not target_path.exists():
        return format_failure(source, line_number, target, "target file does not exist")
    if anchor and target_path.suffix.lower() == ".md":
        anchors = anchor_cache.setdefault(target_path, markdown_anchors(target_path))
        if anchor not in anchors:
            return format_failure(source, line_number, target, f"anchor #{anchor} not found")
    return None


def is_within_root(root: Path, target: Path) -> bool:
    try:
        target.relative_to(root)
        return True
    except ValueError:
        return False


def markdown_anchors(path: Path) -> set[str]:
    text = path.read_text(encoding="utf-8")
    anchors: set[str] = set()
    slug_counts: dict[str, int] = {}
    for line in text.splitlines():
        heading_match = HEADING_RE.match(line)
        if heading_match:
            base = github_heading_slug(strip_markdown(heading_match.group(2)))
            count = slug_counts.get(base, 0)
            slug_counts[base] = count + 1
            anchors.add(base if count == 0 else f"{base}-{count}")
        for html_id in HTML_ID_RE.findall(line):
            anchors.add(html_id)
    return anchors


def strip_markdown(text: str) -> str:
    text = re.sub(r"`([^`]*)`", r"\1", text)
    text = re.sub(r"!\[([^\]]*)\]\([^)]+\)", r"\1", text)
    text = re.sub(r"\[([^\]]+)\]\([^)]+\)", r"\1", text)
    text = text.replace("\\", "")
    return text


def github_heading_slug(text: str) -> str:
    value = text.strip().lower()
    value = re.sub(r"<[^>]+>", "", value)
    value = re.sub(r"[^\w\s-]", "", value, flags=re.UNICODE)
    value = re.sub(r"\s+", "-", value)
    value = re.sub(r"-+", "-", value)
    return value.strip("-")


def format_failure(source: Path, line_number: int, target: str, reason: str) -> str:
    return f"{source}:{line_number}: {reason}: {target}"


def run_self_test() -> int:
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        (root / "docs").mkdir()
        (root / "README.md").write_text(
            "```md\n[fenced missing](docs/fenced-missing.md)\n```\n"
            "[missing](docs/missing.md)\n",
            encoding="utf-8",
        )
        old_cwd = Path.cwd()
        try:
            os.chdir(root)
            failures = check_files(root.resolve(), collect_markdown_files(root.resolve(), []))
        finally:
            os.chdir(old_cwd)
        if not failures:
            print("self-test failed: known-bad link was not detected", file=sys.stderr)
            return 1
        if any("fenced-missing.md" in failure for failure in failures):
            print("self-test failed: fenced code block link was checked", file=sys.stderr)
            return 1
        print("self-test passed: known-bad link failed as expected")
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
