# SPDX-License-Identifier: Apache-2.0
"""Tests for the problem code check (tools/docs/problem_code_refs.py)."""

from __future__ import annotations

import subprocess
from pathlib import Path

import problem_code_refs
import pytest

ROOT = Path(__file__).resolve().parents[3]
CODES = {"conflict": 409, "payload_too_large": 413, "validation_failed": 400}


@pytest.mark.parametrize(
    "text",
    [
        "Otherwise `409 quota_exceeded`.\n",
        "Otherwise `409` `quota_exceeded`.\n",
        "Otherwise 409 `quota_exceeded`.\n",
        "| Case | `507 storage_full` |\n",
        "otherwise `422\n  voice_too_long`.\n",
        "Size: `413 conflict`.\n",
    ],
)
def test_unregistered_pair_fails(text: str) -> None:
    problems = problem_code_refs.check_text("page.md", text, CODES)
    assert len(problems) == 1
    assert problems[0].startswith("page.md:1: ")


def test_status_mismatch_names_registered_status() -> None:
    problems = problem_code_refs.check_text("page.md", "x\n`413 conflict`\n", CODES)
    assert problems == ["page.md:2: `413 conflict`: registered with status 409 (06 section 5)"]


@pytest.mark.parametrize(
    "text",
    [
        "Otherwise `409 conflict` with `reason: quota_exceeded`.\n",
        "Otherwise `409` `conflict`; 413 `payload_too_large`.\n",
        "`400 validation_failed` with `errors[].code` `part_digest_mismatch`.\n",
        "`403` or `404`; `429` with `Retry-After`.\n",
        "Close code `4004` `forbidden`; port 4317 `otlp`.\n",
        "Limit `413` for `message.max_content_bytes`.\n",
        "`409` with code `quota_exceeded`.\n",
        "Level `422`, then `conflict` apart.\n",
    ],
)
def test_registered_or_unrelated_text_passes(text: str) -> None:
    assert problem_code_refs.check_text("page.md", text, CODES) == []


def test_main_reads_openapi_and_exit_codes(tmp_path: Path) -> None:
    openapi = tmp_path / "openapi.yaml"
    openapi.write_text(
        "components:\n  schemas:\n    Problem:\n      x-problem-codes:\n"
        "      - {code: conflict, status: 409}\n",
        encoding="utf-8",
    )
    good = tmp_path / "good.md"
    good.write_text("`409 conflict`\n", encoding="utf-8")
    bad = tmp_path / "bad.md"
    bad.write_text("`409 part_conflict`\n", encoding="utf-8")
    assert problem_code_refs.main(["--openapi", str(openapi), str(good)]) == 0
    assert problem_code_refs.main(["--openapi", str(openapi), str(good), str(bad)]) == 1


def test_repository_is_clean() -> None:
    """The real artefact: every tracked Markdown file and the API files of this repository."""
    files = subprocess.run(
        ["git", "ls-files", "--", "*.md", "docs/api/openapi.yaml", "docs/api/asyncapi.yaml"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split()
    codes = problem_code_refs.load_codes(ROOT / "docs/api/openapi.yaml")
    problems: list[str] = []
    for name in files:
        problems += problem_code_refs.check_text(name, (ROOT / name).read_text("utf-8"), codes)
    assert problems == []
