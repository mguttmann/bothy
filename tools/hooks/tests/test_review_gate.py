# SPDX-License-Identifier: Apache-2.0
"""Tests for the pre-push review evidence gate (docs/quality/red-team-gate.md)."""

from __future__ import annotations

import subprocess
from pathlib import Path

import pytest
import review_gate

ENV_NAMES = (
    "PRE_COMMIT_TO_REF",
    "PRE_COMMIT_LOCAL_BRANCH",
    "PRE_COMMIT_REMOTE_BRANCH",
    "REVIEW_SKIP",
)


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in ENV_NAMES:
        monkeypatch.delenv(name, raising=False)


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    def git(*args: str) -> None:
        subprocess.run(["git", "-C", str(tmp_path), *args], check=True, capture_output=True)

    git("init", "-q", "-b", "main")
    git("config", "user.name", "Alex Example")
    git("config", "user.email", "alex@example.com")
    git("config", "commit.gpgsign", "false")
    git("commit", "-q", "--allow-empty", "-m", "docs: start")
    return tmp_path


def head(repo: Path) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), "rev-parse", "HEAD"], check=True, capture_output=True, text=True
    ).stdout.strip()


def write_report(repo: Path, sha: str, verdict: str) -> None:
    (repo / ".review").mkdir(exist_ok=True)
    (repo / ".review" / f"{sha}.md").write_text(
        f"# Review\n\nFindings: none.\n\n{verdict}\n", encoding="utf-8"
    )


def test_push_without_report_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert review_gate.main(["--repo", str(repo)]) == 1
    assert f".review/{head(repo)}.md" in capsys.readouterr().out


def test_report_with_yes_passes(repo: Path) -> None:
    write_report(repo, head(repo), "Urteil: JA")
    assert review_gate.main(["--repo", str(repo)]) == 0


@pytest.mark.parametrize("verdict", ["Urteil: NEIN offene Rechte-Lücke", "Urteil: ja", "JA"])
def test_report_without_final_yes_fails(repo: Path, verdict: str) -> None:
    write_report(repo, head(repo), verdict)
    assert review_gate.main(["--repo", str(repo)]) == 1


def test_yes_must_be_the_last_line(repo: Path) -> None:
    write_report(repo, head(repo), "Urteil: JA\n\nNachtrag: Urteil: NEIN")
    assert review_gate.main(["--repo", str(repo)]) == 1


def test_pushed_sha_comes_from_pre_commit(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    old = head(repo)
    subprocess.run(
        ["git", "-C", str(repo), "commit", "-q", "--allow-empty", "-m", "docs: next"], check=True
    )
    write_report(repo, head(repo), "Urteil: JA")
    monkeypatch.setenv("PRE_COMMIT_TO_REF", old)
    assert review_gate.main(["--repo", str(repo)]) == 1


def test_local_branch_is_used_without_to_ref(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    write_report(repo, head(repo), "Urteil: JA")
    monkeypatch.setenv("PRE_COMMIT_LOCAL_BRANCH", "refs/heads/main")
    assert review_gate.main(["--repo", str(repo)]) == 0


def test_tag_push_needs_no_report(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("PRE_COMMIT_REMOTE_BRANCH", "refs/tags/v1.0.0")
    assert review_gate.main(["--repo", str(repo)]) == 0


def test_fork_switch_is_explicit_and_visible(
    repo: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setenv("REVIEW_SKIP", "1")
    assert review_gate.main(["--repo", str(repo)]) == 0
    assert "WARNING" in capsys.readouterr().out
