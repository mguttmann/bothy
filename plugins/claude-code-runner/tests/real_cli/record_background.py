# SPDX-License-Identifier: Apache-2.0
"""Record background sessions of the pinned CLI (09 18.2 "Background mode", 18.4).

``record.py`` calls `record_background` and writes what it returns to
``tests/fixtures/recording/background_<v>/``; the level-one test runs it again and compares the
field table. Free: the CLI runs against the stand-in with its own configuration folder
(``background.py``), no login and no model call.

Per scenario the same script of the stand-in runs twice: as a background session, whose
transcript is recorded, and as a print turn, whose stream is recorded. A test holds that both
give the same decisions of the stdin and stop rule
(``tests/unit/test_bgtranscript.py``). A recorded transcript keeps only the record types the
runner reads (`KEPT`, and of the attachments only ``queued_command``, which names whose input
a running turn took in); every other type is dropped, among them the three that hold account
identifiers (``session-owner``, the attachments ``session_context`` and ``credential_org``).

* ``new_post``: a prompt, one tool call, the answer, the release (``claude stop``).
* ``resumed``: the next message of the same session, continued in place after ``claude rm``.
* ``followup``: a line of the inbox socket during a tool call, and one between two turns.
* ``background``: a background command, a refused follow-up, the refused notification of the
  command's end, and the runner's retry line that is answered.
* ``stopped``: a stop during a tool call.
* ``login``: one turn of a session whose requests carry a login token, so its request record
  lists the beta the counter-check of 18.3 reads. The token is a dummy
  (``background.py``): no login, and no host but the stand-in is reached.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import subprocess
import time
import uuid
from collections.abc import Callable
from pathlib import Path
from typing import Any

from real_cli.background import MODEL, RETRY, WAIT, Script, World, new_root
from real_cli.stand_in import cli_env, stand_in, user_line

from claude_code_runner.bgsession import SessionEntry

__all__ = ["KEPT", "NOTIFIED", "PRINT_FLAGS", "QUEUED", "USAGE", "record_background"]

KEPT = (
    "user",
    "assistant",
    "api-request",
    "queue-operation",
    "attachment",
    "system",
    "cost-state",
)
"""The record types a recorded transcript keeps: those the runner reads. Of ``attachment`` only
the type `QUEUED` is kept, of ``system`` only ``turn_duration``."""

QUEUED = "queued_command"
"""The one attachment the runner reads: what a running turn took in, with its origin."""

NOTIFIED = "task-notification"
"""The one kind of origin of which the runner reads more than the kind: whether the
notification is about work the session itself launched (its ``producer``)."""

USAGE = {
    "input_tokens": 1200,
    "output_tokens": 40,
    "cache_read_input_tokens": 500,
    "cache_creation_input_tokens": 300,
}
"""The usage of every answer of the stand-in."""

PRINT_FLAGS = ("--permission-mode", "dontAsk", "--allowedTools", "Bash")
"""The class flags of the print twins: ``scoped`` with ``Bash``, as the sessions run."""

FOLLOWUP = "Follow-up from Sam (@sam):\n\nAlso the footer"
SECOND = "Follow-up from Sam (@sam):\n\nAnd the header"
REFUSED_QUESTION = "Follow-up from Sam (@sam):\n\nREFUSE What is the state?"


def _queued(record: dict[str, Any]) -> bool:
    inner = record.get("attachment")
    return isinstance(inner, dict) and inner.get("type") == QUEUED


def _kept(record: dict[str, Any]) -> bool:
    kind = record.get("type")
    if kind == "system":
        return record.get("subtype") == "turn_duration"
    if kind == "attachment":
        return _queued(record)
    return kind in KEPT


def _fields(records: list[dict[str, Any]], table: dict[str, set[str]]) -> None:
    for record in records:
        name = str(record.get("type"))
        if name == "system":
            name = f"system/{record.get('subtype')}"
        table.setdefault(name, set()).update(record)
        origin = record.get("origin") if name == "user" else None
        if name == "attachment" and _queued(record):
            table.setdefault(f"attachment/{QUEUED}", set()).update(record["attachment"])
            origin = record["attachment"].get("origin")
        if isinstance(origin, dict) and origin.get("kind") == NOTIFIED:
            table.setdefault(f"origin/{NOTIFIED}", set()).update(origin)


class _Recording:
    """What the scenarios produced, in the form of the recording folder."""

    def __init__(self) -> None:
        self.launch: dict[str, dict[str, Any]] = {}
        self.sessions: dict[str, Any] = {}
        self.commands: dict[str, dict[str, Any]] = {}
        self.transcripts: dict[str, list[dict[str, Any]]] = {}
        self.streams: dict[str, list[str]] = {}
        self.after_turn: dict[str, list[str]] = {}
        self.fields: dict[str, set[str]] = {}
        self.entry_members: set[str] = set()

    def launched(self, world: World, key: str, name: str, code: int) -> None:
        stdout, stderr = world.output(name)
        self.launch[key] = {"exit": code, "stdout": stdout, "stderr": stderr}

    async def listed(self, world: World, key: str, short_id: str) -> None:
        """Keep the raw entry of a session as the list shows it now."""
        code, out, _ = await world.cli.run("agents", "--json", "--all")
        entries = json.loads(out) if code == 0 else []
        entry = next((e for e in entries if e.get("id") == short_id), None)
        if entry is not None:
            self.sessions[key] = entry
            self.entry_members.update(entry)

    async def command(self, world: World, key: str, *args: str) -> None:
        code, out, err = await world.cli.run(*args)
        self.commands[key] = {"args": list(args), "exit": code, "stdout": out, "stderr": err}

    def transcript(self, key: str, records: list[dict[str, Any]], statuses: list[str]) -> None:
        _fields(records, self.fields)
        self.transcripts[key] = [r for r in records if _kept(r)]
        self.after_turn[key] = statuses


async def _status_after_turn(world: World, short_id: str) -> str:
    entry = await world.entry(short_id)
    return str(entry.status) if entry is not None else "gone"


async def _turn(world: World, session_id: str, short_id: str, count: int, seen: list[str]) -> None:
    """Wait for the turn end number ``count`` and note the status the list shows after it."""
    await world.until_turns(session_id, count)
    seen.append(await _status_after_turn(world, short_id))


async def _started(world: World, name: str, prompt: str, **more: Any) -> tuple[str, SessionEntry]:
    out = await world.launch(name, prompt, **more)
    assert out.short_id is not None, f"the launch of {name} was refused: {out.refusal}"
    return out.short_id, await world.until_entry(out.short_id)


def _tool_call(record: dict[str, Any]) -> bool:
    content = (record.get("message") or {}).get("content")
    return record.get("type") == "assistant" and any(
        isinstance(b, dict) and b.get("type") == "tool_use" for b in content or []
    )


async def _new_post(base: Path, program: str, rec: _Recording) -> None:
    script = Script()
    with stand_in(script, USAGE) as (url, _):
        world = World(new_root(base), program, url)
        try:
            await rec.command(world, "daemon_status_not_running", "daemon", "status")
            out = await world.launch("post", "Context of the thread.\n\nTOOLSTEP Fix the form")
            assert out.short_id is not None
            rec.launched(world, "new", "post", 0)
            entry = await world.until_entry(out.short_id)
            await rec.command(world, "daemon_status_running", "daemon", "status")
            await world.until_record(entry.session_id, _tool_call)
            await rec.listed(world, "busy", out.short_id)
            statuses: list[str] = []
            await _turn(world, entry.session_id, out.short_id, 1, statuses)
            await rec.listed(world, "idle", out.short_id)
            await rec.command(world, "stop", "stop", out.short_id)
            await world.stop(out.short_id)  # until the process wrote its totals and ended
            await rec.listed(world, "stopped", out.short_id)
            path = world.transcript(entry.session_id)
            assert path is not None
            first = path.stat().st_size
            rec.transcript("new_post", world.records(entry.session_id), statuses)
            await rec.command(world, "rm", "rm", out.short_id)
            # The next message: continued in place under the same id, with this launch's flags.
            again = await world.launch("post", "Add a test", resume=entry.session_id)
            assert again.short_id == out.short_id, "the resume did not continue in place"
            assert again.notes == 0, "the resume printed a note"
            rec.launched(world, "resumed_in_place", "post", 0)
            await world.until_entry(out.short_id)
            statuses = []
            await _turn(world, entry.session_id, out.short_id, 2, statuses)
            await world.stop(out.short_id)
            tail = path.read_bytes()[first:].decode("utf-8").splitlines()
            rec.transcript("resumed", [json.loads(x) for x in tail if x.startswith("{")], statuses)
            await rec.command(world, "stop_unknown", "stop", "00000000")
            await rec.command(world, "rm_unknown", "rm", "00000000")
        finally:
            await world.close()


async def _followup(base: Path, program: str, rec: _Recording) -> None:
    script = Script()
    with stand_in(script, USAGE) as (url, _):
        world = World(new_root(base), program, url)
        try:
            short_id, entry = await _started(world, "fup", "Context.\n\nTOOLSTEP Fix the form")
            await world.until_record(entry.session_id, _tool_call)
            await world.send("fup", FOLLOWUP)  # taken after the running tool call
            statuses: list[str] = []
            await _turn(world, entry.session_id, short_id, 1, statuses)
            await world.send("fup", SECOND)  # between two turns: a turn of its own
            await _turn(world, entry.session_id, short_id, 2, statuses)
            # A resume while the process lives starts a copy, which is stopped and removed.
            copy = await world.launch("fup", "A copy?", resume=entry.session_id)
            rec.launched(world, "copy_while_running", "fup", 0)
            if copy.short_id is not None and copy.short_id != short_id:
                await world.until_entry(copy.short_id, pid=False)
                await world.cli.stop(copy.short_id)
                await world.cli.remove(copy.short_id)
            await world.stop(short_id)
            rec.transcript("followup", world.records(entry.session_id), statuses)
        finally:
            await world.close()


async def _background(base: Path, program: str, rec: _Recording) -> None:
    script = Script()
    with stand_in(script, USAGE) as (url, _):
        world = World(new_root(base), program, url)
        try:
            short_id, entry = await _started(world, "job", "Context.\n\nBGJOB Run the suite")
            statuses: list[str] = []
            await _turn(world, entry.session_id, short_id, 1, statuses)
            await rec.listed(world, "busy_between_turns", short_id)
            await world.send("job", REFUSED_QUESTION)
            await _turn(world, entry.session_id, short_id, 2, statuses)
            await _turn(world, entry.session_id, short_id, 3, statuses)  # the notification
            await world.until_status(short_id, "idle")
            await world.send("job", RETRY)
            await _turn(world, entry.session_id, short_id, 4, statuses)
            await world.stop(short_id)
            rec.transcript("background", world.records(entry.session_id), statuses)
        finally:
            await world.close()


async def _stopped(base: Path, program: str, rec: _Recording) -> None:
    script = Script()
    with stand_in(script, USAGE) as (url, _):
        world = World(new_root(base), program, url)
        try:
            short_id, entry = await _started(world, "halt", "Context.\n\nTOOLSTEP Fix the form")
            await world.until_record(entry.session_id, _tool_call)
            await world.stop(short_id)
            rec.transcript("stopped", world.records(entry.session_id), [])
            # A resume of a session the CLI does not know: a launch line, an entry, no process.
            unknown = str(uuid.uuid4())
            missing = await world.launch("gone", "Anyone?", resume=unknown)
            rec.launched(world, "resume_unknown", "gone", 0)
            if missing.short_id is not None:
                await world.until_entry(missing.short_id, pid=False)
                await asyncio.sleep(2.0)
                await rec.listed(world, "failed", missing.short_id)
            # The refusals: a folder without workspace trust, bypass mode without acceptance.
            other = world.root / "other"
            other.mkdir()
            refused = await world.launch("untrusted", "Hello", cwd=other)
            rec.launched(world, "untrusted", "untrusted", 0 if refused.short_id else 1)
            refused = await world.launch("bypass", "Hello", tool_class="host")
            rec.launched(world, "bypass", "bypass", 0 if refused.short_id else 1)
            # A session that waits for a person, as no class of the runner lets one.
            waiting = await _waiting(world)
            if waiting is not None:
                await world.until_status(waiting, "waiting")
                await rec.listed(world, "waiting", waiting)
        finally:
            await world.close()


async def _login(base: Path, program: str, rec: _Recording) -> None:
    script = Script()
    with stand_in(script, USAGE) as (url, _):
        world = World(new_root(base), program, url, login_token=True)
        try:
            short_id, entry = await _started(world, "login", "Context.\n\nFix the form")
            statuses: list[str] = []
            await _turn(world, entry.session_id, short_id, 1, statuses)
            await world.stop(short_id)
            rec.transcript("login", world.records(entry.session_id), statuses)
        finally:
            await world.close()


async def _waiting(world: World) -> str | None:
    """Launch a session in the default permission mode that asks a question; its short id."""
    proc = await asyncio.create_subprocess_exec(
        world.program,
        "--bg",
        "--name",
        "bothy-wait",
        "--model",
        MODEL,
        "--permission-mode",
        "default",
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd=world.work,
        env=world.env,
    )
    out, _ = await asyncio.wait_for(proc.communicate(b"ASK me"), WAIT)
    parts = out.decode("utf-8").split(" · ")
    if proc.returncode != 0 or len(parts) < 2:
        return None
    world.started.append(parts[1])
    return parts[1]


# ---- the print twins ---------------------------------------------------------------------------


class _Print:
    """One ``claude -p`` process with stream-json on both sides, as the runner starts a turn."""

    def __init__(self, program: str, root: Path, url: str, session: list[str]) -> None:
        self.out = root / f"stream-{uuid.uuid4().hex[:6]}.jsonl"
        (root / "work").mkdir(exist_ok=True)
        argv = [
            program,
            "-p",
            "--input-format",
            "stream-json",
            "--output-format",
            "stream-json",
            "--verbose",
            "--replay-user-messages",
            "--model",
            MODEL,
            *session,
            *PRINT_FLAGS,
            "--permission-prompts",
            "none",
        ]
        with self.out.open("wb") as stdout:
            self.process = subprocess.Popen(
                argv,
                cwd=root / "work",
                env=cli_env(root, url),
                stdin=subprocess.PIPE,
                stdout=stdout,
                stderr=subprocess.DEVNULL,
            )

    def write(self, value: str) -> None:
        assert self.process.stdin is not None
        self.process.stdin.write(user_line(value)[1])
        self.process.stdin.flush()

    def lines(self) -> list[str]:
        written = self.out.read_bytes()
        text = written[: written.rfind(b"\n") + 1].decode("utf-8")
        return [x for x in text.splitlines() if x.startswith("{")]

    def until(self, done: Callable[[list[dict[str, Any]]], bool]) -> None:
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            if done([json.loads(x) for x in self.lines()]):
                return
            if self.process.poll() is not None:
                raise RuntimeError("the CLI ended before the recorded turn was over")
            time.sleep(0.1)
        raise TimeoutError("the print twin did not get there")

    def results(self, count: int) -> None:
        self.until(lambda ms: sum(1 for m in ms if m.get("type") == "result") >= count)

    def close(self) -> list[str]:
        assert self.process.stdin is not None
        with contextlib.suppress(BrokenPipeError):
            self.process.stdin.close()
        self.process.wait(timeout=WAIT)
        return self.lines()


def _tool_started(messages: list[dict[str, Any]]) -> bool:
    return any(m.get("type") == "assistant" and _tool_call(m) for m in messages)


def _print_twins(base: Path, program: str, rec: _Recording) -> None:
    session = str(uuid.uuid4())
    root = new_root(base)
    with stand_in(Script(), USAGE) as (url, _):
        cli = _Print(program, root, url, ["--session-id", session])
        cli.write("Context of the thread.\n\nTOOLSTEP Fix the form")
        cli.results(1)
        rec.streams["new_post"] = cli.close()
        cli = _Print(program, root, url, ["--resume", session])
        cli.write("Add a test")
        cli.results(1)
        rec.streams["resumed"] = cli.close()
    root = new_root(base)
    with stand_in(Script(), USAGE) as (url, _):
        cli = _Print(program, root, url, ["--session-id", str(uuid.uuid4())])
        cli.write("Context.\n\nTOOLSTEP Fix the form")
        cli.until(_tool_started)
        cli.write(FOLLOWUP)
        cli.results(1)
        cli.write(SECOND)
        cli.results(2)
        rec.streams["followup"] = cli.close()
    root = new_root(base)
    with stand_in(Script(), USAGE) as (url, _):
        cli = _Print(program, root, url, ["--session-id", str(uuid.uuid4())])
        cli.write("Context.\n\nBGJOB Run the suite")
        cli.results(1)
        cli.write(REFUSED_QUESTION)
        cli.results(3)  # the refused follow-up, then the refused notification
        cli.write(RETRY)
        cli.results(4)
        rec.streams["background"] = cli.close()


async def _scenarios(base: Path, program: str, rec: _Recording) -> None:
    await _new_post(base, program, rec)
    await _followup(base, program, rec)
    await _background(base, program, rec)
    await _stopped(base, program, rec)
    await _login(base, program, rec)


def record_background(program: Path, base: Path, *, twins: bool = True) -> dict[str, Any]:
    """Run every scenario against the stand-in and return what was recorded.

    Args:
        program: The pinned CLI.
        base: A short folder without a ``CLAUDE.md`` or ``.claude`` above it.
        twins: Also record the print twins (the level-one test compares only the fields).

    Returns:
        ``launch`` (what the launch command printed per case), ``sessions`` (an entry of the
        session list per state), ``commands`` (what ``stop``, ``rm`` and ``daemon status``
        printed), ``transcripts`` (the kept records per scenario), ``after_turn`` (the status
        the list showed after each turn end), ``streams`` (the print twins) and ``fields``
        (the members the CLI wrote per record type, and of a list entry).
    """
    rec = _Recording()
    asyncio.run(_scenarios(base, str(program), rec))
    if twins:
        _print_twins(base, str(program), rec)
    return {
        "launch": rec.launch,
        "sessions": rec.sessions,
        "commands": rec.commands,
        "transcripts": rec.transcripts,
        "after_turn": rec.after_turn,
        "streams": rec.streams,
        "fields": {
            "records": {name: sorted(members) for name, members in sorted(rec.fields.items())},
            "session_entry": sorted(rec.entry_members),
        },
    }
