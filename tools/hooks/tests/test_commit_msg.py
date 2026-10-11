# SPDX-License-Identifier: Apache-2.0
"""Tests for the Conventional Commits and DCO sign-off check."""

from __future__ import annotations

import subprocess
from pathlib import Path

import commit_msg
import pytest

ALEX = "Alex Example <alex@example.com>"


def _check(
    message: str, *, signoff: bool = False, identity: str = ALEX, allow_fixup: bool = True
) -> list[str]:
    return commit_msg.check_message(
        message, require_signoff=signoff, author=identity, allow_fixup=allow_fixup
    )


@pytest.mark.parametrize(
    "header",
    [
        "feat: add thread close button",
        "fix(sync): resume after reconnect",
        "docs(adr)!: replace MkDocs",
        "chore(deps): update uv.lock",
        "revert: feat: add thread close button",
    ],
)
def test_valid_headers(header: str) -> None:
    assert _check(header + "\n") == []


@pytest.mark.parametrize(
    ("header", "reason"),
    [
        ("Add thread close button", "type"),
        ("feature: add x", "type"),
        ("feat:add x", "type"),
        ("feat(): add x", "type"),
        ("feat: ", "description"),
        ("fix CI", "type"),
        ("typo", "type"),
    ],
)
def test_invalid_headers(header: str, reason: str) -> None:
    errors = _check(header + "\n")
    assert errors
    assert reason in errors[0]


def test_header_length_limit() -> None:
    header = "feat: " + "x" * commit_msg.HEADER_MAX_LENGTH
    assert any("longer" in e for e in _check(header))


def test_body_needs_blank_line() -> None:
    assert any("blank line" in e for e in _check("feat: x\nbody without gap\n"))


def test_comments_and_scissors_are_ignored() -> None:
    scissors = "# " + "-" * 24 + " >8 " + "-" * 24
    msg = f"feat: x\n\n# Please enter the commit message\n{scissors}\ndiff a b\n"
    assert _check(msg) == []


def test_signoff_required_and_matching() -> None:
    msg = f"feat: x\n\nbody\n\nSigned-off-by: {ALEX}\n"
    assert _check(msg, signoff=True) == []


def test_signoff_missing() -> None:
    assert any("Signed-off-by" in e for e in _check("feat: x\n", signoff=True))


def test_signoff_of_other_person_is_not_enough() -> None:
    msg = "feat: x\n\nSigned-off-by: Sam Example <sam@example.com>\n"
    assert any("Signed-off-by" in e for e in _check(msg, signoff=True))


def test_signoff_email_comparison_ignores_case() -> None:
    msg = "feat: x\n\nSigned-off-by: Alex Example <ALEX@example.com>\n"
    assert _check(msg, signoff=True) == []


def test_fixup_allowed_locally_rejected_in_ranges() -> None:
    assert _check("fixup! feat: x\n") == []
    assert any("fixup" in e for e in _check("fixup! feat: x\n", allow_fixup=False))


def test_merge_commit_message_is_accepted() -> None:
    assert _check("Merge branch 'main' into feat/x\n") == []


def _git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    _git(tmp_path, "init", "-q", "-b", "main")
    _git(tmp_path, "config", "user.name", "Alex Example")
    _git(tmp_path, "config", "user.email", "alex@example.com")
    _git(tmp_path, "config", "commit.gpgsign", "false")
    _git(tmp_path, "config", "core.hooksPath", "/dev/null")
    (tmp_path / "a").write_text("a\n")
    _git(tmp_path, "add", "a")
    _git(tmp_path, "commit", "-q", "-m", "chore: start")
    return tmp_path


def test_range_mode_checks_every_commit(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    base = _git(repo, "rev-parse", "HEAD").strip()
    (repo / "b").write_text("b\n")
    _git(repo, "add", "b")
    _git(repo, "commit", "-q", "-s", "-m", "feat: add b")
    (repo / "c").write_text("c\n")
    _git(repo, "add", "c")
    _git(repo, "commit", "-q", "-m", "fix CI")
    rc = commit_msg.main(["--repo", str(repo), "--range", f"{base}..HEAD", "--require-signoff"])
    out = capsys.readouterr().out
    assert rc == 1
    assert "fix CI" in out
    assert "add b" not in out


def test_range_mode_passes_for_clean_range(repo: Path) -> None:
    base = _git(repo, "rev-parse", "HEAD").strip()
    (repo / "b").write_text("b\n")
    _git(repo, "add", "b")
    _git(repo, "commit", "-q", "-s", "-m", "feat: add b")
    assert (
        commit_msg.main(["--repo", str(repo), "--range", f"{base}..HEAD", "--require-signoff"]) == 0
    )


def test_file_mode_reads_message_file(repo: Path, tmp_path: Path) -> None:
    msg = tmp_path / "MSG"
    msg.write_text("feat: x\n\nSigned-off-by: Alex Example <alex@example.com>\n")
    assert commit_msg.main(["--repo", str(repo), "--require-signoff", str(msg)]) == 0
