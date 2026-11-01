# SPDX-License-Identifier: Apache-2.0
"""Moving a project folder to the trash (09 18.15, FEAT-delete-spaces-chats D-DEL-10 to 13).

Every refusal code is checked against a real Git repository made in the test's own folder (a
bare repository stands in for the remote); every move goes to a trash under the test's own home
or data folder, never to the trash of the machine that runs the tests.
"""

from __future__ import annotations

import asyncio
import json
import os
import shutil
import subprocess
import sys
from collections.abc import Callable, Mapping
from datetime import UTC, datetime
from pathlib import Path
from uuid import UUID, uuid4

import pytest
from bothy_sdk.generated.models import ProjectReadiness, ProjectTrashRequest

from claude_code_runner.environment import kept_environment
from claude_code_runner.trash import (
    FolderTrash,
    GitRunner,
    TrashPlaces,
    check_folder,
    move_to_trash,
)

pytestmark = pytest.mark.anyio

NOW = datetime(2026, 11, 1, 12, 30, 5, tzinfo=UTC)
GIT = shutil.which("git")
needs_git = pytest.mark.skipif(GIT is None, reason="git is not installed")


# ---- the world of a test ------------------------------------------------------------------


class Host:
    """A projects root, a state folder, a home and a place outside the root."""

    def __init__(self, base: Path) -> None:
        self.base = base
        self.root = base / "projects"
        self.state = base / "state"
        self.home = base / "home"
        self.outside = base / "outside"
        for path in (self.root, self.state, self.home, self.outside):
            path.mkdir()
        # Something that must stay untouched by every check and move.
        (self.outside / "sentinel.txt").write_text("keep")
        self.environ = {
            "HOME": str(self.home),
            "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
            "LANG": "C.UTF-8",
        }

    def git_env(self) -> dict[str, str]:
        return {
            **kept_environment(self.environ),
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_AUTHOR_NAME": "Alex",
            "GIT_AUTHOR_EMAIL": "alex@example.com",
            "GIT_COMMITTER_NAME": "Alex",
            "GIT_COMMITTER_EMAIL": "alex@example.com",
        }

    def git(self, *argv: str, cwd: Path) -> str:
        done = subprocess.run(
            [str(GIT), *argv],
            cwd=cwd,
            env=self.git_env(),
            check=True,
            capture_output=True,
            text=True,
        )
        return done.stdout

    def repo(self, name: str, *, pushed: bool = True) -> Path:
        """A Git work tree under the root with one commit, pushed to a bare remote."""
        path = self.root / name
        path.mkdir(parents=True)
        self.git("init", "--quiet", "--initial-branch", "main", cwd=path)
        (path / "README.md").write_text("hello\n")
        self.git("add", "README.md", cwd=path)
        self.git("commit", "--quiet", "-m", "first", cwd=path)
        if pushed:
            remote = self.outside / f"{name.replace('/', '-')}.git"
            self.git("init", "--quiet", "--bare", str(remote), cwd=self.outside)
            self.git("remote", "add", "origin", str(remote), cwd=path)
            self.git("push", "--quiet", "origin", "main", cwd=path)
        return path

    def runner(self, git_bin: str | None = GIT, timeout: float = 30.0) -> GitRunner:
        return GitRunner(git_bin=git_bin, environ=self.environ, timeout=timeout)

    async def check(self, ref: str, *, others: tuple[str, ...] = (), **git: object) -> str | None:
        return await check_folder(
            self.root,
            ref,
            others=others,
            state_dir=self.state,
            git=self.runner(**git),  # type: ignore[arg-type]
        )

    def places(self, platform: str = "darwin", data_home: Path | None = None) -> TrashPlaces:
        return TrashPlaces(platform=platform, home=self.home, data_home=data_home)

    def untouched_outside(self) -> bool:
        return (self.outside / "sentinel.txt").read_text() == "keep"


@pytest.fixture
def host(tmp_path: Path) -> Host:
    return Host(tmp_path)


# ---- the check: every refusal code ----------------------------------------------------------


@needs_git
async def test_a_clean_pushed_repository_may_go(host: Host) -> None:
    host.repo("website")
    assert await host.check("website") is None


async def test_a_folder_without_git_has_no_git_rule(host: Host) -> None:
    (host.root / "notes").mkdir()
    (host.root / "notes" / "todo.txt").write_text("x")
    assert await host.check("notes") is None
    # Without Git on the host as well: nothing to ask Git about.
    assert await host.check("notes", git_bin=None) is None


async def test_outside_root(host: Host) -> None:
    (host.root / "elsewhere").symlink_to(host.outside)
    for ref in ("../outside", "/etc", "", ".", "elsewhere", "~/x", "a\\b"):
        assert await host.check(ref) == "outside_root", ref
    assert host.untouched_outside()


async def test_a_bound_path_that_is_a_link_inside_the_root(host: Host) -> None:
    (host.root / "website").mkdir()
    (host.root / "alias").symlink_to(host.root / "website")
    assert await host.check("alias") == "symlink"
    # A link as a component below the root counts too.
    (host.root / "real-group" / "site").mkdir(parents=True)
    (host.root / "group").symlink_to(host.root / "real-group")
    assert await host.check("group/site") == "symlink"
    assert await host.check("real-group/site") is None


async def test_workspace_missing(host: Host) -> None:
    assert await host.check("website") == "workspace_missing"
    (host.root / "file.txt").write_text("x")
    assert await host.check("file.txt") == "workspace_missing"


async def test_contains_another_bound_project_or_the_state_folder(host: Host) -> None:
    (host.root / "group" / "website").mkdir(parents=True)
    assert await host.check("group", others=("group/website",)) == "contains_other_project"
    # Two forums bound to the same folder: moving it would take the other's folder.
    assert await host.check("group", others=("group",)) == "contains_other_project"
    assert await host.check("group", others=("other",)) is None
    # The runner's own state directory inside the folder.
    state_inside = host.root / "group" / "state"
    state_inside.mkdir()
    refused = await check_folder(
        host.root, "group", others=(), state_dir=state_inside, git=host.runner()
    )
    assert refused == "contains_other_project"


def _case_insensitive(path: Path) -> bool:
    probe = path / "CaseProbe"
    probe.mkdir()
    try:
        return (path / "caseprobe").exists()
    finally:
        probe.rmdir()


async def test_another_project_in_another_spelling_is_still_inside(host: Host) -> None:
    # On a file system without case (APFS, NTFS) 'Group' and 'group/website' are nested; the
    # check compares folder identity, not spelling (D-DEL-RUN-12).
    if not _case_insensitive(host.root):
        pytest.skip("needs a file system without case")
    (host.root / "Group" / "website").mkdir(parents=True)
    assert await host.check("Group", others=("group/website",)) == "contains_other_project"
    assert await host.check("group", others=("GROUP/website",)) == "contains_other_project"
    assert await host.check("group", others=("Group",)) == "contains_other_project"


async def test_another_project_in_another_unicode_form_is_still_inside(host: Host) -> None:
    nfc, nfd = "caf\u00e9", "cafe\u0301"
    (host.root / nfc / "site").mkdir(parents=True)
    if not (host.root / nfd).exists():
        pytest.skip("needs a file system that folds Unicode forms")
    assert await host.check(nfd, others=(f"{nfc}/site",)) == "contains_other_project"


async def test_the_trash_under_the_root_is_never_a_project(host: Host) -> None:
    (host.root / ".bothy-trash" / "old").mkdir(parents=True)
    assert await host.check(".bothy-trash") == "outside_root"
    assert await host.check(".bothy-trash/old") == "outside_root"
    assert await host.check("./.bothy-trash") == "outside_root"
    assert await host.check("./.BOTHY-TRASH/old") == "outside_root"


@needs_git
async def test_foreign_worktree_for_a_linked_work_tree(host: Host) -> None:
    main = host.repo("website")
    host.git("worktree", "add", "--quiet", str(host.root / "linked"), "-b", "side", cwd=main)
    assert await host.check("linked") == "foreign_worktree"
    # A plain folder inside another work tree is part of that repository.
    (main / "docs").mkdir()
    (main / "docs" / "a.md").write_text("x")
    host.git("add", "docs", cwd=main)
    host.git("commit", "--quiet", "-m", "docs", cwd=main)
    host.git("push", "--quiet", "origin", "main", cwd=main)
    assert await host.check("website/docs") == "foreign_worktree"


@needs_git
async def test_uncommitted_changes(host: Host) -> None:
    path = host.repo("website")
    (path / "README.md").write_text("changed\n")
    assert await host.check("website") == "uncommitted_changes"
    host.git("checkout", "--quiet", "--", "README.md", cwd=path)
    assert await host.check("website") is None
    (path / "new.txt").write_text("untracked")
    assert await host.check("website") == "uncommitted_changes"


@needs_git
async def test_a_stash_counts_as_uncommitted(host: Host) -> None:
    path = host.repo("website")
    (path / "README.md").write_text("changed\n")
    host.git("stash", "--quiet", cwd=path)
    assert (path / "README.md").read_text() == "hello\n"
    assert await host.check("website") == "uncommitted_changes"


@needs_git
async def test_unpushed_commits(host: Host) -> None:
    path = host.repo("website")
    (path / "b.txt").write_text("b")
    host.git("add", "b.txt", cwd=path)
    host.git("commit", "--quiet", "-m", "second", cwd=path)
    assert await host.check("website") == "unpushed_commits"
    host.git("push", "--quiet", "origin", "main", cwd=path)
    assert await host.check("website") is None
    # A local branch no remote branch contains.
    host.git("checkout", "--quiet", "-b", "topic", cwd=path)
    (path / "c.txt").write_text("c")
    host.git("add", "c.txt", cwd=path)
    host.git("commit", "--quiet", "-m", "topic", cwd=path)
    host.git("checkout", "--quiet", "main", cwd=path)
    assert await host.check("website") == "unpushed_commits"


@needs_git
async def test_a_repository_without_a_remote_has_unpushed_commits(host: Host) -> None:
    host.repo("website", pushed=False)
    assert await host.check("website") == "unpushed_commits"


@needs_git
async def test_git_errors_and_time_outs_are_check_failed(host: Host, tmp_path: Path) -> None:
    host.repo("website")
    failing = tmp_path / "git-failing"
    failing.write_text("#!/bin/sh\nexit 2\n")
    failing.chmod(0o755)
    assert await host.check("website", git_bin=str(failing)) == "check_failed"
    hanging = tmp_path / "git-hanging"
    hanging.write_text(f"#!{sys.executable}\nimport time\ntime.sleep(60)\n")
    hanging.chmod(0o755)
    assert await host.check("website", git_bin=str(hanging), timeout=0.3) == "check_failed"
    # A Git work tree on a host without git cannot be checked.
    assert await host.check("website", git_bin=None) == "check_failed"


@needs_git
async def test_git_runs_with_an_argument_list_and_the_allow_list(
    host: Host, tmp_path: Path
) -> None:
    host.repo("website")
    calls = tmp_path / "calls.jsonl"
    recorder = tmp_path / "git-recorder"
    recorder.write_text(
        f"#!{sys.executable}\n"
        "import json, os, sys\n"
        f"with open({str(calls)!r}, 'a') as fh:\n"
        "    fh.write(json.dumps({'argv': sys.argv[1:], 'env': dict(os.environ)}) + '\\n')\n"
        f"os.execv({str(GIT)!r}, [{str(GIT)!r}, *sys.argv[1:]])\n"
    )
    recorder.chmod(0o755)
    host.environ["ANTHROPIC_API_KEY"] = "sk-ant-never-0123"
    assert await host.check("website", git_bin=str(recorder)) is None
    seen = [json.loads(line) for line in calls.read_text().splitlines()]
    assert seen
    for call in seen:
        assert "ANTHROPIC_API_KEY" not in call["env"]
        assert call["env"]["GIT_TERMINAL_PROMPT"] == "0"
        assert call["argv"][:2] == ["-c", "core.fsmonitor=false"]


# ---- the move ---------------------------------------------------------------------------------


def test_macos_moves_into_the_home_trash(host: Host) -> None:
    folder = host.root / "website"
    folder.mkdir()
    (folder / "a.txt").write_text("a")
    outcome = move_to_trash(folder, host.root, host.places("darwin"), now=NOW)
    assert outcome.code is None
    assert outcome.place == "trash"
    assert not folder.exists()
    assert (host.home / ".Trash" / "website" / "a.txt").read_text() == "a"


def test_a_taken_name_gets_a_suffix(host: Host) -> None:
    (host.home / ".Trash" / "website").mkdir(parents=True)
    (host.home / ".Trash" / "website" / "old.txt").write_text("old")
    (host.home / ".Trash" / "website-2").mkdir()
    folder = host.root / "website"
    folder.mkdir()
    assert move_to_trash(folder, host.root, host.places("darwin"), now=NOW).code is None
    assert (host.home / ".Trash" / "website" / "old.txt").read_text() == "old"
    assert (host.home / ".Trash" / "website-3").is_dir()


def test_linux_uses_the_freedesktop_trash_with_trashinfo(host: Host) -> None:
    data = host.base / "data"
    folder = host.root / "my site"
    folder.mkdir()
    outcome = move_to_trash(folder, host.root, host.places("linux", data_home=data), now=NOW)
    assert outcome.code is None
    assert not folder.exists()
    assert (data / "Trash" / "files" / "my site").is_dir()
    info = (data / "Trash" / "info" / "my site.trashinfo").read_text()
    lines = info.splitlines()
    assert lines[0] == "[Trash Info]"
    assert lines[1] == "Path=" + str(folder.resolve()).replace(" ", "%20")
    assert lines[2].startswith("DeletionDate=")
    assert len(lines[2]) == len("DeletionDate=2026-11-01T12:30:05")


def test_linux_without_a_data_home_uses_the_local_share(host: Host) -> None:
    folder = host.root / "website"
    folder.mkdir()
    (host.home / ".local" / "share" / "Trash" / "info").mkdir(parents=True)
    (host.home / ".local" / "share" / "Trash" / "info" / "website.trashinfo").write_text("x")
    assert move_to_trash(folder, host.root, host.places("freebsd14"), now=NOW).code is None
    # The taken info name gives the suffix for both files.
    trash = host.home / ".local" / "share" / "Trash"
    assert (trash / "files" / "website-2").is_dir()
    assert (trash / "info" / "website-2.trashinfo").exists()


def test_another_file_system_uses_the_trash_under_the_root(host: Host) -> None:
    folder = host.root / "website"
    folder.mkdir()
    trash = host.home / ".Trash"

    def device(path: Path) -> int:
        return 2 if Path(path) == trash else 1

    outcome = move_to_trash(folder, host.root, host.places("darwin"), now=NOW, device=device)
    assert outcome.code is None
    assert outcome.place == "fallback"
    assert not folder.exists()
    assert (host.root / ".bothy-trash" / "website-20261101T123005Z").is_dir()
    assert not (trash / "website").exists()
    # A second move at the same second gets a suffix.
    folder.mkdir()
    move_to_trash(folder, host.root, host.places("darwin"), now=NOW, device=device)
    assert (host.root / ".bothy-trash" / "website-20261101T123005Z-2").is_dir()


def test_a_trash_info_folder_the_runner_may_not_write_uses_the_fallback(host: Host) -> None:
    data = host.base / "data"
    (data / "Trash" / "files").mkdir(parents=True)
    (data / "Trash" / "info").mkdir()
    (data / "Trash" / "info").chmod(0o500)
    folder = host.root / "website"
    folder.mkdir()
    try:
        outcome = move_to_trash(folder, host.root, host.places("linux", data_home=data), now=NOW)
    finally:
        (data / "Trash" / "info").chmod(0o700)
    assert outcome.code is None
    assert outcome.place == "fallback"
    assert (host.root / ".bothy-trash" / "website-20261101T123005Z").is_dir()


def test_the_move_never_follows_links_out_of_the_folder(host: Host) -> None:
    folder = host.root / "website"
    folder.mkdir()
    (folder / "out").symlink_to(host.outside)
    (host.root / "neighbour").mkdir()
    assert move_to_trash(folder, host.root, host.places("darwin"), now=NOW).code is None
    assert host.untouched_outside()
    assert (host.root / "neighbour").is_dir()
    moved = host.home / ".Trash" / "website" / "out"
    assert moved.is_symlink()


def test_windows_moves_into_the_trash_under_the_root(host: Host) -> None:
    # The recycle bin of the shell deletes for good when the bin is off, full or the path too
    # long, without telling a process that runs without a window; so Windows always renames
    # into .bothy-trash (D-DEL-RUN-11).
    folder = host.root / "website"
    folder.mkdir()
    outcome = move_to_trash(folder, host.root, host.places("win32"), now=NOW)
    assert outcome.code is None
    assert outcome.place == "fallback"
    assert not folder.exists()
    assert (host.root / ".bothy-trash" / "website-20261101T123005Z").is_dir()


def test_a_fallback_trash_that_is_a_link_is_refused(host: Host) -> None:
    (host.root / ".bothy-trash").symlink_to(host.outside)
    folder = host.root / "website"
    folder.mkdir()
    outcome = move_to_trash(folder, host.root, host.places("win32"), now=NOW)
    assert outcome.code == "move_failed"
    assert folder.is_dir()
    assert sorted(p.name for p in host.outside.iterdir()) == ["sentinel.txt"]
    assert host.untouched_outside()


# ---- the requests ----------------------------------------------------------------------------


class Stops:
    """The runner's stop path, recording whether the folder still existed when it was asked."""

    def __init__(self, folder: Path, done: bool = True) -> None:
        self.folder = folder
        self.done = done
        self.calls: list[tuple[UUID, bool]] = []

    async def __call__(self, channel_id: UUID) -> bool:
        self.calls.append((channel_id, self.folder.exists()))
        return self.done


def make_trash(
    host: Host,
    *,
    bound: Mapping[UUID, str] | None = None,
    on_done: Callable[[], object] | None = None,
) -> FolderTrash:
    async def done() -> None:
        if on_done is not None:
            on_done()

    return FolderTrash(
        projects_root=host.root,
        state_dir=host.state,
        git=host.runner(),
        places=host.places("darwin"),
        interval=60.0,
        bound=lambda: dict(bound or {}),
        now=lambda: NOW,
        on_done=done,
    )


def request(channel: UUID, ref: str) -> ProjectTrashRequest:
    return ProjectTrashRequest(channel_id=channel, workspace_ref=ref)


async def test_a_request_stops_the_turn_checks_and_moves(host: Host) -> None:
    channel = uuid4()
    folder = host.root / "notes"
    folder.mkdir()
    stops = Stops(folder)
    done: list[bool] = []
    trash = make_trash(host, on_done=lambda: done.append(True))
    trash.requested([request(channel, "notes")], stop=stops)
    await trash.wait_idle()
    # The running turn was stopped first, while the folder was still there.
    assert stops.calls == [(channel, True)]
    assert not folder.exists()
    assert (host.home / ".Trash" / "notes").is_dir()
    assert [r.to_dict() for r in trash.results()] == [
        {"channel_id": str(channel), "result": "trashed"}
    ]
    assert done == [True]


@needs_git
async def test_a_refused_folder_is_reported_and_stays(host: Host) -> None:
    channel = uuid4()
    path = host.repo("website")
    (path / "README.md").write_text("changed\n")
    trash = make_trash(host)
    trash.requested([request(channel, "website")], stop=Stops(path))
    await trash.wait_idle()
    assert path.is_dir()
    assert (path / "README.md").read_text() == "changed\n"
    assert not (host.home / ".Trash").exists()
    assert [r.to_dict() for r in trash.results()] == [
        {"channel_id": str(channel), "result": "refused", "reason": "uncommitted_changes"}
    ]


async def test_another_bound_forum_inside_the_folder_refuses(host: Host) -> None:
    channel, other = uuid4(), uuid4()
    (host.root / "group" / "website").mkdir(parents=True)
    trash = make_trash(host, bound={channel: "group", other: "group/website"})
    trash.requested([request(channel, "group")], stop=Stops(host.root / "group"))
    await trash.wait_idle()
    assert (host.root / "group" / "website").is_dir()
    assert trash.results()[0].reason == "contains_other_project"


async def test_a_turn_that_does_not_end_defers_the_request(host: Host) -> None:
    channel = uuid4()
    folder = host.root / "notes"
    folder.mkdir()
    trash = make_trash(host)
    trash.requested([request(channel, "notes")], stop=Stops(folder, done=False))
    await trash.wait_idle()
    assert folder.is_dir()
    assert trash.results() == []
    # The server asks again with the next answer; then the turn is gone.
    trash.requested([request(channel, "notes")], stop=Stops(folder))
    await trash.wait_idle()
    assert not folder.exists()


async def test_a_restart_neither_repeats_a_move_nor_loses_a_result(host: Host) -> None:
    channel = uuid4()
    folder = host.root / "notes"
    folder.mkdir()
    first = make_trash(host)
    first.requested([request(channel, "notes")], stop=Stops(folder))
    await first.wait_idle()
    assert not folder.exists()
    # A new folder of the same name appears; the runner restarts before it reported.
    folder.mkdir()
    (folder / "new.txt").write_text("new")
    second = make_trash(host)
    assert [r.to_dict() for r in second.results()] == [
        {"channel_id": str(channel), "result": "trashed"}
    ]
    stops = Stops(folder)
    second.requested([request(channel, "notes")], stop=stops)
    await second.wait_idle()
    assert (folder / "new.txt").read_text() == "new"
    assert stops.calls == []
    # Once a heartbeat carried the result, it is not sent again, also after a restart.
    second.reported([channel])
    assert second.results() == []
    assert make_trash(host).results() == []


async def test_a_result_the_server_still_asks_for_is_sent_again(host: Host) -> None:
    channel = uuid4()
    (host.root / "notes").mkdir()
    trash = make_trash(host)
    trash.requested([request(channel, "notes")], stop=Stops(host.root / "notes"))
    await trash.wait_idle()
    trash.reported([channel])
    assert trash.results() == []
    trash.requested([request(channel, "notes")], stop=Stops(host.root / "notes"))
    await trash.wait_idle()
    assert [r.result.value for r in trash.results()] == ["trashed"]


async def test_a_move_cut_off_by_a_crash_is_finished_from_the_record(host: Host) -> None:
    channel = uuid4()
    target = host.home / ".Trash" / "notes"
    target.mkdir(parents=True)
    record = {str(channel): {"workspace_ref": "notes", "state": "moving", "target": str(target)}}
    (host.state / "trash.json").write_text(json.dumps(record))
    trash = make_trash(host)
    stops = Stops(host.root / "notes")
    trash.requested([request(channel, "notes")], stop=stops)
    await trash.wait_idle()
    assert [r.to_dict() for r in trash.results()] == [
        {"channel_id": str(channel), "result": "trashed"}
    ]
    assert stops.calls == []


async def test_a_move_cut_off_before_the_rename_runs_again(host: Host) -> None:
    channel = uuid4()
    folder = host.root / "notes"
    folder.mkdir()
    record = {
        str(channel): {
            "workspace_ref": "notes",
            "state": "moving",
            "target": str(host.home / ".Trash" / "notes"),
        }
    }
    (host.state / "trash.json").write_text(json.dumps(record))
    trash = make_trash(host)
    trash.requested([request(channel, "notes")], stop=Stops(folder))
    await trash.wait_idle()
    assert not folder.exists()
    assert (host.home / ".Trash" / "notes").is_dir()


async def test_the_folder_check_is_computed_at_most_every_interval(host: Host) -> None:
    channel = uuid4()
    (host.root / "notes").mkdir()
    now = [0.0]
    trash = FolderTrash(
        projects_root=host.root,
        state_dir=host.state,
        git=host.runner(),
        places=host.places("darwin"),
        interval=60.0,
        bound=lambda: {channel: "notes"},
        clock=lambda: now[0],
    )
    entry = ProjectReadiness(channel_id=channel, ready=True)
    trash.annotate([entry])
    assert entry.to_dict() == {"channel_id": str(channel), "ready": True}
    assert await trash.refresh() is True
    trash.annotate([entry])
    assert entry.to_dict()["trash_check"] == {"ok": True}
    (host.root / "notes").rename(host.root / "gone")
    now[0] = 59.0
    assert await trash.refresh() is False
    now[0] = 60.0
    assert await trash.refresh() is True
    entry = ProjectReadiness(channel_id=channel, ready=False)
    trash.annotate([entry])
    assert entry.to_dict()["trash_check"] == {"ok": False, "reason": "workspace_missing"}


async def test_a_changed_binding_never_shows_the_old_check(host: Host) -> None:
    channel = uuid4()
    (host.root / "notes").mkdir()
    bound = {channel: "notes"}
    trash = FolderTrash(
        projects_root=host.root,
        state_dir=host.state,
        git=host.runner(),
        places=host.places("darwin"),
        interval=60.0,
        bound=lambda: dict(bound),
    )
    await trash.refresh()
    bound[channel] = "other"
    entry = ProjectReadiness(channel_id=channel, ready=False)
    trash.annotate([entry])
    assert "trash_check" not in entry.to_dict()


async def test_logs_name_codes_never_paths(host: Host, caplog: pytest.LogCaptureFixture) -> None:
    caplog.set_level("DEBUG")
    channel = uuid4()
    (host.root / "secret-project").mkdir()
    trash = make_trash(host)
    trash.requested([request(channel, "secret-project")], stop=Stops(host.root / "x"))
    trash.requested([request(uuid4(), "../outside")], stop=Stops(host.root / "x"))
    await trash.wait_idle()
    assert "project_trash_moved" in caplog.text
    assert "project_trash_refused" in caplog.text
    assert "secret-project" not in caplog.text
    assert str(host.base) not in caplog.text


async def test_close_stops_the_background_work(host: Host) -> None:
    trash = make_trash(host)
    gate = asyncio.Event()

    async def stop(_channel: UUID) -> bool:
        await gate.wait()
        return True

    (host.root / "notes").mkdir()
    trash.requested([request(uuid4(), "notes")], stop=stop)
    await asyncio.sleep(0)
    await trash.close()
    assert (host.root / "notes").is_dir()
