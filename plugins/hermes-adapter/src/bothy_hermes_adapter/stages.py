# SPDX-License-Identifier: Apache-2.0
"""Stage events of a coding chain: a ``stages.jsonl`` file followed into the run (09 17.12 point 9).

A tool call of a run may start a long command that writes its progress as one JSON line per
event into a file (the coding chain of a Hermes profile writes ``stages.jsonl`` in its run
folder, one line per start, further pass and end of a stage and of the chain's run). Only the
adapter holds the run's patch channel, so the command does not talk to the server: the tool call
registers the file as a stage source of its thread, and the adapter reads the lines it has not
seen every ``hermes_adapter.background_poll_interval`` while a run of that thread is open, and
turns them into steps and the status line of that run (`StageSteps`).

A line has the members ``at``, ``ticket``, ``run``, ``stage``, ``state``, ``outcome`` (only on
``ended``) and ``text``. Only checked words reach the server: the stage of `STAGES` or ``run``,
the state of `STATES`, the outcome (an outcome off the lists is shown as ``unknown``), the ticket
key and a pass number; the shown text is rebuilt from them, never copied (`render`). A line that
is no JSON object, names an unknown stage or state, a ticket key of another form or of another
ticket than the registration names, or is longer than `LINE_MAX_BYTES`, is skipped and logged
with a reason code only, never with its content.

Records live in ``<profile home>/<platform key>/stages`` with the rules of the job registry
(`bothy_hermes_adapter.jobs`): the folder mode ``0700``, owned by the gateway's user, no link;
each record ``<source ID>.json`` and its cursor ``<source ID>.cursor`` mode ``0600``, written to a
temporary name and renamed. The record holds the thread, the file's absolute path, the ticket key
if named, the time of the registration and the offset where reading starts: the file's size at
the registration, so lines of an earlier call of the command are never shown again. The cursor
holds what the adapter has read (offset, the file's identity, the run it read for and the steps
it opened there), so a reader that starts again (another poll, a later run of the thread after a
restart) goes on from there and never shows a line twice.

A source is retired (record and cursor removed) when its ``run`` ``ended`` line was read, when it
is deregistered, when a newer registration of the same file in the same thread replaces it, and
when it is older than `SOURCE_MAX_AGE_S` at the next registration.

Command line, without the network and without Hermes (like ``job``):
``python -m bothy_hermes_adapter stages register --profile-home <home> --thread <thread ID>
--file <path> [--ticket <key>]``, ``stages deregister --source-id <ID>`` and ``stages list``;
each prints one JSON line per result. A shell started by Hermes' ``terminal`` tool has the thread
and the profile home in ``HERMES_SESSION_THREAD_ID`` and ``HERMES_HOME``.

Examples:
    >>> stages_dir(Path("/srv/profile"), "bothy").as_posix()
    '/srv/profile/bothy/stages'
    >>> parse_line(b'{"ticket":"TOY-7","stage":"build","state":"started"}').text
    'build started'
    >>> parse_line(b'{"ticket":"TOY-7","stage":"deploy","state":"started"}')
    'unknown_stage'
"""

from __future__ import annotations

import argparse
import contextlib
import json
import logging
import os
import re
import stat
import sys
import time
import uuid
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field, replace
from datetime import datetime
from pathlib import Path
from typing import Any, Final

from bothy_hermes_adapter.jobs import JobError, _check_dir, _owned

__all__ = [
    "STAGES",
    "STATES",
    "StageCursor",
    "StageEvent",
    "StageRegistry",
    "StageSource",
    "StageSteps",
    "main",
    "parse_line",
    "read_new",
    "stages_dir",
]

_LOG = logging.getLogger("bothy_hermes_adapter")

STAGES_DIR: Final = "stages"
"""Name of the folder below the adapter's state directory."""
RECORD_VERSION: Final = 1
STAGES: Final = (
    "intake",
    "triage",
    "plan",
    "plan-approval",
    "base-check",
    "build",
    "evidence",
    "review",
    "red-team",
    "ship",
    "ci",
)
"""The stages of the chain (its ``STAGES``); ``run`` names the chain's run itself."""
RUN: Final = "run"
STATES: Final = ("started", "pass", "ended")
STAGE_OUTCOMES: Final = (
    "passed",
    "passed-direct",
    "question",
    "declined",
    "needs-human",
    "failed",
    "base-red",
    "not-taken",
    "red",
    "unknown",
)
"""The outcomes of a stage the chain writes (``unknown`` stands for one off its list)."""
END_STATES: Final = (
    "not-taken",
    "declined",
    "question",
    "needs-human",
    "failed",
    "base-red",
    "halted",
    "ready-for-review",
    "code-done",
)
"""The end states of the chain's run (its ``END_STATES``)."""
FAILED_OUTCOMES: Final = frozenset({"failed", "base-red", "red"})
"""Outcomes that end a stage's step, or the open steps at the chain's end, ``failed``."""
CANCELLED_ENDS: Final = frozenset({"halted"})
"""End states that end the open steps ``cancelled``."""
STEP_KIND: Final = "workflow"
STEP_NAME: Final = "chain_stage"
LINE_MAX_BYTES: Final = 4096
"""Longest line that is read; a longer one is skipped (the chain writes about 200 bytes)."""
READ_MAX_BYTES: Final = 262144
"""Most bytes read from one file per poll; the rest is read at the next poll."""
SOURCE_MAX_AGE_S: Final = 7 * 24 * 3600.0
"""A registration older than this is removed at the next registration of the profile."""
_RECORD_MAX_BYTES: Final = 16384
_PATH_MAX: Final = 4096
_SOURCE_ID: Final = re.compile(r"[0-9a-f]{32}")
_TICKET: Final = re.compile(r"[A-Za-z][A-Za-z0-9_]{0,31}-[0-9]{1,12}")


# ---- lines ---------------------------------------------------------------------------------------


@dataclass(frozen=True)
class StageEvent:
    """One checked line.

    Attributes:
        ticket: The ticket key.
        stage: A stage of `STAGES`, or ``run``.
        state: ``started``, ``pass`` or ``ended``.
        outcome: The outcome of a stage or the end state of the run (``ended`` only).
        pass_no: The number of a further pass, when the line names one.
    """

    ticket: str
    stage: str
    state: str
    outcome: str | None = None
    pass_no: int | None = None

    @property
    def text(self) -> str:
        """The text of the line, rebuilt from its checked words (`render`)."""
        return render(self)


def render(event: StageEvent) -> str:
    """The chain's text of an event, made from the checked words only."""
    if event.state == "started":
        return f"{event.stage} started"
    if event.state == "pass":
        return f"{event.stage} pass" + (f" {event.pass_no}" if event.pass_no else "")
    return f"{event.stage} ended: {event.outcome}"


def parse_line(data: bytes, ticket: str | None = None) -> StageEvent | str:
    """Check one line.

    Args:
        data: The line without its newline.
        ticket: The ticket key the registration names, if any.

    Returns:
        The event, or the reason code why the line is skipped (``too_long``, ``malformed``,
        ``bad_ticket``, ``other_ticket``, ``unknown_stage``, ``unknown_state``).
    """
    if len(data) > LINE_MAX_BYTES:
        return "too_long"
    try:
        raw: Any = json.loads(data)
    except (ValueError, UnicodeDecodeError):
        return "malformed"
    if not isinstance(raw, dict):
        return "malformed"
    key, stage, state = raw.get("ticket"), raw.get("stage"), raw.get("state")
    if not isinstance(key, str) or not _TICKET.fullmatch(key):
        return "bad_ticket"
    if ticket is not None and key != ticket:
        return "other_ticket"
    if not isinstance(stage, str) or (stage not in STAGES and stage != RUN):
        return "unknown_stage"
    if not isinstance(state, str) or state not in STATES or (stage == RUN and state == "pass"):
        return "unknown_state"
    if state == "started":
        return StageEvent(key, stage, state)
    if state == "pass":
        return StageEvent(key, stage, state, pass_no=_pass_no(raw.get("text"), stage))
    known = END_STATES if stage == RUN else STAGE_OUTCOMES
    outcome = raw.get("outcome")
    return StageEvent(key, stage, state, outcome if outcome in known else "unknown")


def _pass_no(text: Any, stage: str) -> int | None:
    if not isinstance(text, str):
        return None
    found = re.fullmatch(re.escape(stage) + r" pass ([1-9][0-9]{0,3})", text)
    return int(found.group(1)) if found else None


@dataclass
class StageCursor:
    """What the adapter has read of one source.

    Attributes:
        offset: The byte after the last line read.
        identity: Device and inode of the file read, ``None`` before the file existed.
        skipping: Whether a line longer than `LINE_MAX_BYTES` is being skipped to its end.
        run_id: The run the steps of ``open`` belong to.
        open: The open step of each stage in that run, by stage.
    """

    offset: int
    identity: tuple[int, int] | None = None
    skipping: bool = False
    run_id: str | None = None
    open: dict[str, str] = field(default_factory=dict)


def read_new(path: str, cursor: StageCursor) -> tuple[list[bytes], list[str]]:
    """Read the complete lines after ``cursor`` and move it past them.

    A missing file holds no line yet. A file whose identity differs from the cursor's (replaced)
    or that is shorter than the offset (truncated) is read from its start. A partial last line
    stays for the next read; a line longer than `LINE_MAX_BYTES` is skipped to its end.

    Args:
        path: The registered file.
        cursor: The cursor; changed in place.

    Returns:
        The lines and the reason codes of what was skipped or reset.
    """
    notes: list[str] = []
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | getattr(os, "O_NOFOLLOW", 0))
    except FileNotFoundError:
        return [], notes
    try:
        st = os.fstat(fd)
        if not stat.S_ISREG(st.st_mode):
            return [], ["not_a_file"]
        identity = (st.st_dev, st.st_ino)
        if cursor.identity is not None and identity != cursor.identity:
            notes.append("replaced")
            cursor.offset, cursor.skipping = 0, False
        elif st.st_size < cursor.offset:
            notes.append("truncated")
            cursor.offset, cursor.skipping = 0, False
        cursor.identity = identity
        if st.st_size <= cursor.offset:
            return [], notes
        os.lseek(fd, cursor.offset, os.SEEK_SET)
        chunk = os.read(fd, min(READ_MAX_BYTES, st.st_size - cursor.offset))
    finally:
        os.close(fd)
    lines: list[bytes] = []
    start = 0
    if cursor.skipping:
        end = chunk.find(b"\n")
        if end < 0:
            cursor.offset += len(chunk)
            return [], notes
        start, cursor.skipping = end + 1, False
    last = chunk.rfind(b"\n")
    if last >= start:
        lines = chunk[start:last].split(b"\n")
        consumed = last + 1
    else:
        consumed = start
    rest = len(chunk) - consumed
    if rest > LINE_MAX_BYTES:
        notes.append("too_long")
        consumed, cursor.skipping = len(chunk), True
    cursor.offset += consumed
    return [line.rstrip(b"\r") for line in lines if line.strip()], notes


# ---- the registry --------------------------------------------------------------------------------


@dataclass(frozen=True)
class StageSource:
    """One registered stage file.

    Attributes:
        source_id: The record's ID (32 lower-case hex digits).
        thread_id: The thread whose run's tool call registered it.
        path: The file's absolute path (kept on the host, never sent or logged).
        offset: Where reading starts (the file's size at the registration).
        ticket: The ticket key whose lines count, ``None`` for every key.
        registered_at: When it was registered, seconds since the epoch.
    """

    source_id: str
    thread_id: uuid.UUID
    path: str = field(repr=False)
    offset: int
    ticket: str | None
    registered_at: float


def stages_dir(profile_home: Path, platform_key: str) -> Path:
    """The registry folder of a profile's adapter: ``<profile home>/<platform key>/stages``."""
    return Path(profile_home) / platform_key / STAGES_DIR


class StageRegistry:
    """The stage sources of one adapter (see the module text for its rules).

    Args:
        directory: The folder (`stages_dir`).
        clock: The wall time in seconds (tests pass their own).
    """

    def __init__(self, directory: Path, clock: Callable[[], float] = time.time) -> None:
        """Remember the folder; nothing is read or created yet."""
        self.directory = Path(directory)
        self._clock = clock
        self._skipped: set[str] = set()

    def prepare(self) -> None:
        """Create the folder (mode ``0700``) below the adapter's existing state directory.

        Raises:
            JobError: The state directory is missing or the folder fails the checks.
        """
        _check_dir(self.directory.parent, "adapter's state directory")
        with contextlib.suppress(FileExistsError):
            self.directory.mkdir(mode=0o700)
        _check_dir(self.directory, "stages folder")

    def _folder(self) -> Path:
        _check_dir(self.directory, "stages folder")
        return self.directory

    def register(
        self, thread_id: uuid.UUID, path: str | Path, ticket: str | None = None
    ) -> StageSource:
        """Register a stage file of a thread; reading starts at its current size.

        Args:
            thread_id: The thread whose run's tool call registers it.
            path: The file; absolute; it need not exist yet.
            ticket: The ticket key whose lines count, if only one.

        Returns:
            The record.

        Raises:
            JobError: The input is invalid or the folder fails the checks; nothing was written.
        """
        text = os.fspath(path)
        if not Path(text).is_absolute() or len(text) > _PATH_MAX or "\x00" in text:
            raise JobError("the file is no absolute path")
        if ticket is not None and not _TICKET.fullmatch(ticket):
            raise JobError("the ticket is no ticket key")
        self.prepare()
        try:
            st = Path(text).lstat()
        except FileNotFoundError:
            offset, identity = 0, None
        else:
            if not stat.S_ISREG(st.st_mode):
                raise JobError("the file is no regular file")
            offset, identity = st.st_size, (st.st_dev, st.st_ino)
        now = self._clock()
        record = StageSource(uuid.uuid4().hex, thread_id, text, offset, ticket, now)
        for old in self.records():
            stale = now - old.registered_at > SOURCE_MAX_AGE_S
            if stale or (old.thread_id == thread_id and old.path == text):
                self.retire(old.source_id)
        payload = {
            "version": RECORD_VERSION,
            "source_id": record.source_id,
            "thread_id": str(thread_id),
            "path": text,
            "offset": offset,
            "ticket": ticket,
            "registered_at": now,
        }
        self._write(f"{record.source_id}.json", json.dumps(payload).encode("utf-8"))
        self.save_cursor(record.source_id, StageCursor(offset, identity))
        return record

    def _write(self, name: str, data: bytes) -> None:
        folder = self._folder()
        temporary = folder / f".{name}.{uuid.uuid4().hex}.tmp"
        flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
        fd = os.open(temporary, flags, 0o600)
        try:
            os.fchmod(fd, 0o600)
            os.write(fd, data)
        finally:
            os.close(fd)
        try:
            temporary.replace(folder / name)
        except OSError:
            temporary.unlink(missing_ok=True)
            raise

    def retire(self, source_id: str) -> bool:
        """Remove a source's record and cursor; returns whether there was a record.

        Raises:
            JobError: The ID is no source ID or the folder fails the checks.
        """
        if not _SOURCE_ID.fullmatch(source_id):
            raise JobError("no source ID")
        names = {f"{source_id}.json", f"{source_id}.cursor"}
        found = False
        for entry in self._folder().iterdir():
            if entry.name in names:
                entry.unlink(missing_ok=True)
                found = found or entry.suffix == ".json"
        return found

    def records(self) -> list[StageSource]:
        """Every valid record, by registration time; invalid ones are skipped and logged once.

        Raises:
            JobError: The folder fails the checks (a missing folder holds no record).
        """
        try:
            folder = self._folder()
        except JobError:
            if not self.directory.exists() and not self.directory.is_symlink():
                return []
            raise
        found = []
        for path in sorted(folder.glob("*.json")):
            data = self._read_private(path)
            record = None if data is None else _parse_record(data, path.stem)
            if record is not None:
                found.append(record)
            elif data is not None:
                self._skip(path.name, "no record")
        return sorted(found, key=lambda r: (r.registered_at, r.source_id))

    def cursor(self, source: StageSource) -> StageCursor:
        """The source's cursor; a missing or unreadable one starts at the registered offset."""
        data = self._read_private(self.directory / f"{source.source_id}.cursor")
        cursor = None if data is None else _parse_cursor(data)
        return cursor if cursor is not None else StageCursor(source.offset)

    def save_cursor(self, source_id: str, cursor: StageCursor) -> None:
        """Write a source's cursor (mode ``0600``, renamed into place)."""
        payload = {
            "version": RECORD_VERSION,
            "offset": cursor.offset,
            "identity": list(cursor.identity) if cursor.identity else None,
            "skipping": cursor.skipping,
            "run_id": cursor.run_id,
            "open": cursor.open,
        }
        self._write(f"{source_id}.cursor", json.dumps(payload).encode("utf-8"))

    def _read_private(self, path: Path) -> bytes | None:
        try:
            fd = os.open(path, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | os.O_NONBLOCK)
        except FileNotFoundError:
            return None
        except OSError:
            self._skip(path.name, "a link or not readable")
            return None
        try:
            st = os.fstat(fd)
            reason = None
            if not stat.S_ISREG(st.st_mode):
                reason = "no file"
            elif not _owned(st):
                reason = "another owner"
            elif st.st_mode & 0o077:
                reason = "open permissions"
            elif st.st_size > _RECORD_MAX_BYTES:
                reason = "too large"
            if reason is not None:
                self._skip(path.name, reason)
                return None
            return os.read(fd, _RECORD_MAX_BYTES + 1)
        finally:
            os.close(fd)

    def _skip(self, name: str, reason: str) -> None:
        if name not in self._skipped:
            self._skipped.add(name)
            _LOG.warning("stage record %s skipped: %s", name, reason)

    def collect(
        self,
        thread_id: uuid.UUID,
        run_id: str,
        known: Mapping[str, StageCursor] | None = None,
    ) -> list[tuple[StageSource, StageCursor, list[StageEvent]]]:
        """Read the new lines of every source of a thread for one of its runs.

        The cursors move past what was read but are not saved: the caller saves each one after
        it applied the events (`save_cursor`). Skipped lines are logged with their reason code.
        A cursor of ``known`` (the reader's own, kept since its last read) wins over the saved
        one, so a cursor that could not be saved never makes a line show twice.

        Args:
            thread_id: The thread of the run.
            run_id: The run (logged, and the cursor's run when it opens steps).
            known: The reader's cursors by source ID (copied, never changed).

        Returns:
            Per source of the thread: the record, the moved cursor and the events.
        """
        out = []
        for source in self.records():
            if source.thread_id != thread_id:
                continue
            mine = (known or {}).get(source.source_id)
            cursor = self.cursor(source) if mine is None else replace(mine, open=dict(mine.open))
            try:
                lines, notes = read_new(source.path, cursor)
            except OSError as exc:
                _LOG.warning(
                    "run %s: stage source %s not readable: %s",
                    run_id,
                    source.source_id,
                    type(exc).__name__,
                )
                continue
            events = []
            for line in lines:
                parsed = parse_line(line, source.ticket)
                if isinstance(parsed, str):
                    notes.append(parsed)
                else:
                    events.append(parsed)
            for note in notes:
                _LOG.info("run %s: stage source %s: %s", run_id, source.source_id, note)
            out.append((source, cursor, events))
        return out


def _parse_record(data: bytes, stem: str) -> StageSource | None:
    try:
        raw: Any = json.loads(data)
    except ValueError:
        return None
    if not isinstance(raw, dict) or raw.get("version") != RECORD_VERSION:
        return None
    source_id, thread, path = raw.get("source_id"), raw.get("thread_id"), raw.get("path")
    offset, ticket, at = raw.get("offset"), raw.get("ticket"), raw.get("registered_at")
    if not (isinstance(source_id, str) and source_id == stem and _SOURCE_ID.fullmatch(source_id)):
        return None
    if not (isinstance(path, str) and Path(path).is_absolute()):
        return None
    if not isinstance(offset, int) or isinstance(offset, bool) or offset < 0:
        return None
    if ticket is not None and not (isinstance(ticket, str) and _TICKET.fullmatch(ticket)):
        return None
    if not isinstance(at, int | float) or isinstance(at, bool):
        return None
    try:
        thread_id = uuid.UUID(str(thread))
    except ValueError:
        return None
    return StageSource(source_id, thread_id, path, offset, ticket, float(at))


def _parse_cursor(data: bytes) -> StageCursor | None:
    try:
        raw: Any = json.loads(data)
    except ValueError:
        return None
    if not isinstance(raw, dict) or raw.get("version") != RECORD_VERSION:
        return None
    offset, identity, run_id, opened = (
        raw.get(k) for k in ("offset", "identity", "run_id", "open")
    )
    if not isinstance(offset, int) or isinstance(offset, bool) or offset < 0:
        return None
    ident: tuple[int, int] | None = None
    if identity is not None:
        if not (isinstance(identity, list) and len(identity) == 2):
            return None
        if not all(isinstance(v, int) and not isinstance(v, bool) for v in identity):
            return None
        ident = (identity[0], identity[1])
    if run_id is not None and not isinstance(run_id, str):
        return None
    if not isinstance(opened, dict) or not all(
        isinstance(k, str) and isinstance(v, str) for k, v in opened.items()
    ):
        return None
    return StageCursor(offset, ident, raw.get("skipping") is True, run_id, dict(opened))


# ---- steps of the run ----------------------------------------------------------------------------


class StageSteps:
    """Turn the events of one source into steps and the status line of one run.

    Mapping (09 17.12 point 9): the chain runs one stage at a time and writes an ``ended`` line for
    each stage; a stage's ``started`` or ``pass`` first ends every open step of another stage
    ``completed``, so a lost end line leaves no step running. A stage's ``started`` opens a step
    ``workflow`` named ``chain_stage`` with the stage as its title; ``pass`` renames the open step
    of the stage to ``<stage> (pass <n>)``, or opens one so titled when the stage has none open;
    ``ended`` closes it ``failed`` for an outcome of `FAILED_OUTCOMES`, else ``completed``, with the
    title ``<stage>: <outcome>`` (an ``ended`` without an open step adds the closed step). ``run``
    ``started`` opens no step; ``run`` ``ended`` closes every open step of the source (``failed``
    after an end of `FAILED_OUTCOMES`, ``cancelled`` after one of `CANCELLED_ENDS`, else
    ``completed``). Every event sets ``/status_line`` to ``<ticket>: <text>``. Detail of a step:
    ``stage``, ``outcome`` and ``pass`` when known.

    Args:
        patcher: The run's patcher (``add_step``, ``update_step``, ``set_status_line``).
        now: The wall time for ``finished_at``.
        opened: The open step of each stage (the cursor's ``open``), changed in place.
    """

    def __init__(self, patcher: Any, now: Callable[[], datetime], opened: dict[str, str]) -> None:
        """Bind to the run's patcher and the source's open steps."""
        self._patch = patcher
        self._now = now
        self.open = opened

    def apply(self, event: StageEvent) -> bool:
        """Show one event; returns whether it ended the chain's run."""
        self._patch.set_status_line(f"{event.ticket}: {event.text}")
        if event.stage == RUN:
            if event.state != "ended":
                return False
            outcome = event.outcome or "unknown"
            status = (
                "failed"
                if outcome in FAILED_OUTCOMES
                else "cancelled"
                if outcome in CANCELLED_ENDS
                else "completed"
            )
            self.close_all(status)
            return True
        if event.state != "ended":
            for stage in [s for s in self.open if s != event.stage]:
                self._finish(self.open.pop(stage), "completed")
        step = self.open.get(event.stage)
        if event.state == "started":
            if step is not None:
                self._finish(step, "completed")
            self._add(event.stage, event.stage, {"stage": event.stage})
        elif event.state == "pass":
            title = f"{event.stage} (pass {event.pass_no})" if event.pass_no else event.stage
            detail = {"stage": event.stage} | ({"pass": event.pass_no} if event.pass_no else {})
            if step is None:
                self._add(event.stage, title, detail)
            else:
                self._patch.update_step(uuid.UUID(step), title=title, detail=detail)
        else:
            outcome = event.outcome or "unknown"
            status = "failed" if outcome in FAILED_OUTCOMES else "completed"
            title = f"{event.stage}: {outcome}"
            detail = {"stage": event.stage, "outcome": outcome}
            if step is None:
                added = self._patch.add_step(
                    STEP_KIND, STEP_NAME, title=title, status="running", detail=detail
                )
                self._finish(str(added), status)
            else:
                self._patch.update_step(uuid.UUID(step), title=title, detail=detail)
                self._finish(step, status)
            self.open.pop(event.stage, None)
        return False

    def close_all(self, status: str) -> None:
        """End every open step with ``status`` (the chain's end, or the run's end)."""
        for step in list(self.open.values()):
            self._finish(step, status)
        self.open.clear()

    def _add(self, stage: str, title: str, detail: dict[str, Any]) -> None:
        step = self._patch.add_step(
            STEP_KIND, STEP_NAME, title=title, status="running", detail=detail
        )
        self.open[stage] = str(step)

    def _finish(self, step: str, status: str) -> None:
        self._patch.update_step(uuid.UUID(step), status=status, finished_at=self._now())


# ---- command line --------------------------------------------------------------------------------


def build_parser(prog: str = "python -m bothy_hermes_adapter stages") -> argparse.ArgumentParser:
    """The parser of ``register``, ``deregister`` and ``list``."""
    parser = argparse.ArgumentParser(prog=prog)
    sub = parser.add_subparsers(dest="action", required=True)
    reg = sub.add_parser("register", help="follow a stage file in the thread's run")
    reg.add_argument("--thread", required=True, help="the thread ID (HERMES_SESSION_THREAD_ID)")
    reg.add_argument("--file", required=True, help="the absolute path of stages.jsonl")
    reg.add_argument("--ticket", help="only lines of this ticket key")
    dereg = sub.add_parser("deregister", help="stop following a stage file")
    dereg.add_argument("--source-id", required=True)
    sub.add_parser("list", help="print every registered stage file")
    for each in sub.choices.values():
        each.add_argument("--profile-home", type=Path, required=True, help="HERMES_HOME")
        each.add_argument("--platform-key", default="bothy")
    return parser


def _emit(value: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(value) + "\n")


def main(argv: Sequence[str] | None = None, prog: str | None = None) -> int:
    """Run one command and print its result as JSON lines.

    Returns:
        ``0`` on success, ``1`` when ``deregister`` found no record, ``2`` on a refusal.
    """
    args = build_parser(prog or "python -m bothy_hermes_adapter.stages").parse_args(argv)
    registry = StageRegistry(stages_dir(args.profile_home, args.platform_key))
    try:
        if args.action == "register":
            try:
                thread = uuid.UUID(args.thread)
            except ValueError:
                raise JobError("the thread is no thread ID") from None
            record = registry.register(thread, args.file, args.ticket)
            _emit({"ok": True, "source_id": record.source_id, "offset": record.offset})
            return 0
        if args.action == "deregister":
            found = registry.retire(args.source_id)
            _emit({"ok": found, "source_id": args.source_id})
            return 0 if found else 1
        for record in registry.records():
            _emit(
                {
                    "source_id": record.source_id,
                    "thread_id": str(record.thread_id),
                    "path": record.path,
                    "ticket": record.ticket,
                    "registered_at": record.registered_at,
                }
            )
        return 0
    except (JobError, OSError) as exc:
        message = str(exc) if isinstance(exc, JobError) else type(exc).__name__
        _emit({"ok": False, "error": message})
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
