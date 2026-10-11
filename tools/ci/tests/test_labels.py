# SPDX-License-Identifier: Apache-2.0
"""Tests for the label definition check and sync command builder."""

from __future__ import annotations

from pathlib import Path

import labels
import pytest

REPO = Path(__file__).resolve().parents[3]


def _write(tmp_path: Path, body: str) -> Path:
    p = tmp_path / "labels.yml"
    p.write_text(body)
    return p


def test_repository_labels_and_references_are_valid() -> None:
    assert labels.main(["check", "--repo", str(REPO)]) == 0


def test_duplicate_and_bad_color(tmp_path: Path) -> None:
    p = _write(
        tmp_path,
        '- {name: a, color: "zzzzzz", description: x}\n'
        '- {name: a, color: "ffffff", description: y}\n',
    )
    problems = labels.validate(labels.load(p))
    assert "a: color must be six hex digits" in problems
    assert "a: defined twice" in problems


def test_description_limit(tmp_path: Path) -> None:
    p = _write(tmp_path, f'- {{name: a, color: "ffffff", description: "{"x" * 101}"}}\n')
    assert labels.validate(labels.load(p)) == ["a: description longer than 100 characters"]


def test_unknown_label_in_template_is_reported(tmp_path: Path) -> None:
    (tmp_path / ".github" / "ISSUE_TEMPLATE").mkdir(parents=True)
    (tmp_path / ".github" / "labels.yml").write_text(
        '- {name: "type: bug", color: "ffffff", description: x}\n'
    )
    (tmp_path / ".github" / "ISSUE_TEMPLATE" / "bug.yml").write_text(
        'name: Bug\nlabels: ["type: bug", "status: triage"]\nbody: []\n'
    )
    (tmp_path / "GOVERNANCE.md").write_text("Use `type: bug` and `priority: urgent`.\n")
    problems = labels.reference_problems(tmp_path)
    assert problems == [
        "GOVERNANCE.md: unknown label 'priority: urgent'",
        ".github/ISSUE_TEMPLATE/bug.yml: unknown label 'status: triage'",
    ]


def test_sync_commands(tmp_path: Path) -> None:
    p = _write(tmp_path, '- {name: "type: bug", color: "d73a4a", description: "Broken"}\n')
    assert labels.sync_commands(labels.load(p)) == [
        [
            "gh",
            "label",
            "create",
            "type: bug",
            "--color",
            "d73a4a",
            "--description",
            "Broken",
            "--force",
        ]
    ]


def test_sync_dry_run_prints(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (tmp_path / ".github").mkdir()
    (tmp_path / ".github" / "labels.yml").write_text(
        '- {name: x, color: "ffffff", description: y}\n'
    )
    assert labels.main(["sync", "--repo", str(tmp_path), "--dry-run"]) == 0
    assert "gh label create x" in capsys.readouterr().out
