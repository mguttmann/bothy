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
        "Sonst `409 quota_exceeded`.\n",
        "Sonst `409` `quota_exceeded`.\n",
        "Sonst 409 `quota_exceeded`.\n",
        "| Fall | `507 storage_full` |\n",
        "sonst `422\n  voice_too_long`.\n",
        "Größe: `413 conflict`.\n",
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
        "Sonst `409 conflict` mit `reason: quota_exceeded`.\n",
        "Sonst `409` `conflict`; 413 `payload_too_large`.\n",
        "`400 validation_failed` mit `errors[].code` `part_digest_mismatch`.\n",
        "`403` bzw. `404`; `429` mit `Retry-After`.\n",
        "Schließ-Code `4004` `forbidden`; Port 4317 `otlp`.\n",
        "Grenze `413` für `message.max_content_bytes`.\n",
        "`409` mit Code `quota_exceeded`.\n",
        "Stufe `422`, dann `conflict` getrennt.\n",
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
