# SPDX-License-Identifier: Apache-2.0
"""Folder creation of a binding with provision init or clone (09 18.12, M4-43).

Clones and failures run a fake ``git`` (a script that records its arguments, working folder
and environment and then acts as told by the file ``mode`` next to it); ``init`` also runs the
installed ``git`` when there is one. No test touches the network.
"""

from __future__ import annotations

import json
import os
import shutil
import stat
import sys
from pathlib import Path
from uuid import UUID, uuid4

import pytest

from claude_code_runner.environment import clean_path, find_program
from claude_code_runner.provision import Provisioner, Want

URL = "https://git.example.com/team/website.git"

_FAKE_GIT = """#!{python}
import json, os, pathlib, sys, time
here = pathlib.Path(__file__).resolve().parent
mode = (here / "mode").read_text().strip() if (here / "mode").exists() else "ok"
with open(here / "calls.jsonl", "a") as fh:
    call = {{"argv": sys.argv[1:], "cwd": os.getcwd(), "env": dict(os.environ)}}
    fh.write(json.dumps(call) + "\\n")
if mode == "hang":
    time.sleep(60)
if mode == "fail":
    sys.exit(128)
if sys.argv[1] == "clone":
    target = pathlib.Path(sys.argv[-1])
    (target / ".git").mkdir(parents=True)
    (target / "README.md").write_text("cloned")
elif sys.argv[1] == "init":
    pathlib.Path(".git").mkdir()
"""


class FakeGit:
    """The fake ``git`` in its own folder with its call log."""

    def __init__(self, base: Path) -> None:
        """Write the script into ``base``."""
        base.mkdir()
        self.dir = base
        self.path = base / "git"
        self.path.write_text(_FAKE_GIT.format(python=sys.executable))
        self.path.chmod(0o755)

    def mode(self, value: str) -> None:
        """Set what the next calls do: ``ok``, ``fail`` or ``hang``."""
        (self.dir / "mode").write_text(value)

    def calls(self) -> list[dict[str, object]]:
        """Every recorded call."""
        log = self.dir / "calls.jsonl"
        if not log.exists():
            return []
        return [json.loads(line) for line in log.read_text().splitlines()]


class Clock:
    """A monotonic clock the tests move."""

    def __init__(self) -> None:
        """Start at zero."""
        self.now = 0.0

    def __call__(self) -> float:
        """The current time."""
        return self.now


@pytest.fixture
def root(tmp_path: Path) -> Path:
    r = tmp_path / "projects"
    r.mkdir()
    return r


@pytest.fixture
def state(tmp_path: Path) -> Path:
    s = tmp_path / "state"
    s.mkdir(mode=0o700)
    return s


@pytest.fixture
def git(tmp_path: Path) -> FakeGit:
    return FakeGit(tmp_path / "fakegit")


ENVIRON = {
    "HOME": "/home/alex",
    "PATH": os.environ.get("PATH", ""),
    "SECRET_TOKEN": "x",
    "LANG": "C",
}


def make(
    root: Path,
    state: Path,
    git_bin: str | None,
    *,
    timeout: float = 30.0,
    clock: Clock | None = None,
    done: list[UUID] | None = None,
) -> Provisioner:
    async def on_done() -> None:
        if done is not None:
            done.append(uuid4())

    return Provisioner(
        projects_root=root,
        state_dir=state,
        git_bin=git_bin,
        environ=ENVIRON,
        timeout=timeout,
        retry_after=300.0,
        clock=clock or Clock(),
        on_done=on_done,
    )


def leftovers(root: Path) -> list[str]:
    return sorted(p.name for p in root.iterdir() if p.name.startswith(".provision-"))


@pytest.mark.anyio
async def test_init_with_installed_git(root: Path, state: Path) -> None:
    real = find_program("git", clean_path(os.environ.get("PATH", "")))
    if real is None:
        pytest.skip("no git installed")
    done: list[UUID] = []
    p = make(root, state, real, done=done)
    channel = uuid4()
    want = Want("website", "init", None, "main")
    assert p.readiness(channel, want) == (False, "provisioning")
    await p.wait_idle()
    assert p.readiness(channel, want) == (True, None)
    assert (root / "website" / ".git").is_dir()
    assert len(done) == 1
    record = state / "provisioned.json"
    assert stat.S_IMODE(record.stat().st_mode) == 0o600
    assert json.loads(record.read_text()) == {str(channel): "website"}


@pytest.mark.anyio
async def test_init_arguments(root: Path, state: Path, git: FakeGit) -> None:
    p = make(root, state, str(git.path))
    channel = uuid4()
    p.readiness(channel, Want("website", "init", None, "trunk"))
    await p.wait_idle()
    (call,) = git.calls()
    assert call["argv"] == ["init", "--quiet", "--initial-branch", "trunk"]
    assert Path(str(call["cwd"])).resolve() == (root / "website").resolve()
    p.readiness(uuid4(), Want("docs", "init", None, None))
    await p.wait_idle()
    assert git.calls()[-1]["argv"] == ["init", "--quiet"]


@pytest.mark.anyio
async def test_clone_arguments_environment_and_rename(
    root: Path, state: Path, git: FakeGit
) -> None:
    done: list[UUID] = []
    p = make(root, state, str(git.path), done=done)
    channel = uuid4()
    want = Want("website", "clone", URL, "main")
    assert p.readiness(channel, want) == (False, "provisioning")
    await p.wait_idle()
    (call,) = git.calls()
    argv = call["argv"]
    assert isinstance(argv, list)
    assert argv[:5] == ["clone", "--quiet", "--no-recurse-submodules", "--", URL]
    tmp = Path(argv[5])
    assert tmp.parent.resolve() == root.resolve()
    assert tmp.name.startswith(".provision-")
    env = call["env"]
    assert isinstance(env, dict)
    assert env["GIT_TERMINAL_PROMPT"] == "0"
    assert env["GIT_ALLOW_PROTOCOL"] == "https"
    assert "SECRET_TOKEN" not in env
    assert env["HOME"] == "/home/alex"
    assert (root / "website" / "README.md").read_text() == "cloned"
    assert leftovers(root) == []
    assert p.readiness(channel, want) == (True, None)
    assert len(done) == 1


@pytest.mark.anyio
async def test_existing_folder_is_never_taken(root: Path, state: Path, git: FakeGit) -> None:
    (root / "website").mkdir()
    (root / "website" / "notes.txt").write_text("mine")
    p = make(root, state, str(git.path))
    for kind in ("init", "clone"):
        assert p.readiness(uuid4(), Want("website", kind, URL, None)) == (False, "workspace_exists")
    await p.wait_idle()
    assert git.calls() == []
    assert sorted(x.name for x in (root / "website").iterdir()) == ["notes.txt"]
    (root / "dangling").symlink_to(root / "nowhere")
    assert p.readiness(uuid4(), Want("dangling", "init", None, None)) == (False, "workspace_exists")


@pytest.mark.parametrize(
    "want",
    [
        Want("team/website", "init", None, None),
        Want(".website", "init", None, None),
        Want("-website", "init", None, None),
        Want("web site", "init", None, None),
        Want("..", "init", None, None),
        Want("website", "init", None, "-x"),
        Want("website", "clone", None, None),
        Want("website", "clone", "http://git.example.com/a", None),
        Want("website", "clone", "https://alex:secret@git.example.com/a", None),
        Want("website", "clone", "https://git.example.com/a?x=1", None),
        Want("website", "clone", "https://git.example.com/a#x", None),
        Want("website", "clone", "ext::sh -c touch% /tmp/x", None),
        Want("website", "clone", "https://git.example.com/a b", None),
        Want("website", "clone", "https:///a", None),
        Want("website", "copy", None, None),
    ],
)
@pytest.mark.anyio
async def test_refused_never_runs_git(root: Path, state: Path, git: FakeGit, want: Want) -> None:
    p = make(root, state, str(git.path))
    assert p.readiness(uuid4(), want) == (False, "provision_refused")
    await p.wait_idle()
    assert git.calls() == []
    assert list(root.iterdir()) == []


@pytest.mark.anyio
async def test_time_limit(root: Path, state: Path, git: FakeGit) -> None:
    git.mode("hang")
    p = make(root, state, str(git.path), timeout=0.5)
    channel = uuid4()
    want = Want("website", "clone", URL, None)
    p.readiness(channel, want)
    await p.wait_idle()
    assert p.readiness(channel, want) == (False, "provision_timeout")
    assert list(root.iterdir()) == []


@pytest.mark.anyio
async def test_failure_removes_only_its_folder_and_waits(
    root: Path, state: Path, git: FakeGit
) -> None:
    (root / "other").mkdir()
    git.mode("fail")
    clock = Clock()
    p = make(root, state, str(git.path), clock=clock)
    channel = uuid4()
    want = Want("website", "clone", URL, None)
    p.readiness(channel, want)
    await p.wait_idle()
    assert p.readiness(channel, want) == (False, "clone_failed")
    assert sorted(x.name for x in root.iterdir()) == ["other"]
    clock.now += 299
    assert p.readiness(channel, want) == (False, "clone_failed")
    await p.wait_idle()
    assert len(git.calls()) == 1
    git.mode("ok")
    clock.now += 2
    assert p.readiness(channel, want) == (False, "provisioning")
    await p.wait_idle()
    assert len(git.calls()) == 2
    assert p.readiness(channel, want) == (True, None)


@pytest.mark.anyio
async def test_changed_binding_tries_again(root: Path, state: Path, git: FakeGit) -> None:
    git.mode("fail")
    p = make(root, state, str(git.path))
    channel = uuid4()
    p.readiness(channel, Want("website", "clone", URL, None))
    await p.wait_idle()
    git.mode("ok")
    assert p.readiness(channel, Want("website", "clone", URL + "x", None)) == (
        False,
        "provisioning",
    )
    await p.wait_idle()
    assert len(git.calls()) == 2


@pytest.mark.anyio
async def test_init_failure_removes_the_new_folder(root: Path, state: Path, git: FakeGit) -> None:
    git.mode("fail")
    p = make(root, state, str(git.path))
    channel = uuid4()
    want = Want("website", "init", None, None)
    p.readiness(channel, want)
    await p.wait_idle()
    assert p.readiness(channel, want) == (False, "init_failed")
    assert list(root.iterdir()) == []


@pytest.mark.anyio
async def test_missing_git(root: Path, state: Path) -> None:
    p = make(root, state, None)
    channel = uuid4()
    want = Want("website", "init", None, None)
    p.readiness(channel, want)
    await p.wait_idle()
    assert p.readiness(channel, want) == (False, "init_failed")
    assert list(root.iterdir()) == []


@pytest.mark.anyio
async def test_record_survives_a_restart(root: Path, state: Path, git: FakeGit) -> None:
    p = make(root, state, str(git.path))
    channel = uuid4()
    want = Want("website", "init", None, None)
    p.readiness(channel, want)
    await p.wait_idle()
    again = make(root, state, str(git.path))
    assert again.readiness(channel, want) == (True, None)
    assert again.readiness(uuid4(), want) == (False, "workspace_exists")
    await again.wait_idle()
    assert len(git.calls()) == 1


@pytest.mark.anyio
async def test_none_is_plain_readiness(root: Path, state: Path, git: FakeGit) -> None:
    p = make(root, state, str(git.path))
    assert p.readiness(uuid4(), Want("website", "none", None, None)) == (False, "workspace_missing")
    (root / "team" / "website" / ".git").mkdir(parents=True)
    assert p.readiness(uuid4(), Want("team/website", "none", None, None)) == (True, None)
    await p.wait_idle()
    assert git.calls() == []


@pytest.mark.anyio
async def test_close_stops_a_running_creation(root: Path, state: Path, git: FakeGit) -> None:
    git.mode("hang")
    p = make(root, state, str(git.path))
    p.readiness(uuid4(), Want("website", "clone", URL, None))
    await p.close()
    assert list(root.iterdir()) == []


def test_fake_git_is_not_on_the_path(git: FakeGit) -> None:
    # The fake is reached only through runner.git_bin, never through PATH.
    assert shutil.which("git", path=str(git.dir.parent)) is None
