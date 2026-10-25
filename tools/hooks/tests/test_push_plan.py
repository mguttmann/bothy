# SPDX-License-Identifier: Apache-2.0
"""Path mapping of the pre-push gate: which gates run for which pushed files."""

from __future__ import annotations

import subprocess
from pathlib import Path

import git_fixtures
import push_plan
import pytest


@pytest.fixture(autouse=True)
def _no_push_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """The pre-push hook exports these to `make test`; every test here starts without them."""
    for name in ("PUSH_FULL", "PRE_COMMIT_TO_REF", "PRE_COMMIT_REMOTE_BRANCH"):
        monkeypatch.delenv(name, raising=False)


@pytest.mark.parametrize(
    ("files", "components", "areas"),
    [
        # Source files of a component alone: only its gates, no tools gates. Both SDKs and the
        # plugins also own services/server/: their real-server and live tests run the server of
        # the tree (M2-29, M2-30, M2-49).
        (
            ["services/server/internal/chat/x.go"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
            [],
        ),
        (
            ["services/server/internal/chat/x_test.go", "services/host/main.go"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
            [],
        ),
        (["apps/client/lib/main.dart", "packages/design_system/lib/a.dart"], ["apps"], []),
        # The agent plugins import the Python SDK from its source tree (M2-33).
        (["sdk/python/src/bothy/client.py"], ["plugins", "sdk-python", "server"], []),
        (["sdk/typescript/src/x.ts"], ["plugins", "sdk-typescript"], []),
        (["plugins/hermes-adapter/adapter.py"], ["plugins"], []),
        # Manifests, configuration, test data and docs of a component are read by tools tests.
        (
            ["services/server/go.mod"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
            ["tools"],
        ),
        (
            ["services/server/Dockerfile"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
            ["tools"],
        ),
        (
            ["services/server/.golangci.yml"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
            ["tools"],
        ),
        (
            ["services/server/CHANGELOG.md", "services/server/x.go"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
            ["tools"],
        ),
        (["packages/design_system/test/goldens/a.png"], ["apps"], ["tools"]),
        (["sdk/python/pyproject.toml"], ["plugins", "sdk-python", "server"], ["tools"]),
        # Docs: no component, the tools gates and every repository gate.
        (["docs/spec/07-permissions.md"], [], ["tools"]),
        (["docs/DECISIONS.md", "README.md"], [], ["tools"]),
        # The terminal suite of the server starts the Compose stack and its collector (M2-48).
        (["infra/compose/secrets.yaml"], ["server"], ["tools"]),
        (["infra/otel/collector.yaml"], ["server"], ["tools"]),
        # The Grafana pin of the Compose stack is an input of the panel plugin's tests (M7-25).
        (["infra/compose/compose.yaml"], ["plugins", "server"], ["tools"]),
        # Inputs of generated code outside the component select the component (codegen-check).
        (["docs/spec/actions.yaml"], ["server"], ["tools"]),
        (["pubspec.lock"], ["apps"], ["tools"]),
        (["design/tokens/color.json"], ["apps"], ["tools"]),
        (
            ["docs/api/openapi.yaml"],
            ["apps", "plugins", "sdk-python", "sdk-typescript", "server"],
            ["tools"],
        ),
    ],
)
def test_affected_plan(files: list[str], components: list[str], areas: list[str]) -> None:
    plan = push_plan.plan(files)
    assert not plan.full, plan.reason
    assert plan.components == components
    assert plan.areas == areas


@pytest.mark.parametrize(
    "files",
    [
        ["Makefile"],
        [".pre-commit-config.yaml"],
        ["tools/uv.lock"],
        ["tools/pyproject.toml"],
        ["tools/node/package-lock.json"],
        ["tools/bin/tools.lock.json"],
        ["tools/hooks/push_plan.py"],
        ["tools/ci/components.py"],
        [".github/workflows/ci.yml"],
        [".github/labels.yml"],
        [".semgrep/bothy-go-tls-verify.yaml"],
        ["quality/sonar/quality-gate.json"],
        ["sonar-project.properties"],
        [".gitleaks.toml"],
        ["mkdocs.yml"],
        ["some-new-top-level.toml"],
        # One shared input in a push of component files is enough.
        ["services/server/x.go", "Makefile"],
    ],
)
def test_shared_inputs_run_the_full_check(files: list[str]) -> None:
    plan = push_plan.plan(files)
    assert plan.full
    assert plan.make_args("a", "b") == ["check"]


def test_make_args_of_an_affected_plan() -> None:
    plan = push_plan.plan(["services/server/x.go"])
    assert plan.make_args() == [
        "check",
        "COMPONENTS=plugins sdk-python sdk-typescript server",
        "AREAS=none",
    ]
    assert plan.make_args("b1", "h1") == [
        "check",
        "COMPONENTS=plugins sdk-python sdk-typescript server",
        "AREAS=none",
        "LINT_FROM_REF=b1",
        "LINT_TO_REF=h1",
    ]
    assert push_plan.plan(["docs/spec/x.md"]).make_args() == [
        "check",
        "COMPONENTS=none",
        "AREAS=tools",
    ]


def test_plan_command_prints_the_make_arguments(capsys: pytest.CaptureFixture[str]) -> None:
    assert push_plan.main(["plan", "--files", "apps/client/lib/a.dart"]) == 0
    assert capsys.readouterr().out.strip() == "check COMPONENTS=apps AREAS=none"


def _repo_with_main(tmp_path: Path) -> Path:
    """A clone whose origin/main is the first commit; HEAD is a branch with one more commit."""
    upstream = git_fixtures.init_repo(tmp_path / "upstream")
    (upstream / "README.md").write_text("x\n", encoding="utf-8")
    git_fixtures.commit(upstream, "docs: start")
    clone = tmp_path / "clone"
    subprocess.run(["git", "clone", "-q", str(upstream), str(clone)], check=True, timeout=60)
    for key, value in (("user.name", "A"), ("user.email", "a@example.com")):
        git_fixtures.git(clone, "config", key, value)
    git_fixtures.git(clone, "config", "commit.gpgsign", "false")
    git_fixtures.git(clone, "checkout", "-q", "-b", "feat/x")
    path = clone / "services/server/x.go"
    path.parent.mkdir(parents=True)
    path.write_text("package x\n", encoding="utf-8")
    git_fixtures.commit(clone, "feat: x")
    return clone


def test_push_plan_reads_the_diff_against_origin_main(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    repo = _repo_with_main(tmp_path)
    plan, base, head = push_plan.push_plan(repo)
    assert not plan.full
    assert plan.components == ["plugins", "sdk-python", "sdk-typescript", "server"]
    assert base == git_fixtures.git(repo, "rev-parse", "origin/main").strip()
    assert head == git_fixtures.git(repo, "rev-parse", "HEAD").strip()
    monkeypatch.setenv("PRE_COMMIT_TO_REF", head)
    assert push_plan.push_plan(repo)[0].components == [
        "plugins",
        "sdk-python",
        "sdk-typescript",
        "server",
    ]


@pytest.mark.parametrize(
    ("env", "reason"),
    [
        ({"PUSH_FULL": "1"}, "PUSH_FULL=1"),
        ({"PRE_COMMIT_REMOTE_BRANCH": "refs/tags/v1.0.0"}, "a tag push"),
        ({"PRE_COMMIT_TO_REF": "0" * 40}, "no merge base"),
    ],
)
def test_push_plan_falls_back_to_the_full_check(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, env: dict[str, str], reason: str
) -> None:
    repo = _repo_with_main(tmp_path)
    for name, value in env.items():
        monkeypatch.setenv(name, value)
    plan, base, head = push_plan.push_plan(repo)
    assert plan.full
    assert reason in plan.reason
    assert (base, head) == ("", "")


def test_push_plan_without_origin_main_runs_the_full_check(tmp_path: Path) -> None:
    repo = git_fixtures.init_repo(tmp_path / "solo")
    git_fixtures.commit(repo, "docs: start")
    plan, _, _ = push_plan.push_plan(repo)
    assert plan.full
    assert "no merge base" in plan.reason


def test_push_plan_without_difference_runs_the_full_check(tmp_path: Path) -> None:
    repo = _repo_with_main(tmp_path)
    git_fixtures.git(repo, "checkout", "-q", "origin/main")
    plan, _, _ = push_plan.push_plan(repo)
    assert plan.full
    assert "no difference" in plan.reason


def test_run_executes_make_with_the_plan(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    calls: list[list[str]] = []

    def fake_run(cmd: list[str], **_: object) -> subprocess.CompletedProcess[str]:
        calls.append(cmd)
        return subprocess.CompletedProcess(cmd, 2)

    affected = push_plan.Plan(full=False, reason="components server", components=["server"])
    monkeypatch.setattr(push_plan, "push_plan", lambda repo: (affected, "b", "h"))
    monkeypatch.setattr(push_plan.subprocess, "run", fake_run)
    assert push_plan.main(["run", "--repo", str(tmp_path)]) == 2
    assert calls == [
        [
            "make",
            "check",
            "COMPONENTS=server",
            "AREAS=none",
            "LINT_FROM_REF=b",
            "LINT_TO_REF=h",
        ]
    ]
    calls.clear()
    assert push_plan.main(["run", "--repo", str(tmp_path), "--dry-run"]) == 0
    assert calls == []


def test_a_rename_counts_with_its_old_path(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Moving a file out of tools/ is a change of tools/ as well (git diff --no-renames)."""
    repo = _repo_with_main(tmp_path)
    tool = repo / "tools/ci/x.py"
    tool.parent.mkdir(parents=True)
    tool.write_text("print('x')\n", encoding="utf-8")
    git_fixtures.git(repo, "add", "-A")
    git_fixtures.git(repo, "commit", "-q", "-m", "chore: add a tool")
    # The tool is on main now; the branch below only moves it.
    git_fixtures.git(repo, "update-ref", "refs/remotes/origin/main", "HEAD")
    git_fixtures.git(repo, "checkout", "-q", "-b", "move")
    (repo / "plugins/p").mkdir(parents=True)
    git_fixtures.git(repo, "mv", "tools/ci/x.py", "plugins/p/x.py")
    git_fixtures.git(repo, "commit", "-q", "-m", "refactor: move the tool")
    plan, _, _ = push_plan.push_plan(repo)
    assert plan.full, plan.reason


def test_the_tools_tests_of_component_sources_exist() -> None:
    """Every file of TOOLS_SOURCE_TESTS (Makefile) is a tools test, so the list cannot go stale."""
    repo = Path(__file__).resolve().parents[3]
    text = (repo / "Makefile").read_text(encoding="utf-8")
    start = text.index("TOOLS_SOURCE_TESTS := ")
    end = text.index("\n", text.index("\\\n", start) + 2)
    names = text[start:end].removeprefix("TOOLS_SOURCE_TESTS := ").replace("\\\n", " ").split()
    assert len(names) == 3
    for name in names:
        assert (repo / "tools" / name).is_file(), name
