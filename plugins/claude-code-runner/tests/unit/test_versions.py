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
SPEC = ROOT.parents[1] / "docs" / "spec" / "09-agents.md"


def test_pyproject_version_equals_version_file() -> None:
    project = tomllib.loads((ROOT / "pyproject.toml").read_text())["project"]
    version = (ROOT / "VERSION").read_text().strip()
    assert project["version"] == version == claude_code_runner.__version__


def test_min_version_is_the_one_18_2_names() -> None:
    text = SPEC.read_text(encoding="utf-8")
    section = text[text.index("### 18.2") : text.index("### 18.3")]
    named = re.findall(r"--permission-prompts none` [^.]*?v(\d+\.\d+\.\d+)", section)
    assert named == [MIN_CLAUDE_CODE_VERSION]


def test_recordings_are_of_the_pinned_version() -> None:
    """A change of the pin records again (``tests/real_cli/record.py``): no older file stays."""
    pin = PINNED_CLAUDE_CODE_VERSION
    recording = ROOT / "tests" / "fixtures" / "recording"
    assert sorted(p.name for p in recording.iterdir()) == [
        f"agent_tree_{pin}.jsonl",
        f"resumed_session_{pin}",
        f"stream_fields_{pin}.json",
    ]
    captured = ROOT / "tests" / "real_cli" / "captured"
    assert [p.name for p in captured.iterdir()] == [f"api_errors_{pin}.jsonl"]
    fields = json.loads((recording / f"stream_fields_{pin}.json").read_text())
    meta = json.loads((recording / f"resumed_session_{pin}" / "meta.json").read_text())
    assert fields["cli_version"] == meta["cli_version"] == pin
    streams = [
        recording / f"agent_tree_{pin}.jsonl",
        recording / f"resumed_session_{pin}" / "stream_resumed.jsonl",
    ]
    for stream in streams:
        inits = [
            message
            for message in map(json.loads, stream.read_text().splitlines())
            if message.get("subtype") == "init"
        ]
        assert inits, stream.name
        assert {m["claude_code_version"] for m in inits} == {pin}, stream.name
    transcript = recording / f"resumed_session_{pin}" / "transcript.jsonl"
    versions = {m.get("version") for m in map(json.loads, transcript.read_text().splitlines())}
    assert versions - {None} == {pin}


def test_fake_claude_reports_the_pinned_version() -> None:
    source = (ROOT / "tests" / "fake_claude" / "fake_claude.py").read_text()
    assert re.findall(r'^PINNED = "([^"]+)"$', source, re.M) == [PINNED_CLAUDE_CODE_VERSION]


def test_fixtures_name_the_pinned_version() -> None:
    fixtures = sorted((ROOT / "tests" / "fake_claude" / "fixtures").glob("*.jsonl"))
    assert fixtures
    for path in fixtures:
        meta = json.loads(path.read_text().splitlines()[0])
        assert meta["type"] == "fixture_meta"
        assert meta["origin"] == "constructed"
        assert meta["cli_version"] == PINNED_CLAUDE_CODE_VERSION


# What a text about the pin never says (AGENTS.md, "This repository is public"). A pin moves
# when hosts install a newer CLI, so its texts are where a sentence about one such host slips
# in; they say what the CLI does, read from its file or run against the stand-in.
ONE_HOST = {
    "what an instance showed in operation": r"\bin operation\b|\blive (?:runner|instance|host)\b",
    "a machine of the owner": r"\bowner (?:already )?runs\b|\b(?:host|machine|cli) of the owner\b",
    "which host runs what since when": r"\bhost (?:runs|ran)\b.{0,80}?\bsince \d{4}-\d\d-\d\d",
    "what a host has configured": r"\bhost (?:has|had)\b.{0,80}?\bconfigured\b",
}


def _texts_about_the_pin() -> list[Path]:
    """The pin's doc comment and its test, the operator texts, the changelog and the rulings."""
    pin_source = ROOT / "src" / "claude_code_runner" / "heartbeat.py"
    rulings = set(re.findall(r"docs/decisions\.d/[\w.-]+\.md", pin_source.read_text()))
    assert rulings, "the doc comment of the pin names the fragment with its rulings"
    return [
        pin_source,
        ROOT / "tests" / "unit" / "test_heartbeat.py",
        ROOT / "README.md",
        ROOT / "config.example.toml",
        ROOT / "CHANGELOG.md",
        *sorted((ROOT / "changelog.d").glob("*.md")),
        *(ROOT.parents[1] / ruling for ruling in sorted(rulings)),
    ]


def test_texts_about_the_pin_describe_no_host_in_operation() -> None:
    found = []
    for path in _texts_about_the_pin():
        # One line of spaces: a sentence wraps, and a comment sign starts its next line.
        text = " ".join(re.sub(r"(?m)^\s*#", "", path.read_text(encoding="utf-8")).split())
        for what, pattern in ONE_HOST.items():
            found += [
                f"{path.name}: {what}: {match.group(0)}"
                for match in re.finditer(pattern, text, re.IGNORECASE)
            ]
    assert found == []
