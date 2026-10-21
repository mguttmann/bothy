# SPDX-License-Identifier: Apache-2.0
"""Versions held equal: component, spec pin and fixtures (design notes 15 and 17)."""

from __future__ import annotations

import json
import re
import tomllib
from pathlib import Path

import claude_code_runner
from claude_code_runner.heartbeat import MIN_CLAUDE_CODE_VERSION, PINNED_CLAUDE_CODE_VERSION

ROOT = Path(__file__).resolve().parents[2]
SPEC = ROOT.parents[1] / "docs" / "spec" / "09-agenten.md"


def test_pyproject_version_equals_version_file() -> None:
    project = tomllib.loads((ROOT / "pyproject.toml").read_text())["project"]
    version = (ROOT / "VERSION").read_text().strip()
    assert project["version"] == version == claude_code_runner.__version__


def test_min_version_is_the_one_18_2_names() -> None:
    text = SPEC.read_text(encoding="utf-8")
    section = text[text.index("### 18.2") : text.index("### 18.3")]
    named = re.findall(r"--permission-prompts none` [^.]*?v(\d+\.\d+\.\d+)", section)
    assert named == [MIN_CLAUDE_CODE_VERSION]


def test_fixtures_name_the_pinned_version() -> None:
    fixtures = sorted((ROOT / "tests" / "fake_claude" / "fixtures").glob("*.jsonl"))
    assert fixtures
    for path in fixtures:
        meta = json.loads(path.read_text().splitlines()[0])
        assert meta["type"] == "fixture_meta"
        assert meta["origin"] == "constructed"
        assert meta["cli_version"] == PINNED_CLAUDE_CODE_VERSION
