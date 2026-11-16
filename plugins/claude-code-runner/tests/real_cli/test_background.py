# SPDX-License-Identifier: Apache-2.0
"""Background sessions of the pinned CLI, driven by the runner's own code (09 18.2, 18.4, 18.5).

Collected only with ``CLAUDE_CODE_RUNNER_REAL_CLI=1`` (``tests/conftest.py``); free: no login and
no model call. The CLI runs against the local stand-in for the API with a configuration folder
of its own (``background.py``), so its supervisor and its session list are separate from any
session of the developer, and every test stops and removes its sessions and waits until its
supervisor has ended.

What background mode rests on, each run here with the functions the runner uses:

1. The launch returns at once and prints the short id; the prompt is the command's stdin and
   reaches the model unchanged; the flags of the launch are honoured; the session list names
   the session's process and its status.
2. ``claude stop`` ends the process at once and the transcript then ends with the session's
   totals; ``claude rm`` removes the entry and keeps the transcript.
3. A resume starts a copy while the session has a process, and continues it in place with the
   launch's flags once its entry was removed; print mode continues a session a background
   process created.
4. A folder without workspace trust and bypass mode without a recorded acceptance are refused.
5. A line on the inbox socket is taken after the running tool call or starts a turn of its
   own, with its ``uuid`` kept; a refused request leaves an API error record and the process;
   a background command outlives its turn and two refused turns, and the turn of the runner's
   retry line carries what was left without an answer.
6. The members the CLI writes per transcript record equal the recorded table.
7. Right after the launch the entry has no process yet; ``claude stop`` and ``claude rm`` at
   that moment remove the entry, and no session takes the prompt afterwards.

Level two (``CLAUDE_CODE_RUNNER_REAL_TURN=1`` in addition, the owner's decision) is one short
background turn on the stored login of the host user: it proves the path of the turn and the
login beta the counter-check of 18.3 reads. It needs ``CLAUDE_CODE_RUNNER_REAL_BG_FOLDER``, a
folder that has the workspace trust of that user, takes the model from
``CLAUDE_CODE_RUNNER_REAL_BG_MODEL`` (default ``haiku``), refuses to start while a supervisor
runs, and stops and removes only the session it started.

The temporary folder must not lie below a ``CLAUDE.md`` or a ``.claude`` folder (point
``TMPDIR`` at a folder outside the home folder then), and it must be short: the inbox socket of
a session lies in it.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import shutil
import subprocess
import tempfile
import time
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

from real_cli.background import (
    MODEL,
    RETRY,
    TOOL_SECONDS,
    WAIT,
    Script,
    World,
    new_root,
    path_facts,
)
from real_cli.record import RECORDING
from real_cli.record_background import record_background
from real_cli.stand_in import cli_env, private_ancestors, stand_in, user_line
from real_cli.test_real_cli import claude_bin, turn

from claude_code_runner.account import AccountError, account_error
from claude_code_runner.bgmode import launch_environment
from claude_code_runner.bgsession import (
    Reading,
    SessionCli,
    SessionList,
    background_argv,
    launch,
    settings_document,
)
from claude_code_runner.bgtranscript import TranscriptMapper, TranscriptReader
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION
from claude_code_runner.processstate import Decision, ProcessState
from claude_code_runner.usage import claude_config_dir, find_transcript, transcript_totals

PROMPT = (
    "Context of the thread, first part.\n"
    "/compact is only a line of the context here\n"
    "Not ASCII in the context: café, naïve, señor, 日本語, €\n\n"
    "Fix the login form"
)
FOLLOWUP = "Follow-up from Sam (@sam):\n\nAlso the footer"
SETTLE = 6.0
"""Seconds after which a session that was going to start has sent its first request."""


def _base() -> Path:
    base = Path(tempfile.mkdtemp(prefix="ccr"))
    found = private_ancestors(base / "x" / "work")
    assert not found, f"the temporary folder lies below {found[0].parent}: set TMPDIR elsewhere"
    return base


@contextlib.asynccontextmanager
async def world() -> AsyncIterator[tuple[World, Script]]:
    """One isolated configuration folder against the stand-in; nothing of it is left running."""
    base = _base()
    script = Script()
    try:
        with stand_in(script) as (url, _):
            place = World(new_root(base), claude_bin(), url)
            try:
                yield place, script
            finally:
                await place.close()
    finally:
        shutil.rmtree(base, ignore_errors=True)


def gone(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    return False


def origin(record: dict[str, Any]) -> str | None:
    kind = (record.get("origin") or {}).get("kind")
    return kind if isinstance(kind, str) else None


def test_launch_list_stop_and_remove() -> None:
    async def scenario() -> None:
        async with world() as (place, script):
            extra = place.root / "incoming"
            extra.mkdir()
            out = await place.launch("post", PROMPT, add_dir=extra)
            # 1. The launch returns with the short id; no supervisor ran, so it started one.
            assert out.refusal is None
            assert out.short_id is not None
            assert out.started_service is True
            entry = await place.until_entry(out.short_id)
            assert entry.own
            assert entry.name == "bothy-post"
            assert entry.session_id.startswith(out.short_id)
            assert entry.pid is not None
            assert os.getpgid(entry.pid) == entry.pid  # it leads a process group of its own
            assert place.socket("post").is_socket()
            records = await place.until_turns(entry.session_id, 1)
            # The prompt reached the session and the model unchanged, a line with "/" included.
            [prompt] = [r for r in records if origin(r) == "human"]
            assert prompt["message"]["content"] == PROMPT
            assert PROMPT in script.last_user()
            # The path of the turn: a background session, on the helper's key (no login here).
            assert path_facts(records) == ({"cli"}, {"bg"}, False)
            # The flags of the launch were honoured.
            assert script.bodies[0]["model"] == MODEL
            assert "SYSTEM-MARKER of the test." in script.system()
            assert "Bash" in script.tools()
            assert "AskUserQuestion" not in script.tools()
            # The runner's reader over the live session: one turn, its answer, no background work.
            reader = TranscriptReader(
                lambda: place.transcript(entry.session_id),
                0.05,
                lambda: False,
                TranscriptMapper("prompt-uuid"),
                sessions=place.sessions,
                short_id=out.short_id,
                pid=entry.pid,
            )
            messages = [m async for m in reader.items() if m is not None]
            state = ProcessState(has_stdin=True)
            for message in messages:
                state.apply(message, 0.0)
            results = [m for m in messages if m["type"] == "result"]
            assert [(r["is_error"], r["result"]) for r in results] == [(False, "Answer 0")]
            assert not [m for m in messages if m.get("subtype") == "background_tasks_changed"]
            assert reader.status == "idle"
            decided = state.decide(
                0.0, stdin_open=True, pending=0, followup_grace=30.0, stop_grace=10.0
            )
            assert decided is Decision.CLOSE
            # 2. Stop: the process is gone at once, and the transcript ends with the totals.
            assert await place.cli.stop(out.short_id) is True
            deadline = time.monotonic() + 5.0
            while not gone(entry.pid) and time.monotonic() < deadline:
                await asyncio.sleep(0.02)
            assert gone(entry.pid)
            stopped = await place.until_entry(out.short_id, pid=False)
            assert stopped.has_process is False
            path = place.transcript(entry.session_id)
            assert path is not None
            assert place.records(entry.session_id)[-1]["type"] == "cost-state"
            totals = transcript_totals(path)
            assert totals is not None
            assert totals[MODEL].input_tokens == 10  # one request of the stand-in
            # Remove: the entry goes, the transcript stays.
            assert await place.cli.remove(out.short_id) is True
            assert await place.entry(out.short_id) is None
            assert path.is_file()
            assert await place.cli.remove(out.short_id) is False  # no such entry any more

    asyncio.run(scenario())


def test_stop_and_remove_right_after_the_launch_leave_no_session() -> None:
    # What a cancellation during the launch rests on (09 18.6): the runner stops and removes
    # the session of a launch at once, also while its entry names no process yet.
    async def scenario() -> None:
        async with world() as (place, script):
            out = await place.launch("early", "TOOLSTEP please")
            assert out.short_id is not None
            assert await place.cli.stop(out.short_id) is True
            assert await place.cli.remove(out.short_id) is True
            # Longer than a session needs for its first request (well under a second above).
            await asyncio.sleep(SETTLE)
            assert await place.entry(out.short_id) is None
            assert script.bodies == []  # the prompt reached no request

    asyncio.run(scenario())


def test_resume_copies_a_live_session_and_continues_a_removed_one_in_place() -> None:
    async def scenario() -> None:
        async with world() as (place, script):
            first = await place.launch("thread", "First message")
            assert first.short_id is not None
            entry = await place.until_entry(first.short_id)
            await place.until_turns(entry.session_id, 1)
            # 3. While the session has a process, a resume starts a copy and says so.
            copy = await place.launch("thread", "A copy?", resume=entry.session_id)
            assert copy.notes == 1
            assert copy.short_id not in (None, first.short_id)
            assert copy.short_id is not None
            await place.until_entry(copy.short_id, pid=False)
            await place.cli.stop(copy.short_id)
            await place.cli.remove(copy.short_id)
            # Stopped and removed, the same command continues the session under its own id.
            await place.stop(first.short_id)
            await place.cli.remove(first.short_id)
            again = await place.launch("thread", "Second message", resume=entry.session_id)
            assert (again.short_id, again.notes) == (first.short_id, 0)
            resumed = await place.until_entry(first.short_id)
            assert resumed.session_id == entry.session_id
            assert resumed.pid != entry.pid
            records = await place.until_turns(entry.session_id, 2)
            assert [r["message"]["content"] for r in records if origin(r) == "human"] == [
                "First message",
                "Second message",
            ]
            assert "Second message" in script.last_user()
            before = len(script.bodies[-1]["messages"])
            assert before == 3  # the conversation went on: two inputs and the first answer
            await place.stop(first.short_id)
            await place.cli.remove(first.short_id)
            # Print mode continues the session a background process created.
            cli = subprocess.run(
                [
                    place.program,
                    "-p",
                    "--input-format",
                    "stream-json",
                    "--output-format",
                    "stream-json",
                    "--verbose",
                    "--model",
                    MODEL,
                    "--resume",
                    entry.session_id,
                    "--permission-mode",
                    "dontAsk",
                    "--allowedTools",
                    "Bash",
                    "--permission-prompts",
                    "none",
                ],
                input=user_line("Third message")[1],
                capture_output=True,
                cwd=place.work,
                env=cli_env(place.root, script_url(place)),
                timeout=WAIT,
                check=False,
            )
            lines = [json.loads(x) for x in cli.stdout.decode().splitlines() if x.startswith("{")]
            [result] = [m for m in lines if m.get("type") == "result"]
            assert (result["is_error"], result["session_id"]) == (False, entry.session_id)
            assert len(script.bodies[-1]["messages"]) == 5

    asyncio.run(scenario())


def script_url(place: World) -> str:
    return place.env["ANTHROPIC_BASE_URL"]


def test_a_folder_without_trust_and_bypass_mode_without_acceptance_are_refused() -> None:
    async def scenario() -> None:
        async with world() as (place, _):
            other = place.root / "other"
            other.mkdir()
            # 4. Neither starts a session, and each names its cause.
            untrusted = await place.launch("untrusted", "Hello", cwd=other)
            assert (untrusted.short_id, untrusted.refusal) == (None, "untrusted")
            assert untrusted.exit_error is True
            bypass = await place.launch("bypass", "Hello", tool_class="host")
            assert (bypass.short_id, bypass.refusal) == (None, "bypass")
            reading = await place.sessions.read(after=place.sessions.now())
            assert isinstance(reading, Reading)
            assert reading.entries == ()
            # The documented manual entry satisfies the trust check.
            place.trust(other.resolve())
            trusted = await place.launch("trusted", "Hello", cwd=other)
            assert trusted.refusal is None
            assert trusted.short_id is not None
            await place.until_entry(trusted.short_id)

    asyncio.run(scenario())


def test_inbox_lines_refused_requests_and_background_work() -> None:
    async def scenario() -> None:
        async with world() as (place, script):
            out = await place.launch("job", "Context.\n\nBGJOB Run the suite")
            assert out.short_id is not None
            entry = await place.until_entry(out.short_id)
            session = entry.session_id
            await place.until_turns(session, 1)
            # 5. The background command outlives its turn: the list says busy without a turn.
            assert (await place.until_status(out.short_id, "busy")).pid == entry.pid
            # A refused request: an API error record with the member the runner reads.
            asked = await place.send("job", "REFUSE What is the state?")
            records = await place.until_turns(session, 2)
            [peer] = [r for r in records if origin(r) == "peer"]
            assert peer["uuid"] == asked  # the uuid of the line is kept
            [error] = [r for r in records if r.get("isApiErrorMessage")]
            assert error["error"] == "rate_limit"
            mapper = TranscriptMapper("prompt-uuid")
            mapped = [m for r in records for m in mapper.map(r)]
            [refused] = [m for m in mapped if m.get("is_api_error_message")]
            assert account_error(refused) == AccountError("usage_limit")
            assert [m["is_error"] for m in mapped if m["type"] == "result"] == [False, True]
            # The command's end starts a turn by itself, which is refused too; the process stays.
            records = await place.until_turns(session, 3)
            assert [origin(r) for r in records if origin(r)][-1] == "task-notification"
            after = await place.until_status(out.short_id, "idle")
            assert after.pid == entry.pid
            # The runner's retry line is answered, and its request carries what was left.
            await place.send("job", RETRY)
            records = await place.until_turns(session, 4)
            texts = " ".join(script.last_user())
            assert RETRY in texts
            assert "REFUSE What is the state?" in texts
            assert "<task-notification>" in texts
            mapper = TranscriptMapper("prompt-uuid")
            results = [m for r in records for m in mapper.map(r) if m["type"] == "result"]
            assert [r["is_error"] for r in results] == [False, True, True, False]
            # A line during a tool call is taken after it, into the same turn.
            await place.send("job", "TOOLSTEP One more step")
            step = f"sleep {TOOL_SECONDS}"
            await place.until_record(session, lambda r: step in json.dumps(r.get("message")))
            steered = await place.send("job", FOLLOWUP)
            records = await place.until_turns(session, 5)
            [taken] = [r for r in records if r.get("reason") == "absorbed_mid_turn"]
            assert taken["commandUuid"] == steered
            # The request after the tool call carries the line (as a note of the CLI).
            assert "Also the footer" in json.dumps(script.bodies[-1], ensure_ascii=False)
            assert sum(1 for r in records if r.get("subtype") == "turn_duration") == 5

    asyncio.run(scenario())


def test_the_recorded_transcript_fields_equal_a_fresh_run() -> None:
    base = _base()
    try:
        fresh = record_background(Path(claude_bin()), base, twins=False)
    finally:
        shutil.rmtree(base, ignore_errors=True)
    folder = RECORDING / f"background_{PINNED_CLAUDE_CODE_VERSION}"
    recorded = json.loads((folder / "transcript_fields.json").read_text("utf-8"))
    assert fresh["fields"] == recorded
    meta = json.loads((folder / "meta.json").read_text("utf-8"))
    assert fresh["after_turn"] == meta["after_turn"]
    for name in ("launch", "sessions"):
        cases = json.loads((folder / f"{name}.json").read_text("utf-8"))
        assert sorted(fresh[name]) == sorted(cases)


@turn
def test_real_background_turn_runs_on_the_stored_login() -> None:
    """Level two: one short background turn on the login of the host user.

    It starts a background session of that user, so it runs only on the owner's word. No
    session of anyone else is touched: the test refuses to start while a supervisor runs, and
    it stops and removes exactly the session it launched.
    """
    folder = Path(os.environ["CLAUDE_CODE_RUNNER_REAL_BG_FOLDER"]).resolve()
    model = os.environ.get("CLAUDE_CODE_RUNNER_REAL_BG_MODEL", "haiku")
    env = launch_environment({"HOME": os.environ["HOME"], "PATH": os.environ["PATH"]})
    config = claude_config_dir(env)
    assert config is not None
    short_base = os.environ.get("CLAUDE_CODE_RUNNER_SHORT_TMP") or "/tmp"

    async def scenario(files: Path) -> None:
        cli = SessionCli(claude_bin(), env, folder, WAIT)
        sessions = SessionList(cli, 0.5)
        readable, supervisor = await cli.supervisor()
        assert readable, "claude daemon status cannot be read"
        assert supervisor is None, "a supervisor runs: end the background sessions of this user"
        prompt_file = files / "system-prompt.md"
        prompt_file.write_text("Answer briefly.", encoding="utf-8")
        settings = files / "settings.json"
        settings.write_text(json.dumps(settings_document({})), encoding="utf-8")
        argv = background_argv(
            claude_bin=claude_bin(),
            model=model,
            resume=None,
            name=f"bothy-level-two-{files.name}",
            tool_class="chat",
            tools=[],
            add_dir=None,
            system_prompt_file=prompt_file,
            socket_file=files / "inbox.sock",
            settings_file=settings,
        )
        out = await launch(
            argv,
            prompt="Reply with one word.",
            cwd=folder,
            env=env,
            stdout=files / "launch.out",
            stderr=files / "launch.err",
            timeout=WAIT,
        )
        try:
            assert out.refusal is None, out.refusal
            assert out.short_id is not None
            deadline = time.monotonic() + WAIT
            records: list[dict[str, Any]] = []
            while time.monotonic() < deadline:
                reading = await sessions.read(after=sessions.now())
                entry = reading.entry(out.short_id) if reading is not None else None
                path = find_transcript(config, entry.session_id) if entry is not None else None
                if path is not None:
                    written = path.read_bytes()
                    lines = written[: written.rfind(b"\n") + 1].decode("utf-8").splitlines()
                    records = [json.loads(x) for x in lines if x.startswith("{")]
                if any(r.get("subtype") == "turn_duration" for r in records):
                    break
                await asyncio.sleep(0.5)
            # The turn ran as a background session and its request used the stored login.
            assert path_facts(records) == ({"cli"}, {"bg"}, True)
        finally:
            if out.short_id is not None:
                await cli.stop(out.short_id)
                await cli.remove(out.short_id)
            # The supervisor this launch started ends with its last session; it is never
            # signalled, because by now a session of the user may run under it.
            deadline = time.monotonic() + WAIT
            while time.monotonic() < deadline:
                _, left = await cli.supervisor()
                if left is None:
                    break
                await asyncio.sleep(0.5)

    with tempfile.TemporaryDirectory(prefix="ccr", dir=short_base) as files:
        asyncio.run(scenario(Path(files)))
