# SPDX-License-Identifier: Apache-2.0
"""Registered external jobs: background processes outside Hermes' registries (09 17.12 point 8).

A tool call of a run may start a process that Hermes does not track: a supervisor script that
detaches itself (a new session, the init process as its parent) and runs for hours. Hermes'
registries (``delegate_task`` and ``terminal`` with ``background``) never see it, so the adapter
could not hold the run for it. Such a process registers itself here, and the adapter treats a
live registered job of a thread like a background process of that thread's run (09 17.12).

A record names the thread, the PID, the process's start time (so a PID the system gave to
another process is never taken for the job), a title, an optional log path (kept on the host,
never sent) and the time the job started. Records live in ``<profile home>/<platform key>/jobs``,
next to the adapter's state:

* the folder is the adapter's (mode ``0700``, owned by the user that runs the gateway, no link);
  the helper creates it only below an existing state directory of the adapter;
* each record is one file ``<job ID>.json`` with mode ``0600``, written to a temporary name and
  renamed; a record that is a link, another user's, readable by others, too large or not a
  record is skipped (and logged once);
* the adapter stops a job only after it read the PID's start time again: it signals the job's
  process group when the job leads one (a supervisor started with ``start_new_session``), else
  the process alone, and never its own process or process group.

Two ways in for a script of the profile, both without the network and without Hermes:

* `JobRegistry.register` and `JobRegistry.deregister` (the folder from `jobs_dir`);
* ``python -m bothy_hermes_adapter job register|deregister|list|stop`` (also as
  ``python -m bothy_hermes_adapter.jobs``), which prints one JSON line per result; ``stop`` is
  the operator's way to end a job that has no run (and so no stop button). The thread and
  the profile home are arguments: a shell started by Hermes' ``terminal`` tool has them in
  ``HERMES_SESSION_THREAD_ID`` and ``HERMES_HOME``.

Examples:
    >>> jobs_dir(Path("/srv/profile"), "bothy").as_posix()
    '/srv/profile/bothy/jobs'
    >>> process_start(0) is None
    True
"""

from __future__ import annotations

import argparse
import contextlib
import ctypes
import json
import logging
import math
import os
import re
import signal
import stat
import struct
import sys
import time
import uuid
from collections.abc import Sequence
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any, Final

__all__ = [
    "JobError",
    "JobRecord",
    "JobRegistry",
    "jobs_dir",
    "main",
    "process_start",
]

_LOG = logging.getLogger("bothy_hermes_adapter")

JOBS_DIR: Final = "jobs"
"""Name of the folder below the adapter's state directory."""
RECORD_VERSION: Final = 1
TITLE_MAX: Final = 1000
"""Longest title a record keeps (the step shows at most ``thread.title_max_chars``)."""
KILL_GRACE_S: Final = 5.0
"""How long a job has to end after ``SIGTERM`` before its group gets ``SIGKILL``."""
_RECORD_MAX_BYTES: Final = 16384
# The latest job start a record may name: the end of year 9999 UTC, the last time a date shows.
_STARTED_MAX: Final = 253402300799.0
_JOB_ID: Final = re.compile(r"[0-9a-f]{32}")
_POLL_S: Final = 0.05
# macOS: sysctl(CTL_KERN, KERN_PROC, KERN_PROC_PID, pid) fills a ``struct kinfo_proc``; its
# ``kp_proc.p_starttime`` (a timeval) is at offset 0 and ``kp_proc.p_stat`` at offset 36.
_CTL_KERN: Final = 1
_KERN_PROC: Final = 14
_KERN_PROC_PID: Final = 1
_KINFO_SIZE: Final = 648
_P_STAT_OFFSET: Final = 36
_SZOMB: Final = 5


class JobError(Exception):
    """A registration or a read was refused; the message names the reason, never a content."""


@dataclass(frozen=True)
class JobRecord:
    """One registered job.

    Attributes:
        job_id: The record's ID (32 lower-case hex digits).
        thread_id: The thread whose run started the job.
        pid: The job's process (the leader of its process group, as a rule).
        pid_start: The process's start time as `process_start` tells it.
        title: What the job does (the step title).
        started_at: When the job started, seconds since the epoch.
        log_path: Where the job writes its log, if anywhere (kept on the host).
    """

    job_id: str
    thread_id: uuid.UUID
    pid: int
    pid_start: str
    title: str = field(repr=False)
    started_at: float
    log_path: str | None = field(default=None, repr=False)


def jobs_dir(profile_home: Path, platform_key: str) -> Path:
    """The registry folder of a profile's adapter: ``<profile home>/<platform key>/jobs``."""
    return Path(profile_home) / platform_key / JOBS_DIR


def process_start(pid: int) -> str | None:
    """The start time of a running process, as a text that differs for a reused PID.

    Args:
        pid: The process.

    Returns:
        The start time, or ``None`` when no such process runs (also a zombie).

    Raises:
        JobError: The system cannot tell (an unsupported platform or an unreadable answer).
    """
    if pid <= 0:
        return None
    # A plain string, so that mypy checks both branches on either platform's gate.
    system: str = sys.platform
    if system.startswith("linux"):
        return _linux_start(pid)
    if system == "darwin":
        return _darwin_start(pid)
    raise JobError(f"process start times are not readable on {system}")


def _linux_start(pid: int) -> str | None:
    try:
        data = Path(f"/proc/{pid}/stat").read_text(encoding="ascii", errors="replace")
    except FileNotFoundError:
        return None
    except OSError as exc:
        raise JobError(f"process {pid} not readable: {type(exc).__name__}") from None
    fields = data[data.rfind(")") + 2 :].split()
    if len(fields) < 20:
        raise JobError(f"process {pid} not readable: short stat")
    if fields[0] in ("Z", "X"):
        return None
    return f"linux:{fields[19]}"


def _darwin_start(pid: int) -> str | None:
    libc = ctypes.CDLL(None, use_errno=True)
    mib = (ctypes.c_int * 4)(_CTL_KERN, _KERN_PROC, _KERN_PROC_PID, pid)
    buf = ctypes.create_string_buffer(_KINFO_SIZE)
    size = ctypes.c_size_t(_KINFO_SIZE)
    if libc.sysctl(mib, 4, buf, ctypes.byref(size), None, 0) != 0:
        raise JobError(f"process {pid} not readable: errno {ctypes.get_errno()}")
    if size.value == 0:
        return None
    if size.value < _P_STAT_OFFSET + 1:
        raise JobError(f"process {pid} not readable: short answer")
    raw = buf.raw[: size.value]
    if raw[_P_STAT_OFFSET] == _SZOMB:
        return None
    seconds, micros = struct.unpack_from("=qi", raw, 0)
    return f"darwin:{seconds}.{micros:06d}"


def _owned(st: os.stat_result) -> bool:
    """Whether a file or folder belongs to the user that runs this process."""
    return st.st_uid == os.geteuid()


def _check_dir(path: Path, what: str) -> None:
    try:
        st = path.lstat()
    except FileNotFoundError:
        raise JobError(f"the {what} does not exist") from None
    if stat.S_ISLNK(st.st_mode):
        raise JobError(f"the {what} is a link")
    if not stat.S_ISDIR(st.st_mode):
        raise JobError(f"the {what} is no folder")
    if not _owned(st):
        raise JobError(f"the {what} has another owner")
    if st.st_mode & 0o077:
        raise JobError(f"the {what} has open permissions")


class JobRegistry:
    """The registry folder of one adapter (see the module text for its rules).

    Args:
        directory: The folder (`jobs_dir`).
    """

    def __init__(self, directory: Path) -> None:
        """Remember the folder; nothing is read or created yet."""
        self.directory = Path(directory)
        self._skipped: set[str] = set()

    # ---- the folder ----------------------------------------------------------------------------

    def prepare(self) -> None:
        """Create the folder (mode ``0700``) below the adapter's existing state directory.

        Raises:
            JobError: The state directory is missing or the folder fails the checks.
        """
        _check_dir(self.directory.parent, "adapter's state directory")
        with contextlib.suppress(FileExistsError):
            self.directory.mkdir(mode=0o700)
        _check_dir(self.directory, "jobs folder")

    def _folder(self) -> Path:
        _check_dir(self.directory, "jobs folder")
        return self.directory

    # ---- writing -------------------------------------------------------------------------------

    def register(
        self,
        thread_id: uuid.UUID,
        pid: int,
        title: str,
        *,
        log_path: str | None = None,
        started_at: float | None = None,
    ) -> JobRecord:
        """Register a running process as a job of a thread.

        Args:
            thread_id: The thread whose run started the job.
            pid: The process; it must run now.
            title: What the job does (one line is shown).
            log_path: Where the job writes its log, if anywhere.
            started_at: When the job started (default: now).

        Returns:
            The record.

        Raises:
            JobError: The process does not run, the input is invalid, or the folder fails the
                checks; nothing was written.
        """
        if pid <= 1:
            raise JobError("the PID is no job's")
        if not title.strip():
            raise JobError("the title is empty")
        self.prepare()
        start = process_start(pid)
        if start is None:
            raise JobError(f"process {pid} is not running")
        record = JobRecord(
            job_id=uuid.uuid4().hex,
            thread_id=thread_id,
            pid=pid,
            pid_start=start,
            title=title.strip()[:TITLE_MAX],
            started_at=time.time() if started_at is None else float(started_at),
            log_path=log_path,
        )
        if not _valid_start(record.started_at):
            raise JobError("the start time is no time")
        payload = asdict(record) | {"version": RECORD_VERSION, "thread_id": str(thread_id)}
        data = json.dumps(payload).encode("utf-8")
        if len(data) > _RECORD_MAX_BYTES:
            raise JobError("the record is too large (shorten the title or the log path)")
        self._write(f"{record.job_id}.json", data)
        return record

    def _write(self, name: str, data: bytes) -> None:
        folder = self._folder()
        temporary = folder / f".{name}.{uuid.uuid4().hex}.tmp"
        flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
        fd = os.open(temporary, flags, 0o600)
        try:
            os.fchmod(fd, 0o600)
            os.write(fd, data)
            os.fsync(fd)
        finally:
            os.close(fd)
        try:
            temporary.replace(folder / name)
        except OSError:
            temporary.unlink(missing_ok=True)
            raise

    def deregister(self, job_id: str) -> bool:
        """Remove a job's record (the job ended); returns whether there was one.

        Raises:
            JobError: The ID is no job ID or the folder fails the checks.
        """
        if not _JOB_ID.fullmatch(job_id):
            raise JobError("no job ID")
        # Only entries the folder lists are removed, so no name from outside forms a path.
        names = {f"{job_id}.reported", f"{job_id}.json"}
        found = False
        for entry in self._folder().iterdir():
            if entry.name in names:
                entry.unlink(missing_ok=True)
                found = found or entry.suffix == ".json"
        return found

    def reported(self, job_id: str) -> bool:
        """Whether the adapter already posted the job as one without a run (09 17.12 point 8)."""
        return (self._folder() / f"{job_id}.reported").is_file()

    def mark_reported(self, job_id: str) -> None:
        """Remember across restarts that the job was posted as one without a run."""
        if not _JOB_ID.fullmatch(job_id):
            raise JobError("no job ID")
        self._write(f"{job_id}.reported", b"")

    # ---- reading -------------------------------------------------------------------------------

    def records(self) -> list[JobRecord]:
        """Every valid record, by start time; invalid ones are skipped and logged once.

        Raises:
            JobError: The folder fails the checks (a missing folder holds no record).
        """
        try:
            folder = self._folder()
        except JobError as exc:
            if not self.directory.exists() and not self.directory.is_symlink():
                return []
            raise exc from None
        found: list[JobRecord] = []
        for path in sorted(folder.glob("*.json")):
            record = self._read(path)
            if record is not None:
                found.append(record)
        return sorted(found, key=lambda r: (r.started_at, r.job_id))

    def _read(self, path: Path) -> JobRecord | None:
        try:
            fd = os.open(path, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | os.O_NONBLOCK)
        except FileNotFoundError:
            return None  # deregistered in the meantime
        except OSError:
            self._skip(path, "a link or not readable")
            return None
        try:
            st = os.fstat(fd)
            if not stat.S_ISREG(st.st_mode):
                self._skip(path, "no file")
                return None
            if not _owned(st):
                self._skip(path, "another owner")
                return None
            if st.st_mode & 0o077:
                self._skip(path, "open permissions")
                return None
            if st.st_size > _RECORD_MAX_BYTES:
                self._skip(path, "too large")
                return None
            data = os.read(fd, _RECORD_MAX_BYTES + 1)
        finally:
            os.close(fd)
        record = _parse(data, path.stem)
        if record is None:
            self._skip(path, "no record")
            return None
        return record

    def _skip(self, path: Path, reason: str) -> None:
        if path.name not in self._skipped:
            self._skipped.add(path.name)
            _LOG.warning("job record %s skipped: %s", path.name, reason)

    def alive(self, record: JobRecord) -> bool:
        """Whether the record's process runs and is the one registered (same start time).

        Raises:
            JobError: The system cannot tell.
        """
        return process_start(record.pid) == record.pid_start

    def sweep(self) -> list[JobRecord]:
        """Remove the records of ended jobs and stray marks; return the live records.

        A job whose process cannot be checked counts as live (it is never removed unseen).

        Raises:
            JobError: The folder fails the checks.
        """
        live: list[JobRecord] = []
        for record in self.records():
            try:
                running = self.alive(record)
            except JobError:
                running = True
            if running:
                live.append(record)
            else:
                with contextlib.suppress(JobError):
                    self.deregister(record.job_id)
        if self.directory.is_dir():
            known = {r.job_id for r in live}
            for mark in self._folder().glob("*.reported"):
                if mark.stem not in known:
                    mark.unlink(missing_ok=True)
        return live

    # ---- stopping ------------------------------------------------------------------------------

    def stop(self, record: JobRecord, grace: float = KILL_GRACE_S) -> bool:
        """Stop a live job: ``SIGTERM``, then ``SIGKILL`` after ``grace`` seconds.

        Every signal goes out only after the PID's start time was read again and matched. A job
        that leads its process group gets the signal for the whole group, any other job alone;
        the adapter's own process and process group are never signalled.

        Args:
            record: The job.
            grace: Seconds between ``SIGTERM`` and ``SIGKILL``.

        Returns:
            Whether a signal was sent (False: the job had ended, the PID was reused, or the job
            is the adapter's own process).
        """
        if record.pid == os.getpid() or not self._signal(record, signal.SIGTERM):
            return False
        deadline = time.monotonic() + grace
        while time.monotonic() < deadline:
            if not self._still(record):
                return True
            time.sleep(_POLL_S)
        self._signal(record, signal.SIGKILL)
        return True

    def _still(self, record: JobRecord) -> bool:
        try:
            return self.alive(record)
        except JobError:
            return False

    def _signal(self, record: JobRecord, signum: int) -> bool:
        if not self._still(record):
            return False
        try:
            group = os.getpgid(record.pid)
            if group == record.pid and group != os.getpgrp():
                os.killpg(group, signum)
            else:
                os.kill(record.pid, signum)
        except ProcessLookupError:
            return False
        except PermissionError:
            _LOG.warning("job %s: signal refused by the system", record.job_id)
            return False
        _LOG.info("job %s: signal %s sent", record.job_id, int(signum))
        return True


def _parse(data: bytes, stem: str) -> JobRecord | None:
    try:
        raw: Any = json.loads(data)
    except ValueError:
        return None
    if not isinstance(raw, dict) or raw.get("version") != RECORD_VERSION:
        return None
    job_id, thread, pid, start = (raw.get(k) for k in ("job_id", "thread_id", "pid", "pid_start"))
    title, started, log = (raw.get(k) for k in ("title", "started_at", "log_path"))
    if not (isinstance(job_id, str) and job_id == stem and _JOB_ID.fullmatch(job_id)):
        return None
    if not isinstance(pid, int) or isinstance(pid, bool) or pid <= 1:
        return None
    if not (isinstance(start, str) and start and isinstance(title, str) and title.strip()):
        return None
    if not isinstance(started, int | float) or isinstance(started, bool):
        return None
    if not _valid_start(started):
        return None
    if log is not None and not isinstance(log, str):
        return None
    try:
        thread_id = uuid.UUID(str(thread))
    except ValueError:
        return None
    return JobRecord(job_id, thread_id, pid, start, title[:TITLE_MAX], float(started), log)


def _valid_start(started: float) -> bool:
    """Whether a job start is a time a date can show (finite, not before the epoch).

    An integer is compared as it is: one too large for a float would overflow its conversion.
    """
    if isinstance(started, int):
        return 0 <= started <= _STARTED_MAX
    return math.isfinite(started) and 0.0 <= started <= _STARTED_MAX


# ---- command line ------------------------------------------------------------------------------


def build_parser(prog: str = "python -m bothy_hermes_adapter job") -> argparse.ArgumentParser:
    """The parser of ``register``, ``deregister``, ``list`` and ``stop``."""
    parser = argparse.ArgumentParser(prog=prog)
    sub = parser.add_subparsers(dest="action", required=True)
    reg = sub.add_parser("register", help="register a running process as a job of a thread")
    reg.add_argument("--thread", required=True, help="the thread ID (HERMES_SESSION_THREAD_ID)")
    reg.add_argument("--pid", type=int, required=True)
    reg.add_argument("--title", required=True)
    reg.add_argument("--log")
    reg.add_argument("--started-at", type=float)
    dereg = sub.add_parser("deregister", help="remove a job's record (the job ended)")
    dereg.add_argument("--job-id", required=True)
    sub.add_parser("list", help="print every record with whether its process runs")
    halt = sub.add_parser("stop", help="stop a job's process group and remove its record")
    halt.add_argument("--job-id", required=True)
    for each in sub.choices.values():
        each.add_argument("--profile-home", type=Path, required=True, help="HERMES_HOME")
        each.add_argument("--platform-key", default="bothy")
    return parser


def _emit(value: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(value) + "\n")


def _register(registry: JobRegistry, args: argparse.Namespace) -> int:
    try:
        thread = uuid.UUID(args.thread)
    except ValueError:
        raise JobError("the thread is no thread ID") from None
    record = registry.register(
        thread, args.pid, args.title, log_path=args.log, started_at=args.started_at
    )
    _emit({"ok": True, "job_id": record.job_id})
    return 0


def _stop(registry: JobRegistry, args: argparse.Namespace) -> int:
    matching = [r for r in registry.records() if r.job_id == args.job_id]
    stopped = bool(matching) and registry.stop(matching[0])
    if matching:
        registry.deregister(args.job_id)
    _emit({"ok": stopped, "job_id": args.job_id})
    return 0 if stopped else 1


def _deregister(registry: JobRegistry, args: argparse.Namespace) -> int:
    found = registry.deregister(args.job_id)
    _emit({"ok": found, "job_id": args.job_id})
    return 0 if found else 1


def _list(registry: JobRegistry, _args: argparse.Namespace) -> int:
    for record in registry.records():
        try:
            alive: bool | None = registry.alive(record)
        except JobError:
            alive = None
        _emit(
            {
                "job_id": record.job_id,
                "thread_id": str(record.thread_id),
                "pid": record.pid,
                "title": record.title,
                "started_at": record.started_at,
                "log_path": record.log_path,
                "alive": alive,
            }
        )
    return 0


_ACTIONS: Final = {"register": _register, "stop": _stop, "deregister": _deregister, "list": _list}


def main(argv: Sequence[str] | None = None, prog: str | None = None) -> int:
    """Run one command and print its result as JSON lines.

    Returns:
        ``0`` on success, ``1`` when ``deregister`` or ``stop`` found no record (``stop``: or
        no live job), ``2`` on a refusal.
    """
    args = build_parser(prog or "python -m bothy_hermes_adapter.jobs").parse_args(argv)
    registry = JobRegistry(jobs_dir(args.profile_home, args.platform_key))
    try:
        return _ACTIONS[args.action](registry, args)
    except (JobError, OSError) as exc:
        message = str(exc) if isinstance(exc, JobError) else type(exc).__name__
        _emit({"ok": False, "error": message})
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
