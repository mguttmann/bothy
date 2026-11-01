# SPDX-License-Identifier: Apache-2.0
"""Moving the project folder of a deleted forum to the trash of the host (09 18.15).

The runner checks every bound folder it serves at most every ``runner.trash_check_interval``
and reports the outcome in the readiness entry (``trash_check``). When a forum's deletion is
final the server hands ``HeartbeatAck.project_trash``; the runner stops a turn still running in
that forum, checks the folder again and moves it, and reports ``trashed`` or ``refused`` with a
code in ``project_trash_results`` of a later heartbeat (D-DEL-10, D-DEL-11).

A move is always one ``os.rename`` on one file system (macOS ``~/.Trash``, the freedesktop
trash elsewhere, ``.bothy-trash`` under the projects root when the trash is on another file
system or cannot be used, and always on Windows); nothing is ever copied or removed (D-DEL-12,
D-DEL-RUN-11). ``git`` runs from an argument list with the allow list environment and a
time limit, as in the folder creation of 18.12. The handled requests and their results are kept
in ``<runner.state_dir>/trash.json``, so a restart neither repeats a move nor loses a result.
Log lines carry codes and channel IDs, never a path or a name.

Examples:
    >>> CHECK_REASONS[0], MOVE_REASONS
    ('outside_root', ('move_failed', 'trash_unavailable'))
"""

from __future__ import annotations

import asyncio
import contextlib
import errno
import json
import logging
import os
import signal
import stat
import sys
import time
from collections.abc import Awaitable, Callable, Iterable, Mapping, Sequence
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from urllib.parse import quote
from uuid import UUID

from bothy_sdk.generated.models import (
    ProjectReadiness,
    ProjectTrashCheckReport,
    ProjectTrashRequest,
    ProjectTrashResult,
    ProjectTrashResultResult,
)

from claude_code_runner.environment import kept_environment
from claude_code_runner.statedir import write_private
from claude_code_runner.workspace import WorkspaceError, resolve

__all__ = [
    "CHECK_REASONS",
    "MOVE_REASONS",
    "FolderTrash",
    "GitRunner",
    "Moved",
    "TrashPlaces",
    "check_folder",
    "move_to_trash",
]

CHECK_REASONS = (
    "outside_root",
    "symlink",
    "workspace_missing",
    "contains_other_project",
    "foreign_worktree",
    "uncommitted_changes",
    "unpushed_commits",
    "check_failed",
)
"""The refusal codes of the folder check, in the order the rules are applied (D-DEL-13)."""

MOVE_REASONS = ("move_failed", "trash_unavailable")
"""The refusal codes of the move itself: a failed rename; ``trash_unavailable`` is kept for
older records and no longer sent (D-DEL-RUN-11)."""

FALLBACK_DIR = ".bothy-trash"
"""The trash under the projects root for a host trash on another file system (D-DEL-12)."""

_RECORD = "trash.json"
_UNUSABLE = frozenset({errno.EXDEV, errno.EACCES, errno.EPERM})
"""Rename errors that make the host trash unusable: another file system or no right to it."""
_SUFFIX_MAX = 10_000
_LOG = logging.getLogger("claude_code_runner")


def _log(event: str, *, warning: bool = False, **ids: object) -> None:
    line = json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}})
    if warning:
        _LOG.warning(line)
    else:
        _LOG.info(line)


# ---- the check ------------------------------------------------------------------------------


class _GitError(Exception):
    """``git`` did not start, did not end in time or ended with an error."""


@dataclass(frozen=True)
class GitRunner:
    """How the check runs ``git``.

    Attributes:
        git_bin: The ``git`` found from ``runner.git_bin`` in the cleaned ``PATH``, or ``None``.
        environ: The runner's own environment (the allow list is taken from it).
        timeout: The time limit of one ``git`` call (``runner.cli_probe_timeout``).
    """

    git_bin: str | None
    environ: Mapping[str, str]
    timeout: float

    def env(self) -> dict[str, str]:
        """The allow list of 18.2 plus no prompt and messages in the C locale."""
        return {
            **kept_environment(self.environ),
            "GIT_TERMINAL_PROMPT": "0",
            "GIT_OPTIONAL_LOCKS": "0",
            "LC_ALL": "C",
        }

    async def run(self, argv: Sequence[str], cwd: Path) -> tuple[int, bytes]:
        """Run ``git -c core.fsmonitor=false <argv>`` in ``cwd``.

        Args:
            argv: The arguments after the global options.
            cwd: The working folder.

        Returns:
            The exit status and the standard output.

        Raises:
            _GitError: ``git`` is missing, did not start or did not end in time.
        """
        if self.git_bin is None:
            raise _GitError
        try:
            proc = await asyncio.create_subprocess_exec(
                self.git_bin,
                "-c",
                "core.fsmonitor=false",
                *argv,
                cwd=cwd,
                env=self.env(),
                stdin=asyncio.subprocess.DEVNULL,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.DEVNULL,
                start_new_session=True,
            )
        except OSError:
            raise _GitError from None
        try:
            out, _ = await asyncio.wait_for(proc.communicate(), self.timeout)
        except TimeoutError:
            _kill_group(proc.pid)
            await proc.wait()
            raise _GitError from None
        except asyncio.CancelledError:
            _kill_group(proc.pid)
            await proc.wait()
            raise
        return int(proc.returncode or 0), out


async def check_folder(  # noqa: PLR0911 - one return per rule
    projects_root: Path,
    workspace_ref: str,
    *,
    others: Iterable[str],
    state_dir: Path,
    git: GitRunner,
) -> str | None:
    """Check whether the bound folder may move to the trash now (D-DEL-13).

    Args:
        projects_root: ``runner.projects_root``.
        workspace_ref: The binding's ``workspace_ref``.
        others: The ``workspace_ref`` of every other bound forum the runner serves.
        state_dir: ``runner.state_dir``.
        git: How ``git`` runs.

    Returns:
        ``None`` when the folder may go, else a code of `CHECK_REASONS`.
    """
    first = next((p for p in workspace_ref.split("/") if p not in ("", ".")), "")
    if first.casefold() == FALLBACK_DIR:
        return "outside_root"  # the runner's own trash is never a project (D-DEL-RUN-12)
    try:
        path = resolve(projects_root, workspace_ref)
    except WorkspaceError:
        return "outside_root"
    code = _link_or_missing(projects_root, workspace_ref)
    if code is not None:
        return code
    if not path.is_dir():
        return "workspace_missing"
    if _holds_another(path, others, projects_root, state_dir):
        return "contains_other_project"
    try:
        return await _git_rules(path, git)
    except _GitError:
        return "check_failed"


def _link_or_missing(projects_root: Path, workspace_ref: str) -> str | None:
    """``symlink`` when the bound path or a component below the root is a symbolic link."""
    current = Path(projects_root)
    for part in workspace_ref.split("/"):
        if part in ("", "."):
            continue
        current /= part
        try:
            mode = os.lstat(current).st_mode
        except FileNotFoundError:
            return "workspace_missing"
        except OSError:
            return "check_failed"
        if stat.S_ISLNK(mode):
            return "symlink"
    return None


def _inside(path: Path, folder: Path) -> bool:
    """Whether ``path`` is ``folder`` or below it, by folder identity, not by spelling.

    A file system without case or Unicode forms (APFS, NTFS) names one folder in several
    spellings, so the real paths are compared by spelling first and then every existing
    ancestor of ``path`` by device and inode against ``folder`` (D-DEL-RUN-12).
    """
    a, b = os.path.realpath(path), os.path.realpath(folder)
    if a == b or os.path.commonpath([b, a]) == b:
        return True
    try:
        target = Path(b).stat()
    except OSError:
        return False
    current = Path(a)
    while True:
        try:
            st = current.stat()
        except OSError:
            st = None
        if st is not None and (st.st_dev, st.st_ino) == (target.st_dev, target.st_ino):
            return True
        parent = current.parent
        if parent == current:
            return False
        current = parent


def _holds_another(path: Path, others: Iterable[str], root: Path, state_dir: Path) -> bool:
    """Whether the folder is or holds another bound folder or the runner's state directory."""
    if _inside(Path(state_dir), path):
        return True
    for ref in others:
        try:
            other = resolve(root, ref)
        except WorkspaceError:
            continue
        if _inside(other, path):
            return True
    return False


async def _git_rules(path: Path, git: GitRunner) -> str | None:  # noqa: PLR0911 - one per rule
    """The Git rules: own repository, nothing uncommitted, stashed or unpushed."""
    try:
        status, out = await git.run(["rev-parse", "--is-inside-work-tree"], path)
    except _GitError:
        if git.git_bin is None and not _git_above(path):
            return None  # no Git on the host and no Git here: nothing to ask
        raise
    if status != 0:
        if _git_above(path):
            raise _GitError
        return None  # not a Git work tree: no Git rule
    if out.strip() != b"true":
        raise _GitError
    common = await _output(git, ["rev-parse", "--path-format=absolute", "--git-common-dir"], path)
    common_dir = os.path.realpath(common.decode("utf-8", errors="surrogateescape").strip())
    if common_dir == str(path) or not _inside(Path(common_dir), path):
        return "foreign_worktree"
    if await _output(git, ["status", "--porcelain", "--untracked-files=normal"], path):
        return "uncommitted_changes"
    if await _output(git, ["stash", "list"], path):
        return "uncommitted_changes"
    if await _output(git, ["log", "--branches", "--not", "--remotes", "-1", "--format=%H"], path):
        return "unpushed_commits"
    return None


async def _output(git: GitRunner, argv: Sequence[str], cwd: Path) -> bytes:
    status, out = await git.run(argv, cwd)
    if status != 0:
        raise _GitError
    return out.strip()


def _git_above(path: Path) -> bool:
    """Whether ``.git`` exists in the folder or one of its parents (a work tree Git refused)."""
    return any(os.path.lexists(p / ".git") for p in (path, *path.parents))


# ---- the move -------------------------------------------------------------------------------


@dataclass(frozen=True)
class TrashPlaces:
    """Where the trash of the host is.

    Attributes:
        platform: ``sys.platform`` of the host.
        home: The home folder of the runner user.
        data_home: ``XDG_DATA_HOME`` when it is an absolute path, else ``None``.
    """

    platform: str
    home: Path
    data_home: Path | None = None

    @classmethod
    def of(cls, environ: Mapping[str, str], platform: str = sys.platform) -> TrashPlaces:
        """The places of the runner's own environment.

        Args:
            environ: The runner's environment (``HOME``, ``XDG_DATA_HOME``).
            platform: ``sys.platform``.

        Returns:
            The places; ``HOME`` that is not absolute falls back to `Path.home`.
        """
        home = Path(environ.get("HOME", ""))
        data = Path(environ.get("XDG_DATA_HOME", ""))
        return cls(
            platform=platform,
            home=home if home.is_absolute() else Path.home(),
            data_home=data if data.is_absolute() else None,
        )


@dataclass(frozen=True)
class Moved:
    """The outcome of `move_to_trash`.

    Attributes:
        code: ``None`` when the folder moved, else a code of `MOVE_REASONS`.
        place: ``trash`` or ``fallback`` (``.bothy-trash``); empty on failure.
    """

    code: str | None
    place: str = ""


def _device(path: Path) -> int:
    return path.stat().st_dev


def move_to_trash(  # noqa: PLR0913 - the parts of one move are explicit
    folder: Path,
    projects_root: Path,
    places: TrashPlaces,
    *,
    now: datetime,
    device: Callable[[Path], int] = _device,
    on_target: Callable[[Path | None], None] | None = None,
) -> Moved:
    """Move ``folder`` into the trash of the host, never removing anything (D-DEL-12).

    Args:
        folder: The checked real path of the bound folder.
        projects_root: ``runner.projects_root``.
        places: Where the host's trash is.
        now: The time of the move (``DeletionDate``, the fallback name).
        device: ``st_dev`` of a path (tests simulate another file system).
        on_target: Called with the target path right before the rename, so a crash in
            between can be finished after a restart.

    Returns:
        The outcome.
    """
    if places.platform == "win32":
        # The shell's recycle bin deletes for good when the bin is off, full or the path too
        # long, without telling a process that has no window; Windows always renames into the
        # trash under the root (D-DEL-RUN-11).
        return _fallback(folder, projects_root, now, on_target)
    try:
        target, info = _host_trash(folder, places, now, device)
    except _HostTrashUnusableError:
        return _fallback(folder, projects_root, now, on_target)
    if on_target is not None:
        on_target(target)
    try:
        folder.rename(target)
    except OSError as err:
        if info is not None:
            _forget(info)
        if err.errno in _UNUSABLE:
            return _fallback(folder, projects_root, now, on_target)
        return Moved("move_failed")
    return Moved(None, "trash")


class _HostTrashUnusableError(Exception):
    """The host trash is on another file system than the folder, or cannot be used."""


def _host_trash(
    folder: Path, places: TrashPlaces, now: datetime, device: Callable[[Path], int]
) -> tuple[Path, Path | None]:
    """The target in the host trash and, for the freedesktop trash, its reserved info file."""
    freedesktop = places.platform != "darwin"
    if freedesktop:
        base = (places.data_home or places.home / ".local" / "share") / "Trash"
        files, infos = base / "files", base / "info"
    else:
        files, infos = places.home / ".Trash", None
    try:
        for place in (files, infos):
            if place is not None:
                place.mkdir(mode=0o700, parents=True, exist_ok=True)
        if device(files) != device(folder):
            raise _HostTrashUnusableError
    except OSError:
        raise _HostTrashUnusableError from None
    for name in _names(folder.name):
        target = files / name
        if os.path.lexists(target):
            continue
        if infos is None:
            return target, None
        info = infos / f"{name}.trashinfo"
        if _reserve(info, folder, now):
            return target, info
    raise _HostTrashUnusableError


def _names(name: str) -> Iterable[str]:
    yield name
    for n in range(2, _SUFFIX_MAX):
        yield f"{name}-{n}"


def _reserve(info: Path, folder: Path, now: datetime) -> bool:
    """Create the ``.trashinfo`` of the freedesktop trash; false when the name is taken.

    Raises:
        _HostTrashUnusableError: The info file cannot be created or written.
    """
    text = (
        "[Trash Info]\n"
        f"Path={quote(str(folder), safe='/')}\n"
        f"DeletionDate={now.astimezone().strftime('%Y-%m-%dT%H:%M:%S')}\n"
    )
    try:
        fd = os.open(info, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError:
        return False
    except OSError:
        raise _HostTrashUnusableError from None
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            fh.write(text)
    except OSError:
        _forget(info)
        raise _HostTrashUnusableError from None
    return True


def _forget(info: Path) -> None:
    """Drop the info file this move reserved itself (never anything else)."""
    with contextlib.suppress(OSError):
        info.unlink()


def _fallback(
    folder: Path,
    projects_root: Path,
    now: datetime,
    on_target: Callable[[Path | None], None] | None,
) -> Moved:
    """Move into ``.bothy-trash`` directly under the projects root."""
    base = Path(os.path.realpath(projects_root)) / FALLBACK_DIR
    try:
        base.mkdir(mode=0o700, exist_ok=True)
        mode = os.lstat(base).st_mode
    except OSError:
        return Moved("move_failed")
    if stat.S_ISLNK(mode) or not stat.S_ISDIR(mode):
        return Moved("move_failed")  # a link would lead out of the root (D-DEL-RUN-12)
    stamp = now.astimezone(UTC).strftime("%Y%m%dT%H%M%SZ")
    for name in _names(f"{folder.name}-{stamp}"):
        target = base / name
        if os.path.lexists(target):
            continue
        if on_target is not None:
            on_target(target)
        try:
            folder.rename(target)
        except OSError:
            return Moved("move_failed")
        return Moved(None, "fallback")
    return Moved("move_failed")


def _kill_group(pid: int) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pid, signal.SIGKILL)


# ---- checks and requests of the runner ------------------------------------------------------


@dataclass
class _Check:
    workspace_ref: str
    reason: str | None


class FolderTrash:
    """The folder checks of the bound forums and the folder moves the server asks for.

    Args:
        projects_root: ``runner.projects_root``.
        state_dir: ``runner.state_dir`` (holds ``trash.json``).
        git: How ``git`` runs.
        places: Where the host's trash is.
        interval: ``runner.trash_check_interval`` in seconds.
        bound: Returns ``workspace_ref`` by channel of every bound forum the runner serves.
        clock: A monotonic clock for the interval.
        now: The time of a move.
        on_done: Called after requests were handled (the runner sends a heartbeat at once).
        mover: The move (tests replace the device).
    """

    def __init__(  # noqa: PLR0913 - the parts of the trash are explicit
        self,
        *,
        projects_root: Path,
        state_dir: Path,
        git: GitRunner,
        places: TrashPlaces,
        interval: float,
        bound: Callable[[], Mapping[UUID, str]],
        clock: Callable[[], float] | None = None,
        now: Callable[[], datetime] | None = None,
        on_done: Callable[[], Awaitable[None]] | None = None,
        mover: Callable[..., Moved] = move_to_trash,
    ) -> None:
        """Create the trash; the record of handled requests is read once here."""
        self._root = projects_root
        self._state_dir = state_dir
        self._record_path = state_dir / _RECORD
        self._git = git
        self._places = places
        self._interval = interval
        self._bound = bound
        self._clock = clock or time.monotonic
        self._now = now or (lambda: datetime.now(UTC))
        self._on_done = on_done
        self._mover = mover
        self._checks: dict[UUID, _Check] = {}
        self._checked_at: float | None = None
        self._record = self._load()
        self._queue: dict[UUID, ProjectTrashRequest] = {}
        self._stop: Callable[[UUID], Awaitable[bool]] | None = None
        self._job: asyncio.Task[None] | None = None

    # ---- checks ------------------------------------------------------------------------------

    async def refresh(self) -> bool:
        """Check every bound forum again, at most every ``interval``.

        Returns:
            True when a check was computed and an outcome differs from the last one.
        """
        now = self._clock()
        if self._checked_at is not None and now - self._checked_at < self._interval:
            return False
        self._checked_at = now
        bound = dict(self._bound())
        checks: dict[UUID, _Check] = {}
        for channel_id, ref in bound.items():
            reason = await self._check(channel_id, ref, bound)
            checks[channel_id] = _Check(ref, reason)
        changed = checks != self._checks
        self._checks = checks
        return changed

    async def _check(self, channel_id: UUID, ref: str, bound: Mapping[UUID, str]) -> str | None:
        others = [r for c, r in bound.items() if c != channel_id]
        return await check_folder(
            self._root, ref, others=others, state_dir=self._state_dir, git=self._git
        )

    def annotate(self, entries: Iterable[ProjectReadiness]) -> None:
        """Add the last check to each readiness entry whose binding it was computed for.

        Args:
            entries: The readiness entries of the heartbeat.
        """
        bound = self._bound()
        for entry in entries:
            check = self._checks.get(entry.channel_id)
            if check is None or bound.get(entry.channel_id) != check.workspace_ref:
                continue
            report = ProjectTrashCheckReport(ok=check.reason is None)
            if check.reason is not None:
                report.reason = check.reason
            entry.trash_check = report

    # ---- requests ----------------------------------------------------------------------------

    def requested(
        self, requests: Sequence[ProjectTrashRequest], *, stop: Callable[[UUID], Awaitable[bool]]
    ) -> None:
        """Take the requests of a heartbeat answer and handle new ones in the background.

        A request already handled is not moved again; when its result was sent but the server
        still asks, the result is sent again.

        Args:
            requests: ``HeartbeatAck.project_trash``.
            stop: Stops the turns of a channel; true once none runs any more.
        """
        changed = False
        for req in requests:
            key = str(req.channel_id)
            entry = self._record.get(key)
            if entry is not None and entry.get("state") == "done":
                if entry.get("sent"):
                    entry["sent"] = False
                    changed = True
                continue
            self._queue[req.channel_id] = req
        if changed:
            self._save()
        self._stop = stop
        if self._queue and (self._job is None or self._job.done()):
            with contextlib.suppress(RuntimeError):
                self._job = asyncio.get_running_loop().create_task(self._work())

    async def _work(self) -> None:
        handled = False
        while self._queue:
            channel_id, req = next(iter(self._queue.items()))
            try:
                handled = await self._handle(req) or handled
            except Exception:  # an unexpected error leaves the request to the next answer
                _log("project_trash_failed", warning=True, channel_id=channel_id)
            finally:
                self._queue.pop(channel_id, None)
        if handled and self._on_done is not None:
            await self._on_done()

    async def _handle(self, req: ProjectTrashRequest) -> bool:
        """One request: finish an interrupted move, or stop, check again and move."""
        key = str(req.channel_id)
        ref = req.workspace_ref
        entry = self._record.get(key)
        if entry is not None and entry.get("state") == "moving":
            finished = self._finish_interrupted(req, entry)
            if finished:
                return True
        stop = self._stop
        if stop is not None and not await stop(req.channel_id):
            _log("project_trash_deferred", channel_id=req.channel_id)
            return False
        bound = {c: r for c, r in self._bound().items() if c != req.channel_id}
        reason = await check_folder(
            self._root, ref, others=bound.values(), state_dir=self._state_dir, git=self._git
        )
        if reason is not None:
            self._done(req.channel_id, ref, "refused", reason)
            return True
        folder = resolve(self._root, ref)

        def remember(target: Path | None) -> None:
            self._record[key] = {
                "workspace_ref": ref,
                "state": "moving",
                "target": None if target is None else str(target),
            }
            self._save()

        moved = self._mover(folder, self._root, self._places, now=self._now(), on_target=remember)
        if moved.code is not None:
            self._done(req.channel_id, ref, "refused", moved.code)
        else:
            self._done(req.channel_id, ref, "trashed", None, place=moved.place)
        return True

    def _finish_interrupted(self, req: ProjectTrashRequest, entry: Mapping[str, Any]) -> bool:
        """A move a crash cut off: done when the folder left the root, else run it again."""
        try:
            folder: Path | None = resolve(self._root, req.workspace_ref)
        except WorkspaceError:
            folder = None
        if folder is not None and os.path.lexists(folder):
            return False
        target = entry.get("target")
        if target is None or os.path.lexists(str(target)):
            self._done(req.channel_id, req.workspace_ref, "trashed", None, place="recovered")
        else:
            self._done(req.channel_id, req.workspace_ref, "refused", "workspace_missing")
        return True

    def _done(
        self, channel_id: UUID, ref: str, result: str, reason: str | None, *, place: str = ""
    ) -> None:
        self._record[str(channel_id)] = {
            "workspace_ref": ref,
            "state": "done",
            "result": result,
            "reason": reason,
            "sent": False,
        }
        self._save()
        if result == "trashed":
            _log("project_trash_moved", channel_id=channel_id, place=place)
        else:
            _log("project_trash_refused", warning=True, channel_id=channel_id, code=reason)

    def results(self) -> list[ProjectTrashResult]:
        """The results not yet carried by an accepted heartbeat, in channel order.

        Returns:
            One entry per handled request whose result was not sent.
        """
        out = []
        for key in sorted(self._record):
            entry = self._record[key]
            if entry.get("state") != "done" or entry.get("sent"):
                continue
            result = ProjectTrashResult(
                channel_id=UUID(key), result=ProjectTrashResultResult(entry["result"])
            )
            if entry.get("reason"):
                result.reason = str(entry["reason"])
            out.append(result)
        return out

    def reported(self, channel_ids: Iterable[UUID]) -> None:
        """Mark the results of these channels as sent: an accepted heartbeat carried them.

        Args:
            channel_ids: The channels of ``project_trash_results`` of that heartbeat.
        """
        changed = False
        for channel_id in channel_ids:
            entry = self._record.get(str(channel_id))
            if entry is not None and entry.get("state") == "done" and not entry.get("sent"):
                entry["sent"] = True
                changed = True
        if changed:
            self._save()

    # ---- record and shutdown -----------------------------------------------------------------

    def _load(self) -> dict[str, dict[str, Any]]:
        try:
            data = json.loads(self._record_path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            return {}
        if not isinstance(data, dict):
            return {}
        out: dict[str, dict[str, Any]] = {}
        for key, value in data.items():
            try:
                UUID(str(key))
            except ValueError:
                continue
            if isinstance(value, dict) and value.get("state") in ("moving", "done"):
                out[str(key)] = dict(value)
        return out

    def _save(self) -> None:
        try:
            write_private(self._record_path, json.dumps(self._record, sort_keys=True), atomic=True)
        except OSError:
            _log("project_trash_record_unwritable", warning=True)

    async def wait_idle(self) -> None:
        """Wait until the queued requests are handled (tests and orderly shutdown)."""
        while self._job is not None and not self._job.done():
            await asyncio.gather(self._job, return_exceptions=True)

    async def close(self) -> None:
        """Cancel the handling of queued requests; a move in progress is a single rename."""
        job = self._job
        if job is not None and not job.done():
            job.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await job
