# SPDX-License-Identifier: Apache-2.0
"""The pre-push hook runs only the fast gates for a push to build/** (build_push.py)."""

from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

import build_push
import pytest
from git_fixtures import commit, git, init_repo

ENV_NAMES = ("PRE_COMMIT_REMOTE_BRANCH", "PRE_COMMIT_TO_REF", "PRE_COMMIT_LOCAL_BRANCH")
TODAY = ["env", "-u", "GIT_DIR", "make", "check"]


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in ENV_NAMES:
        monkeypatch.delenv(name, raising=False)


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    """A repository whose origin/main is its first commit."""
    init_repo(tmp_path)
    (tmp_path / "README.md").write_text("start\n", encoding="utf-8")
    commit(tmp_path, "docs: start")
    git(tmp_path, "update-ref", "refs/remotes/origin/main", "HEAD")
    return tmp_path


def change(repo: Path, *paths: str) -> None:
    for rel in paths:
        target = repo / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(f"{rel}\n", encoding="utf-8")
    commit(repo, "feat: change")


@pytest.mark.parametrize(
    "ref",
    [
        "refs/heads/build/M2-44",
        "refs/heads/build/M2-44/retry",
        "build/M4-31",
        # The base ref of a build branch (tools/ci/build_base.py), pushed on its own before it.
        "refs/heads/build-base/M2-44",
        "build-base/M4-31",
    ],
)
def test_build_refs_are_selected(ref: str) -> None:
    assert build_push.is_build_ref(ref)


@pytest.mark.parametrize(
    "ref",
    [
        "",
        "refs/heads/main",
        "refs/heads/feat/M2-44-relay",
        "refs/heads/build",
        "refs/heads/build/",
        "refs/heads/rebuild/M2-44",
        "refs/heads/feat/build/M2-44",
        "refs/tags/build/v1.0.0",
        "refs/heads/build-base",
        "refs/heads/build-base/",
        "refs/heads/build-basex/M2-44",
        "refs/tags/build-base/M2-44",
    ],
)
def test_other_refs_are_not_selected(ref: str) -> None:
    assert not build_push.is_build_ref(ref)


def test_docs_change_runs_the_fast_gates_without_codegen(repo: Path) -> None:
    change(repo, "docs/quality/gates.md")
    assert build_push.fast_make_args(repo, "HEAD") == [
        "denylist",
        "gitleaks",
        "commitlint",
        "lint",
        "COMPONENTS=none",
    ]


def test_component_change_lints_and_checks_codegen_of_that_component(repo: Path) -> None:
    change(repo, "services/server/internal/x.go")
    assert build_push.fast_make_args(repo, "HEAD") == [
        "denylist",
        "gitleaks",
        "commitlint",
        "codegen-check",
        "lint",
        "COMPONENTS=sdk-python sdk-typescript server",
    ]


def test_generator_input_outside_a_component_runs_codegen(repo: Path) -> None:
    change(repo, "docs/operations/configuration.md")
    args = build_push.fast_make_args(repo, "HEAD")
    assert "codegen-check" in args
    assert args[-1] == "COMPONENTS=none"


def test_shared_input_selects_every_component(repo: Path) -> None:
    change(repo, "Makefile")
    args = build_push.fast_make_args(repo, "HEAD")
    assert "codegen-check" in args
    assert args[-1] == "COMPONENTS=all"


def test_diff_starts_at_the_merge_base_with_main(repo: Path) -> None:
    # A newer main does not add its own files to the pushed diff.
    git(repo, "checkout", "-q", "-b", "feat/x")
    change(repo, "docs/a.md")
    git(repo, "checkout", "-q", "main")
    change(repo, "services/server/y.go")
    git(repo, "update-ref", "refs/remotes/origin/main", "HEAD")
    assert build_push.fast_make_args(repo, "feat/x")[-1] == "COMPONENTS=none"


def test_without_origin_main_every_component_is_selected(repo: Path) -> None:
    git(repo, "update-ref", "-d", "refs/remotes/origin/main")
    assert build_push.fast_make_args(repo, "HEAD")[-1] == "COMPONENTS=all"


def _record(monkeypatch: pytest.MonkeyPatch) -> list[dict[str, Any]]:
    calls: list[dict[str, Any]] = []
    real_run = subprocess.run

    def fake_run(cmd: list[str], **kwargs: Any) -> subprocess.CompletedProcess[str]:
        if cmd[0] == "git":
            return real_run(cmd, **kwargs)  # type: ignore[no-any-return]
        calls.append({"cmd": cmd, **kwargs})
        return subprocess.CompletedProcess(cmd, 3)

    monkeypatch.setattr(build_push.subprocess, "run", fake_run)
    return calls


@pytest.mark.parametrize("ref", ["", "refs/heads/feat/M2-44-relay", "refs/heads/main"])
def test_every_other_ref_runs_the_command_of_today(
    monkeypatch: pytest.MonkeyPatch, repo: Path, ref: str
) -> None:
    monkeypatch.setenv("PRE_COMMIT_REMOTE_BRANCH", ref)
    calls = _record(monkeypatch)
    assert build_push.main(["--repo", str(repo), "--", *TODAY]) == 3
    assert [c["cmd"] for c in calls] == [TODAY]


def test_build_ref_runs_make_with_the_fast_gates_only(
    monkeypatch: pytest.MonkeyPatch, repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    change(repo, "apps/client/lib/main.dart")
    monkeypatch.setenv("PRE_COMMIT_REMOTE_BRANCH", "refs/heads/build/M4-31")
    monkeypatch.setenv("PRE_COMMIT_TO_REF", git(repo, "rev-parse", "HEAD").strip())
    for name in build_push.HOOK_VARIABLES:
        monkeypatch.setenv(name, "/nonexistent")
    calls = _record(monkeypatch)
    assert build_push.main(["--repo", str(repo), "--", *TODAY]) == 3
    [call] = calls
    assert call["cmd"] == [
        "make",
        "denylist",
        "gitleaks",
        "commitlint",
        "codegen-check",
        "lint",
        "COMPONENTS=apps",
    ]
    assert call["cwd"] == repo
    assert not set(call["env"]) & set(build_push.HOOK_VARIABLES)
    assert "build-verify.yml" in capsys.readouterr().out


def test_a_command_is_required() -> None:
    with pytest.raises(SystemExit):
        build_push.main([])


def test_the_pre_push_hook_runs_the_push_plan_through_the_selection() -> None:
    import yaml

    repo = Path(__file__).resolve().parents[3]
    config = yaml.safe_load((repo / ".pre-commit-config.yaml").read_text(encoding="utf-8"))
    entries = {
        hook["id"]: str(hook.get("entry", "")).split()
        for block in config["repos"]
        for hook in block.get("hooks", [])
    }
    entry = entries["make-check"]
    split = entry.index("--")
    assert entry[split - 1] == "tools/hooks/build_push.py"
    # Every other push runs the affected-only plan of make check (tools/hooks/push_plan.py).
    assert entry[split + 1 :][-3:] == ["python", "tools/hooks/push_plan.py", "run"]
