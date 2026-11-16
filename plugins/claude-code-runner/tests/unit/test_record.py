# SPDX-License-Identifier: Apache-2.0
"""The recording program of the pinned CLI, held without a CLI (``tests/real_cli/record.py``).

The program runs by hand at a change of the pin, against the real CLI. What keeps a recording
free of the recording host, what reads the message schemas from the CLI's file and what waits
for the end of a recorded turn is held here, in the suite that runs on every change.
"""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

import pytest
from real_cli import record
from real_cli.record import Cleaner, Cli
from real_cli.stand_in import private_ancestors

# Schemas in the form of the CLI's file: nested objects, a text with brackets and quotes, a
# quoted key, and two objects of one message type (the stored and the stream form).
CLI_FILE = (
    b'var A=p(()=>u({type:C("result"),subtype:C("success"),uuid:g(),session_id:o(),'
    b'result:o().describe("a } bracket, a \\" quote and (more)"),'
    b"usage:u({input_tokens:k(),output_tokens:k()}),"
    b'"quoted-key":o(),list:A(u({inner:o()})).optional(),'
    b'status:B(["completed","failed","stopped"])}));'
    b'var S=u({type:C("assistant"),message:m()});'
    b'var W=u({type:C("assistant"),message:m(),error:e(),uuid:g()});'
)


@pytest.fixture
def alex(monkeypatch: pytest.MonkeyPatch) -> None:
    """The recording host of these tests: the user ``alex`` with the home ``/home/alex``."""
    monkeypatch.setattr(record.getpass, "getuser", lambda: "alex")
    monkeypatch.setattr(Path, "home", classmethod(lambda cls: Path("/home/alex")))


@pytest.mark.usefixtures("alex")
def test_a_recorded_line_loses_every_path_of_the_recording_folder(tmp_path: Path) -> None:
    root = tmp_path / "tree"
    root.mkdir()
    flat = re.sub(r"[^A-Za-z0-9]", "-", str(root.resolve()))
    line = {
        "type": "assistant",
        "cwd": f"{root}/work",
        "file": f"{root.resolve()}/work/note.txt",
        "transcript": f"{root}/config/projects/{flat}-work/session.jsonl",
        "task": f"/private/tmp/claude-501/{flat}-work/session/tasks/b1.output",
    }
    assert json.loads(Cleaner(root).line(json.dumps(line))) == {
        "type": "assistant",
        "cwd": "/work",
        "file": "/work/note.txt",
        "transcript": "/record/config/projects/-work/session.jsonl",
        "task": "/tmp/claude/-work/session/tasks/b1.output",
    }


@pytest.mark.usefixtures("alex")
@pytest.mark.parametrize("leak", ["/home/alex/notes.md", "a folder of alex"])
def test_a_line_that_names_the_home_folder_or_the_user_is_refused(
    tmp_path: Path, leak: str
) -> None:
    clean = Cleaner(tmp_path)
    assert json.loads(clean.line(json.dumps({"type": "assistant", "text": "a note"})))
    with pytest.raises(RuntimeError, match="names the home folder or the user"):
        clean.line(json.dumps({"type": "assistant", "text": leak}))


@pytest.mark.usefixtures("alex")
def test_the_init_message_keeps_only_what_the_runner_reads(tmp_path: Path) -> None:
    init = {
        "type": "system",
        "subtype": "init",
        "session_id": "s",
        "model": "model-small",
        "apiKeySource": "none",
        "claude_code_version": "0.0.0",
        "tools": ["Bash"],
        "mcp_servers": [{"name": "of the recording host"}],
        "slash_commands": ["a-command-of-the-recording-host"],
        "plugins": [],
    }
    kept = json.loads(Cleaner(tmp_path).line(json.dumps(init)))
    assert kept == {k: init[k] for k in record._INIT_MEMBERS if k in init}
    assert not {"tools", "mcp_servers", "slash_commands", "plugins"} & set(kept)


def test_the_schema_reader_names_the_top_level_members_in_order() -> None:
    result = record._schema(CLI_FILE, "result/success")
    assert record._members(result) == [
        "type",
        "subtype",
        "uuid",
        "session_id",
        "result",
        "usage",
        "quoted-key",
        "list",
        "status",
    ]
    assert record._nested(result, "usage") == ["input_tokens", "output_tokens"]
    assert record._enum(result, "status") == ["completed", "failed", "stopped"]
    # A type without a subtype has several objects: the stream form has the most members.
    assert record._members(record._schema(CLI_FILE, "assistant")) == [
        "type",
        "message",
        "error",
        "uuid",
    ]
    with pytest.raises(ValueError, match="no schema of system/init"):
        record._schema(CLI_FILE, "system/init")


def test_written_members_are_named_per_message_and_per_workflow_agent() -> None:
    agent = {"type": "workflow_agent", "index": 0, "label": "writer", "state": "start"}
    phase = {"type": "workflow_phase", "title": "Build"}
    messages: list[dict[str, Any]] = [
        {
            "type": "system",
            "subtype": "task_progress",
            "uuid": "u",
            "session_id": "s",
            "task_id": "w1",
        },
        {"type": "system", "subtype": "task_progress", "workflow_progress": [phase, agent]},
        {"type": "result", "subtype": "success", "num_turns": 1},
        {"type": "tool_progress", "tool_name": "Bash"},
    ]
    written = record.written_members(messages)
    assert written["messages"] == {
        "system/task_progress": {"task_id", "workflow_progress"},
        "result/success": {"num_turns"},
        "tool_progress": {"tool_name"},
    }
    assert written["workflow_agent"] == ["index", "label", "state", "type"]


class _Process:
    """A process that ends after a number of polls (never, for ``None``)."""

    def __init__(self, ends_after: int | None) -> None:
        self.polls = 0
        self.ends_after = ends_after

    def poll(self) -> int | None:
        self.polls += 1
        return 0 if self.ends_after is not None and self.polls > self.ends_after else None


def _cli(tmp_path: Path, process: _Process, lines: list[dict[str, Any]]) -> Cli:
    cli = object.__new__(Cli)  # without starting a program
    cli.out = tmp_path / "stream.jsonl"
    cli.out.write_text("".join(json.dumps(line) + "\n" for line in lines), "utf-8")
    cli.process = process  # type: ignore[assignment]
    return cli


def _over(messages: list[dict[str, Any]]) -> bool:
    return any(m.get("type") == "result" for m in messages)


def test_the_wait_for_a_quiet_stream_ends_when_the_turn_is_over(tmp_path: Path) -> None:
    cli = _cli(tmp_path, _Process(None), [{"type": "assistant"}, {"type": "result"}])
    cli.until_quiet(_over, quiet=0.0, wait=30.0)


def test_the_wait_fails_when_the_cli_ended_before_the_turn_was_over(tmp_path: Path) -> None:
    """A CLI that ended early used to leave the recording program waiting without an end."""
    cli = _cli(tmp_path, _Process(0), [{"type": "assistant"}])
    with pytest.raises(RuntimeError, match="ended before the recorded turn was over"):
        cli.until_quiet(_over, quiet=0.0, wait=30.0)


def test_the_wait_is_bound(tmp_path: Path) -> None:
    cli = _cli(tmp_path, _Process(None), [{"type": "assistant"}])
    with pytest.raises(TimeoutError, match="did not come to rest"):
        cli.until_quiet(_over, quiet=0.0, wait=0.05)


def test_a_folder_below_instruction_files_is_no_place_for_a_recording(tmp_path: Path) -> None:
    work = tmp_path / "project" / "tmp" / "work"
    work.mkdir(parents=True)
    before = private_ancestors(work)
    planted = [tmp_path / "project" / "CLAUDE.md", tmp_path / ".claude"]
    planted[0].write_text("instructions of a developer\n", "utf-8")
    planted[1].mkdir()
    found = private_ancestors(work)
    assert [p for p in found if p not in before] == [p.resolve() for p in planted]


def test_a_background_recording_is_written_without_the_recording_host(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    root = tmp_path / "bg"
    root.mkdir()
    monkeypatch.setattr(record, "TESTS", tmp_path / "tests")
    monkeypatch.setattr(record, "RECORDING", tmp_path / "tests" / "recording")
    request = {
        "type": "api-request",
        "sessionId": "s",
        "params": {"model": "m", "betas": ["b"], "max_tokens": 1},
        "requestDigest": "d",
    }
    status = (
        "pid:     4711\nversion: 2.1.325\n"
        f"config:  {root}/bg12ab/config/daemon.json\n  sock dir:     /tmp/cc-daemon-501/0a1b2c3d\n"
    )
    recorded = {
        "transcripts": {"new_post": [request, {"type": "user", "cwd": f"{root}/bg12ab/work"}]},
        "streams": {"new_post": [json.dumps({"type": "result", "cwd": f"{root}/bg12ab/work"})]},
        "launch": {"untrusted": {"exit": 1, "stdout": "", "stderr": f"in {root}/bg12ab/other"}},
        "sessions": {"idle": {"id": "1a2b3c4d", "cwd": f"{root}/bg12ab/work", "status": "idle"}},
        "commands": {"daemon_status_running": {"exit": 0, "stdout": status, "stderr": ""}},
        "fields": {"records": {"user": ["cwd", "type"]}, "session_entry": ["id"]},
        "after_turn": {"new_post": ["idle"]},
    }
    record.write_background(recorded, Cleaner(root), "2026-11-16")
    [folder] = (tmp_path / "tests" / "recording").iterdir()
    assert folder.name == f"background_{record.PINNED_CLAUDE_CODE_VERSION}"
    written = {p.name: p.read_text() for p in folder.iterdir()}
    assert sorted(written) == [
        "commands.json",
        "launch.json",
        "meta.json",
        "sessions.json",
        "stream_new_post.jsonl",
        "transcript_fields.json",
        "transcript_new_post.jsonl",
    ]
    assert str(tmp_path) not in "".join(written.values())
    kept, user = map(json.loads, written["transcript_new_post.jsonl"].splitlines())
    assert kept == {
        "type": "api-request",
        "sessionId": "s",
        "params": {"model": "m", "betas": ["b"]},
    }
    assert user["cwd"] == "/work"
    assert json.loads(written["sessions.json"])["idle"]["cwd"] == "/work"
    assert json.loads(written["launch.json"])["untrusted"]["stderr"] == "in /record/bg12ab/other"
    stdout = json.loads(written["commands.json"])["daemon_status_running"]["stdout"]
    assert "/tmp/cc-daemon/dir" in stdout
    assert "cc-daemon-501" not in stdout
    meta = json.loads(written["meta.json"])
    assert meta["after_turn"] == {"new_post": ["idle"]}
    assert meta["cli_version"] == record.PINNED_CLAUDE_CODE_VERSION
