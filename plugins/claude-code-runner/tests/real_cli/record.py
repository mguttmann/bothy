# SPDX-License-Identifier: Apache-2.0
"""Record the fixtures of the pinned CLI again (a change of the pin, design notes 15 and 17).

From the directory of the runner, with the pinned CLI installed into a temporary folder (the npm
package of the platform at the pinned version, no login)::

    CLAUDE_CODE_RUNNER_CLAUDE_BIN=<pinned claude> \
        uv run --locked python tests/real_cli/record.py

Free: no login, no credential and no model call. The CLI runs against the local stand-in for
the API on loopback (``stand_in.py``) with an empty ``HOME`` and its own ``CLAUDE_CONFIG_DIR``,
in a temporary working folder that has no ``CLAUDE.md`` and no ``.claude`` above it. The program
refuses a CLI whose version is not `PINNED_CLAUDE_CODE_VERSION`: move the pin first, then record.

What it writes (``<v>`` is the pinned version), and what reads it:

* ``tests/fixtures/recording/stream_fields_<v>.json``: the members of every stream message the
  runner reads, from the message schemas in the CLI's file, plus members the CLI wrote in the
  recorded turn that no schema names (``test_livebox_graph.py``; the level-one test compares it
  with the installed CLI).
* ``tests/fixtures/recording/agent_tree_<v>.jsonl``: one turn with a workflow of three agents, a
  background and a foreground shell command, the foreground commands of the workflow agents
  and an Agent tool subagent with its own tool calls (``test_livebox_tree.py``).
* ``tests/fixtures/recording/resumed_session_<v>/``: the transcript of a session after its first
  turn and the stream of its resumed second turn (``test_usage.py``, ``test_resume_cost.py``).
* ``tests/real_cli/captured/api_errors_<v>.jsonl``: the ``assistant`` and ``result`` lines of a
  prompt above the context window (400), a refused key (401, which the CLI retries for about
  three minutes) and a CLI below the model's minimum (400) (``test_failures.py``).

The recordings of an earlier pin are removed by hand in the same change. Every answer of the
stand-in is written in this file, so a recording holds no text of a person; paths of the
temporary folder are replaced, and the program refuses to write a line that still names the
home folder or the user.
"""

from __future__ import annotations

import argparse
import contextlib
import getpass
import json
import os
import re
import subprocess
import sys
import tempfile
import time
import uuid
from collections.abc import Callable, Iterator, Mapping
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from real_cli.stand_in import (
    Answer,
    ApiError,
    Script,
    cli_env,
    private_ancestors,
    stand_in,
    text,
    user_line,
)

from claude_code_runner.environment import clean_path, find_program
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION, parse_version
from claude_code_runner.livebox import WORKFLOW_TASK_TYPE

TESTS = Path(__file__).resolve().parents[1]
RECORDING = TESTS / "fixtures" / "recording"
CAPTURED = TESTS / "real_cli" / "captured"
WAIT = 600.0
"""The bound of every wait (seconds); the refused key alone takes about three minutes."""
QUIET = 6.0
"""A turn with background work is over after this many seconds without a stream line."""
MODEL = "claude-sonnet-4-5"
SESSION = "0192d3a0-0000-4000-8000-0000000000b2"
"""The invented session reference of the recordings (the scenarios need a UUID)."""
USAGE = {
    "input_tokens": 1200,
    "output_tokens": 40,
    "cache_read_input_tokens": 500,
    "cache_creation_input_tokens": 300,
}
"""The usage of every answer of the stand-in (the values of RUNNER-RESUME-COST)."""
_WRAPPER_MAX_BYTES = 1 << 20  # a launcher script, not the program; read its package instead
_INIT_MEMBERS = (
    "type",
    "subtype",
    "cwd",
    "session_id",
    "model",
    "apiKeySource",
    "claude_code_version",
    "output_style",
    "permissionMode",
)
"""What a recording keeps of ``system/init``: the members the runner reads and the version; the
rest lists the tools, commands and folders of the recording host."""


# ---- the CLI's file: message schemas --------------------------------------------------------

SCHEMAS = (
    "system/init",
    "assistant",
    "user",
    "tool_progress",
    "system/task_started",
    "system/task_progress",
    "system/task_updated",
    "system/task_notification",
    "system/background_tasks_changed",
    "system/status",
    "system/api_retry",
    "command_lifecycle",
    "result/success",
)
"""The stream messages of 09 18.4, as ``<type>`` or ``<type>/<subtype>``."""
_ENVELOPE = frozenset({"type", "subtype", "uuid", "session_id"})
_IDENT = rb"[A-Za-z_$][\w$.]*"
_KEY = re.compile(rb'([A-Za-z_$][\w$]*|"[^"]+"):')
_QUOTES = (b'"', b"'", b"`")
_SCHEMA_MAX_BYTES = 1 << 18  # no message schema is longer; a match in other bytes ends here


def cli_bytes(program: Path) -> bytes:
    """Return the bytes of the CLI: the program, or the package of a launcher script."""
    if program.stat().st_size > _WRAPPER_MAX_BYTES:
        return program.read_bytes()
    package = program.parent.parent if program.parent.name == "bin" else program.parent
    return b"\n".join(p.read_bytes() for p in sorted(package.rglob("*")) if p.is_file())


def _object_at(data: bytes, start: int) -> bytes:
    """Return the object literal that opens at ``data[start]``, strings respected."""
    depth = 0
    quote: bytes | None = None
    index = start
    while index < min(len(data), start + _SCHEMA_MAX_BYTES):
        char = data[index : index + 1]
        if quote is not None:
            if char == b"\\":
                index += 1
            elif char == quote:
                quote = None
        elif char in _QUOTES:
            quote = char
        elif char in (b"{", b"(", b"["):
            depth += 1
        elif char in (b"}", b")", b"]"):
            depth -= 1
            if depth == 0:
                return data[start : index + 1]
        index += 1
    raise ValueError("unbalanced object literal")


def _members(obj: bytes) -> list[str]:
    """Return the member names of the top level of an object literal, in order."""
    names: list[str] = []
    depth = 0
    quote: bytes | None = None
    index = 0
    at_key = False
    while index < len(obj):
        char = obj[index : index + 1]
        key = _KEY.match(obj, index) if depth == 1 and at_key and quote is None else None
        if quote is not None:
            if char == b"\\":
                index += 1
            elif char == quote:
                quote = None
        elif key is not None:
            names.append(key.group(1).decode().strip('"'))
            at_key = False
            index = key.end() - 1
        elif char in _QUOTES:
            quote = char
        elif char in (b"{", b"(", b"["):
            depth += 1
            at_key = depth == 1
        elif char in (b"}", b")", b"]"):
            depth -= 1
        elif char == b"," and depth == 1:
            at_key = True
        index += 1
    return names


def _schema(data: bytes, name: str) -> bytes:
    """Return the schema object of one stream message from the CLI's file."""
    kind, _, subtype = name.partition("/")
    pattern = rb"\{type:" + _IDENT + rb'\("' + kind.encode() + rb'"\),'
    if subtype:
        pattern += rb"subtype:" + _IDENT + rb'\("' + subtype.encode() + rb'"\)'
    found = set()
    for match in re.finditer(pattern, data):
        with contextlib.suppress(ValueError):  # the same bytes outside a schema
            found.add(_object_at(data, match.start()))
    if not found:
        raise ValueError(f"no schema of {name} in the CLI's file")
    # A message type without a subtype has several objects (the stored and the stream form):
    # the stream form is the one with the most members.
    return max(found, key=lambda obj: len(_members(obj)))


def _nested(obj: bytes, member: str) -> list[str]:
    """Return the member names of the first object literal after ``<member>:``."""
    at = obj.index(member.encode() + b":")
    return _members(_object_at(obj, obj.index(b"{", at)))


def _enum(obj: bytes, member: str) -> list[str]:
    """Return the values of the string list after ``<member>:``."""
    found = re.search(member.encode() + rb":" + _IDENT + rb"\(\[([^\]]*)\]", obj)
    if found is None:
        raise ValueError(f"no value list of {member}")
    return [v.decode() for v in re.findall(rb'"([^"]+)"', found.group(1))]


def read_fields(data: bytes) -> dict[str, Any]:
    """Read the stream field table from the bytes of the CLI.

    Args:
        data: The bytes of the CLI (`cli_bytes`).

    Returns:
        The member names per message without the envelope (``type``, ``subtype``, ``uuid``,
        ``session_id``), the members of a background task entry and of a task's ``usage``,
        the value lists of the task status and the command state, and the task types.
    """
    schemas = {name: _schema(data, name) for name in SCHEMAS}
    prefixes = re.search(rb'\{local_bash:"\w"(?:,[a-z_]+:"\w")+\}', data)
    if prefixes is None:
        raise ValueError("no task type table in the CLI's file")
    task_types = sorted(_members(prefixes.group(0)))
    if WORKFLOW_TASK_TYPE not in task_types:
        raise ValueError(f"the CLI names no task type {WORKFLOW_TASK_TYPE}")
    return {
        "messages": {
            name: [m for m in _members(obj) if m not in _ENVELOPE] for name, obj in schemas.items()
        },
        "background_task": _nested(schemas["system/background_tasks_changed"], "tasks"),
        "usage": _nested(schemas["system/task_progress"], "usage"),
        "task_status": _enum(schemas["system/task_notification"], "status"),
        "command_states": _enum(schemas["command_lifecycle"], "state"),
        "task_types": task_types,
        "workflow_task_type": WORKFLOW_TASK_TYPE,
    }


def written_members(messages: list[dict[str, Any]]) -> dict[str, Any]:
    """Return what the CLI wrote in a recorded turn: members per message and per agent entry."""
    by_message: dict[str, set[str]] = {}
    agent: set[str] = set()
    for message in messages:
        name = str(message.get("type"))
        if isinstance(message.get("subtype"), str):
            name += "/" + message["subtype"]
        by_message.setdefault(name, set()).update(set(message) - _ENVELOPE)
        for entry in message.get("workflow_progress") or []:
            if isinstance(entry, dict) and entry.get("type") == "workflow_agent":
                agent.update(entry)
    return {"messages": by_message, "workflow_agent": sorted(agent)}


def fields_document(data: bytes, tree: list[dict[str, Any]], today: str) -> dict[str, Any]:
    """Build ``stream_fields_<v>.json`` from the CLI's file and the recorded turn."""
    fields = read_fields(data)
    written = written_members(tree)
    for name, members in fields["messages"].items():
        members.extend(sorted(written["messages"].get(name, set()) - set(members)))
    return {
        "cli_version": PINNED_CLAUDE_CODE_VERSION,
        "origin": (
            "read from the file of the pinned CLI (the npm package of the platform; no session, "
            "no login, no model call) and from the recorded turn "
            f"agent_tree_{PINNED_CLAUDE_CODE_VERSION}.jsonl by tests/real_cli/record.py on {today}"
        ),
        "note": (
            "Message types, subtypes and member names only, no content. Each list of messages "
            "names the members of the message schema in the CLI's file without the envelope "
            "(type, subtype, uuid, session_id), then the members the CLI wrote in the recorded "
            "turn that the schema does not name. workflow_agent lists what the CLI wrote in the "
            "agent entries of workflow_progress."
        ),
        **fields,
        "workflow_agent": written["workflow_agent"],
    }


# ---- one CLI process against the stand-in ---------------------------------------------------


class Cli:
    """One ``claude -p`` process with stream-json on both sides, as the runner starts it."""

    def __init__(self, program: Path, root: Path, url: str, flags: list[str]) -> None:
        self.root = root
        self.work = root / "work"
        self.work.mkdir(parents=True, exist_ok=True)
        self.out = root / f"stream-{uuid.uuid4().hex[:8]}.jsonl"
        self.err = self.out.with_suffix(".stderr")
        argv = [
            str(program),
            "-p",
            "--input-format",
            "stream-json",
            "--output-format",
            "stream-json",
            "--verbose",
            "--replay-user-messages",
            "--model",
            MODEL,
            *flags,
            "--permission-prompts",
            "none",
        ]
        with self.out.open("wb") as stdout, self.err.open("wb") as stderr:
            self.process = subprocess.Popen(
                argv,
                cwd=self.work,
                env=cli_env(root, url),
                stdin=subprocess.PIPE,
                stdout=stdout,
                stderr=stderr,
            )

    def write(self, value: str) -> None:
        assert self.process.stdin is not None
        self.process.stdin.write(user_line(value)[1])
        self.process.stdin.flush()

    def lines(self) -> list[str]:
        return [x for x in self.out.read_text("utf-8").splitlines() if x.startswith("{")]

    def messages(self) -> list[dict[str, Any]]:
        return [json.loads(x) for x in self.lines()]

    def until(self, done: Callable[[list[dict[str, Any]]], bool]) -> None:
        """Wait until ``done`` holds for the messages so far, or the process has ended."""
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            if done(self.messages()) or self.process.poll() is not None:
                return
            time.sleep(0.2)
        raise TimeoutError(f"the CLI did not get there; stream: {self.out.read_text()[-2000:]}")

    def close(self) -> int:
        """End the input and return the exit code."""
        assert self.process.stdin is not None
        with contextlib.suppress(BrokenPipeError):
            self.process.stdin.close()
        return self.process.wait(timeout=WAIT)


def _results(messages: list[dict[str, Any]]) -> int:
    return sum(1 for m in messages if m.get("type") == "result")


def _one_turn(program: Path, root: Path, script: Script, flags: list[str], prompt: str) -> Cli:
    with stand_in(script, USAGE) as (url, _):
        cli = Cli(program, root, url, flags)
        try:
            cli.write(prompt)
            cli.until(lambda messages: _results(messages) >= 1)
        finally:
            cli.close()
    return cli


# ---- the recordings -------------------------------------------------------------------------


def api_error_cases(version: str) -> list[tuple[str, ApiError]]:
    """The answers of the API the runner recognises (09 18.9), as the stand-in sends them."""
    major, minor, patch = parse_version(version) or (0, 0, 0)
    too_old = (
        f"Claude Code {version} does not support this model; "
        f"version {major}.{minor}.{patch + 1} or newer is required"
    )
    return [
        (
            "a prompt above the context window",
            ApiError(
                400,
                "invalid_request_error",
                "prompt is too long: 250000 tokens > 200000 maximum",
            ),
        ),
        ("a refused key", ApiError(401, "authentication_error", "invalid x-api-key")),
        (
            "a CLI below the model's minimum",
            ApiError(
                400,
                "invalid_request_error",
                too_old,
                {"error_code": "claude_code_version_too_old"},
            ),
        ),
    ]


def record_api_errors(program: Path, root: Path) -> list[str]:
    """Return the ``assistant`` and ``result`` lines the CLI wrote for each refused request."""
    flags = ["--permission-mode", "dontAsk", "--tools", ""]
    lines: list[str] = []
    for index, (label, error) in enumerate(api_error_cases(PINNED_CLAUDE_CODE_VERSION)):
        session = ["--session-id", str(uuid.uuid4())]
        cli = _one_turn(
            program, root / f"error{index}", lambda _, e=error: e, session + flags, "Hello."
        )
        kept = [x for x in cli.lines() if json.loads(x).get("type") in ("assistant", "result")]
        if len(kept) != 2:
            raise RuntimeError(f"{label}: expected one assistant and one result line")
        lines += kept
        print(f"  {label}: {json.loads(kept[1]).get('result')!r}"[:160])
    return lines


def record_resumed_session(program: Path, root: Path) -> tuple[list[str], list[str]]:
    """Return the transcript after turn one and the stream of the resumed turn two."""
    flags = ["--permission-mode", "dontAsk", "--tools", ""]
    session = str(uuid.uuid4())

    def answer(_: Mapping[str, Any]) -> Answer:
        return [text("Done.")]

    _one_turn(program, root, answer, ["--session-id", session, *flags], "first")
    (path,) = (root / "config" / "projects").glob(f"*/{session}.jsonl")
    kept = ("user", "assistant", "last-prompt", "cost-state")
    transcript = [
        x for x in path.read_text("utf-8").splitlines() if json.loads(x).get("type") in kept
    ]
    resumed = _one_turn(program, root, answer, ["--resume", session, *flags], "second")
    return (
        [x.replace(session, SESSION) for x in transcript],
        [x.replace(session, SESSION) for x in resumed.lines()],
    )


WORKFLOW = """export const meta = {
  name: 'record-tree',
  description: 'Three agents of one phase',
  phases: [{ title: 'Build' }],
}
const labels = ['writer', 'checker', 'review']
const reports = await parallel(labels.map(label => () =>
  agent('WORKFLOW-AGENT ' + label + ': run one command and report.', { label, phase: 'Build' })
))
return { reports }
"""
"""The workflow of the recorded turn: three agents of one phase, each with one command."""


def tree_script(work: Path) -> Script:
    """Return the answers of the recorded turn: the main thread, a subagent, three agents."""
    count = iter(range(1, 1000))

    def tool(name: str, **tool_input: Any) -> dict[str, Any]:
        return {
            "type": "tool_use",
            "id": f"toolu_{next(count):02d}",
            "name": name,
            "input": tool_input,
        }

    def shell(command: str, description: str, **more: Any) -> dict[str, Any]:
        return tool("Bash", command=command, description=description, **more)

    def main_thread(step: int) -> Answer:
        plan: list[Callable[[], Answer]] = [
            lambda: [text("Starting."), shell("echo one", "Print one")],
            lambda: [shell("sleep 8; echo late", "Background job", run_in_background=True)],
            lambda: [shell("sleep 4; echo slow", "Slow foreground command")],
            lambda: [
                tool(
                    "Agent",
                    description="Read the note",
                    prompt="SUBAGENT: run one command, read the note and report.",
                    subagent_type="general-purpose",
                    run_in_background=False,
                )
            ],
            lambda: [tool("Workflow", script=WORKFLOW)],
            lambda: [text("The workflow runs in the background.")],
        ]
        return plan[step]() if step < len(plan) else [text("Noted.")]

    def subagent(step: int) -> Answer:
        plan: list[Callable[[], Answer]] = [
            lambda: [shell("echo sub", "Subagent command")],
            lambda: [tool("Read", file_path=str(work / "note.txt"))],
        ]
        return plan[step]() if step < len(plan) else [text("Subagent report.")]

    def script(body: Mapping[str, Any]) -> Answer:
        messages = body.get("messages") or []
        step = sum(1 for m in messages if m.get("role") == "assistant")
        prompt = json.dumps(messages[:1])
        if "WORKFLOW-AGENT" in prompt:
            return [shell("sleep 3; echo agent", "Agent command")] if step == 0 else [text("Ok.")]
        if "SUBAGENT" in prompt:
            return subagent(step)
        if not body.get("tools"):
            return [text("Ok.")]  # a request of the CLI itself, not of the conversation
        return main_thread(step)

    return script


def _tree_over(messages: list[dict[str, Any]]) -> bool:
    """The turn and its background work are over: no task runs and no turn is open."""
    lists = [m for m in messages if m.get("subtype") == "background_tasks_changed"]
    inits = sum(1 for m in messages if m.get("subtype") == "init")
    workflow_ended = any(
        m.get("subtype") == "task_notification" and str(m.get("task_id", "")).startswith("w")
        for m in messages
    )
    idle = bool(lists) and not lists[-1].get("tasks") and _results(messages) >= inits > 0
    return idle and workflow_ended


def record_agent_tree(program: Path, root: Path) -> list[str]:
    """Return the stream of one turn with a workflow, a subagent and shell tasks."""
    flags = ["--session-id", str(uuid.uuid4()), "--permission-mode", "bypassPermissions"]
    (root / "work").mkdir()
    (root / "work" / "note.txt").write_text("A note.\n", encoding="utf-8")
    with stand_in(tree_script(root / "work"), USAGE) as (url, _):
        cli = Cli(program, root, url, flags)
        try:
            cli.write("Use a workflow: run the plan of the recording.")
            cli.until(_tree_over)
            # The CLI starts a turn of its own for the last notification: wait until it is quiet.
            seen = -1
            while seen != len(cli.lines()) or not _tree_over(cli.messages()):
                seen = len(cli.lines())
                time.sleep(QUIET)
        finally:
            code = cli.close()
    if code != 0:
        raise RuntimeError(f"the recorded turn ended with exit code {code}")
    return [x.replace(flags[1], SESSION) for x in cli.lines()]


# ---- cleaning and writing -------------------------------------------------------------------


class Cleaner:
    """Replaces what names the recording host: the temporary folder and the CLI's task folder."""

    def __init__(self, root: Path) -> None:
        forms = {str(root), str(root.resolve())}
        self._forms = sorted(forms, key=len, reverse=True)
        self._flat = sorted({re.sub(r"[^A-Za-z0-9]", "-", f) for f in forms}, key=len, reverse=True)
        self._forbidden = [str(Path.home()), getpass.getuser()]

    def line(self, raw: str) -> str:
        """Return one recorded line without the paths of the recording host."""
        doc = json.loads(raw)
        if doc.get("type") == "system" and doc.get("subtype") == "init":
            doc = {k: doc[k] for k in _INIT_MEMBERS if k in doc}
        out = json.dumps(doc, separators=(",", ":"))
        for form in self._forms:
            out = re.sub(re.escape(form) + r"(?:/[0-9a-z]+)?/work", "/work", out)
            out = out.replace(form, "/record")
        for flat in self._flat:
            out = re.sub(re.escape(flat) + r"(?:-[0-9a-z]+)?-work", "-work", out)
        out = re.sub(r"/(?:private/)?tmp/claude-\d+/", "/tmp/claude/", out)
        for word in self._forbidden:
            if word and word in out:
                raise RuntimeError("a recorded line names the home folder or the user")
        return out


def _write(path: Path, lines: list[str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("".join(line + "\n" for line in lines), encoding="utf-8")
    print(f"wrote {path.relative_to(TESTS.parent)} ({len(lines)} lines)")


def _write_json(path: Path, doc: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {path.relative_to(TESTS.parent)}")


def resumed_meta(today: str) -> dict[str, Any]:
    """Return ``meta.json`` of the resumed session recording."""
    return {
        "cli_version": PINNED_CLAUDE_CODE_VERSION,
        "origin": (
            f"recorded {today} by tests/real_cli/record.py: the pinned CLI against a local "
            "stand-in of the Messages API on loopback (fixed answer, 1200 input, 40 output, 500 "
            "cache read and 300 cache write tokens per request), its own CLAUDE_CONFIG_DIR, no "
            "credential and no model call; turn 1 with --session-id, turn 2 with --resume of the "
            "same session"
        ),
        "files": {
            "transcript.jsonl": (
                "the session transcript after turn 1 (projects/<folder>/<session>.jsonl), "
                "reduced to its user, assistant, last-prompt and cost-state lines; the "
                "cost-state line is what turn 2 restored"
            ),
            "stream_resumed.jsonl": (
                "the stream-json output of turn 2 (--resume); the init message reduced to the "
                "fields the runner reads"
            ),
        },
        "note": (
            "Paths replaced by /work; the session reference is invented. The result of the "
            "resumed turn counts modelUsage and total_cost_usd for the whole session (turn 1 "
            "plus turn 2), while its usage counts turn 2 only."
        ),
    }


def _roots(base: Path, names: tuple[str, ...]) -> Iterator[Path]:
    for name in names:
        root = base / name
        root.mkdir()
        yield root


def _program(name: str) -> Path:
    found = find_program(name, clean_path(os.environ.get("PATH", "")))
    if found is None:
        raise SystemExit(f"record: {name} is not installed")
    return Path(found).resolve()


def main(argv: list[str] | None = None) -> int:
    """Record every fixture of the pinned CLI; see the module docstring."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").splitlines()[0])
    parser.add_argument(
        "--claude-bin",
        default=os.environ.get("CLAUDE_CODE_RUNNER_CLAUDE_BIN", "claude"),
        help="the pinned CLI (default: CLAUDE_CODE_RUNNER_CLAUDE_BIN, else claude in PATH)",
    )
    args = parser.parse_args(argv)
    program = _program(args.claude_bin)
    with tempfile.TemporaryDirectory() as tmp:
        base = Path(tmp)
        if found := private_ancestors(base / "work"):
            raise SystemExit(f"record: the temporary folder lies below {found[0].parent}")
        probe = subprocess.run(
            [str(program), "--version"],
            capture_output=True,
            text=True,
            timeout=120,
            check=False,
            cwd=base,
            env={"HOME": str(base), "PATH": os.environ.get("PATH", "/usr/bin:/bin")},
        )
        version = ".".join(map(str, parse_version(probe.stdout) or ()))
        if version != PINNED_CLAUDE_CODE_VERSION:
            raise SystemExit(
                f"record: the CLI is {version or 'unreadable'}, the pin is "
                f"{PINNED_CLAUDE_CODE_VERSION}; move the pin first"
            )
        today = datetime.now(UTC).date().isoformat()
        pin = PINNED_CLAUDE_CODE_VERSION
        print(f"recording Claude Code {pin} against the local stand-in (no login, no model call)")
        tree_root, resume_root, error_root = _roots(base, ("tree", "resume", "errors"))
        tree = [Cleaner(tree_root).line(x) for x in record_agent_tree(program, tree_root)]
        _write(RECORDING / f"agent_tree_{pin}.jsonl", tree)
        fields = fields_document(cli_bytes(program), [json.loads(x) for x in tree], today)
        _write_json(RECORDING / f"stream_fields_{pin}.json", fields)
        transcript, stream = record_resumed_session(program, resume_root)
        clean = Cleaner(resume_root)
        folder = RECORDING / f"resumed_session_{pin}"
        _write(folder / "transcript.jsonl", [clean.line(x) for x in transcript])
        _write(folder / "stream_resumed.jsonl", [clean.line(x) for x in stream])
        _write_json(folder / "meta.json", resumed_meta(today))
        print("capturing the API errors (the refused key takes about three minutes)")
        errors = [Cleaner(error_root).line(x) for x in record_api_errors(program, error_root)]
        _write(CAPTURED / f"api_errors_{pin}.jsonl", errors)
    return 0


if __name__ == "__main__":
    sys.exit(main())
