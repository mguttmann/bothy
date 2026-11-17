# SPDX-License-Identifier: Apache-2.0
r"""A turn as a Claude Code background session: launch, list, stop, remove (09 18.2, 18.6).

With ``runner.launch_mode`` ``background`` a turn of the runner's own agent in billing mode
``personal`` is not a ``claude -p`` process of the runner but a background session of the CLI
(``claude --bg``), which runs under the CLI's own supervisor. This module is the runner's side
of that contract and holds every surface of the pinned CLI the mode rests on, each behind a
pure function a test holds against a recording of the pinned version:

* the launch command (`background_argv`, `settings_document`, `prompt_text`) and what it prints
  (`parse_launch`): ``backgrounded · <short id> · <name>``, the refusals of an untrusted folder
  and of bypass mode without a recorded acceptance, and the ``note:`` lines of a copy;
* the session list ``claude agents --json --all`` (`parse_sessions`), of which the runner reads
  ``id``, ``sessionId``, ``pid``, ``status``, ``waitingFor``, ``kind`` and ``name`` of its own
  sessions and never ``state``, which a model request writes;
* ``claude stop``, ``claude rm`` and ``claude daemon status`` (`SessionCli`, `parse_supervisor`);
* one JSON line per input on the session's inbox socket (`socket_frame`, `send_frames`), for a
  session that bypasses permission prompts inside the CLI's envelope that states the sender's
  class (`sender_class`).

What a session takes from the other Claude Code sessions of the host user is never widened: the
settings file of a run sets no ``crossSessionInbound``, so the CLI's own rule per message and
any stricter value of the host stay, and the tools found in the pinned CLI that write to or
list other sessions are disallowed (`DENIED_TOOLS`). Neither authenticates a sender: the runner
decides by the ``uuid`` of every input the transcript shows (`claude_code_runner.bgtranscript`,
09 18.5).

A session inherits the environment of the supervisor, which lives across runs. So the launch
command, which starts the supervisor when none runs, gets the fixed environment of
`claude_code_runner.bgmode.BackgroundMode` and never a value of a run, and every value of a run
reaches its session only through the ``env`` block of the settings file, where every name is
set in every launch (`RUN_ENV_NAMES`).

`BgSession` is the handle of a started session as `claude_code_runner.runturn.RunTurn` drives a
turn process: it tells whether the process lives, writes inputs, and stops the session with
``claude stop`` and the escalation of 18.6. `SessionList` is the one reader of the session list
for every background turn of the runner process.

The runner never installs, starts or stops the supervisor. The launch command starts it when
none runs, and a supervisor the runner did not start, or one of another version than the pin,
makes a run use print mode (`supervisor_verdict`): sessions inherit the supervisor's
environment, and only a supervisor a launch of the runner started carries the allow list of
18.2 and nothing else.

No function of this module copies a transcript line, a prompt or an output of the CLI into a
log line; log lines carry event codes and ids.

Examples:
    >>> parse_launch(0, "backgrounded · 1a2b3c4d · bothy-x\n", "").short_id
    '1a2b3c4d'
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import os
import re
import signal
import socket
import subprocess  # nosec B404 - argument lists, never a shell, program from the cleaned PATH
import time
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from claude_code_runner.adoption import process_id, read_identity
from claude_code_runner.commandline import (
    ArgumentError,
    HostedArgs,
    class_flags,
    hosted_additions,
    hosted_class_flags,
)
from claude_code_runner.config import check_argument
from claude_code_runner.statedir import ensure_private_dir, write_private
from claude_code_runner.stop import escalation_due

__all__ = [
    "DENIED_TOOLS",
    "NAME_PREFIX",
    "RUN_ENV_NAMES",
    "BgSession",
    "LaunchOutput",
    "Reading",
    "SessionCli",
    "SessionEntry",
    "SessionList",
    "Supervisor",
    "background_argv",
    "background_class_flags",
    "launch",
    "load_supervisor",
    "parse_launch",
    "parse_sessions",
    "parse_supervisor",
    "prompt_text",
    "send_frames",
    "sender_class",
    "session_name",
    "settings_document",
    "socket_frame",
    "store_supervisor",
    "supervisor_verdict",
]

_LOG = logging.getLogger("claude_code_runner")

NAME_PREFIX = "bothy-"
"""The name prefix of every session the runner launches (``--name bothy-<run id>``): an entry
of the session list under another name belongs to another program and is never stopped or
removed."""

DENIED_TOOLS = ("AskUserQuestion", "SendMessage", "ListAgents", "SendFile")
"""The tools no background session of the runner has (``--disallowedTools``, 09 18.2). A
question of ``AskUserQuestion`` blocks a session until a person answers it. The other three
are the tools found in the pinned CLI that reach the other Claude Code sessions of the host
user: ``SendMessage`` writes to one and ``ListAgents`` lists them (run with the pinned CLI),
and ``SendFile`` sends files to one, by a name of that list or by the address of its socket
(read in the file of the pinned CLI: an account feature switches the tool on, so a session
without a login offers none and no run showed it). The deny rule for ``SendMessage`` also
takes the messages to the session's own subagents and teammates, which that tool serves too.

The list is what was found, by runs and by reading the file, not a proof that the CLI has no
further such tool: one that a later version or another account feature adds is not denied
until it stands here, and ``tests/real_cli/test_pinned_cli.py`` holds each name in the file
of the pinned version."""

RUN_ENV_NAMES = ("CLAUDE_PRODUCT_RUN", "DISABLE_AUTOUPDATER", "TMPDIR", "SSH_AUTH_SOCK")
"""The names of the ``env`` block of a run's settings file, each set in every launch."""

_ENVELOPE = "cross-session-message"
"""The element the CLI wraps a message of another session in; its attribute ``from-mode`` is
how a sender states its permission class (read in the file of the pinned CLI and run)."""
_ENVELOPE_LETTERS = "".join(c for c in _ENVELOPE if c.isalpha())
_BYPASS_CLASS = "bypass"
"""The class of a session that bypasses permission prompts (``bypassPermissions``)."""

_LAUNCHED = re.compile(r"^backgrounded · ([0-9a-f]{8}) · ", re.MULTILINE)
"""The first line of a launch (run with the pinned CLI 2.1.325 on 2026-11-16,
``tests/fixtures/recording/background_<version>``)."""
_UNTRUSTED = "Workspace not trusted."
_BYPASS = "--bg with bypassPermissions requires accepting the disclaimer first."
_SERVICE = "Starting background service"
_NOTE = "note:"
_PID = re.compile(r"^pid:\s+(\d+)\s*$", re.MULTILINE)
_VERSION = re.compile(r"^version:\s+(\S+)\s*$", re.MULTILINE)
_SUPERVISOR_FILE = "supervisor.json"
_IDENTITY_INTERVAL = 1.0
"""``ps`` checks the start time of a session process at most this often (seconds); between
two checks the process counts as alive while it still leads its group."""
_SOCKET_TIMEOUT = 5.0


def _log(event: str, **ids: object) -> None:
    _LOG.info(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def _warn(event: str, **ids: object) -> None:
    _LOG.warning(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


# ---- the launch command ----------------------------------------------------------------------


def session_name(run_id: object) -> str:
    """The name of a run's session in the session list.

    Args:
        run_id: The run.

    Returns:
        ``bothy-<run id>``.
    """
    return f"{NAME_PREFIX}{run_id}"


def background_class_flags(
    tool_class: str, tools: Sequence[str], hosted: HostedArgs | None = None
) -> list[str]:
    """Return the flags of a tool class for a background session (09 18.2).

    The flags of `claude_code_runner.commandline.class_flags`, and under ``scoped`` and ``host``
    `DENIED_TOOLS` in ``--disallowedTools``: nobody is there to answer a question, and a
    session of the runner neither writes or sends files to nor lists the other sessions of
    the host user. ``chat`` has no tool (``--tools ""`` leaves none, run with the pinned CLI).

    Args:
        tool_class: ``chat``, ``scoped`` or ``host``.
        tools: The named tools of ``scoped``.
        hosted: The parts of a hosted turn: the tool flags of 26 5.3 instead of 18.2.

    Returns:
        The flags.

    Raises:
        ArgumentError: As `claude_code_runner.commandline.class_flags`.

    Examples:
        >>> background_class_flags("host", [])[:3]
        ['--permission-mode', 'bypassPermissions', '--disallowedTools']
        >>> background_class_flags("host", [])[3:]
        ['AskUserQuestion', 'SendMessage', 'ListAgents', 'SendFile']
    """
    if hosted is None:
        flags = class_flags(tool_class, tools)
    else:
        flags = hosted_class_flags(tool_class, tools, has_skills=hosted.has_skills)
    if tool_class == "chat":
        return flags
    return [*flags, "--disallowedTools", *DENIED_TOOLS]


def background_argv(  # noqa: PLR0913 - every input of the command line is explicit
    *,
    claude_bin: str,
    model: str,
    resume: str | None,
    name: str,
    tool_class: str,
    tools: Sequence[str],
    add_dir: Path | None,
    system_prompt_file: Path,
    socket_file: Path,
    settings_file: Path,
    hosted: HostedArgs | None = None,
    plugin_dirs: Sequence[Path] = (),
) -> list[str]:
    """Build the argument list of the launch command in the order of 09 18.2.

    A hosted turn (26 5.3) and a turn with channel skills (26 5.9) carry the same parts as in
    print mode: the tool flags of the hosted class, ``--restricted`` or ``--setting-sources``,
    ``--plugin-dir`` per folder and ``--mcp-config`` with ``--strict-mcp-config``. The hosted
    settings are not a second ``--settings``: `settings_document` writes them into the one
    settings file of the launch. ``--max-budget-usd`` is left out, because a background
    session does not act on it (the runner holds the cost cap itself).

    The prompt is not an argument: it is the stdin of the command, so it is neither bounded
    by the system's argument limit nor readable in the process list. ``--session-id`` is
    left out, because the CLI makes the session id of a background session itself, and
    ``--permission-prompts none`` belongs to print mode.

    Args:
        claude_bin: The absolute path of the CLI found in the cleaned ``PATH``.
        model: The model.
        resume: The session to continue (``--resume``); ``None`` starts a new session.
        name: The name of the session (`session_name`).
        tool_class: ``runner.tool_class``.
        tools: The tools of ``scoped``.
        add_dir: The thread's incoming folder, only for ``scoped`` and ``host`` with files.
        system_prompt_file: The file of ``--append-system-prompt-file``.
        socket_file: The inbox socket (``--messaging-socket-path``).
        settings_file: The settings file of the run (``--settings``).
        hosted: The parts of a hosted turn (26 5.3); ``None`` for the runner's own agent.
        plugin_dirs: Channel skill folders (26 5.9), only for ``host``.

    Returns:
        The argument list.

    Raises:
        ArgumentError: An invalid model or tool, ``add_dir`` or an MCP file under ``chat``,
            or a channel skill folder below ``host``.
    """
    if not check_argument(model):
        raise ArgumentError("invalid_argument")
    if add_dir is not None and tool_class == "chat":
        raise ArgumentError("invalid_argument")
    if plugin_dirs and tool_class != "host":  # 26 5.9: channel skills only in host turns
        raise ArgumentError("invalid_argument")
    argv = [claude_bin]
    if resume is not None:
        argv += ["--resume", resume]
    argv += ["--bg", "--name", name, "--model", model]
    argv += background_class_flags(tool_class, tools, hosted)
    if hosted is None:
        argv += [a for folder in plugin_dirs for a in ("--plugin-dir", str(folder))]
    else:
        argv += hosted_additions(tool_class, hosted, plugin_dirs, settings=False)
    if add_dir is not None:
        argv += ["--add-dir", str(add_dir)]
    argv += [
        "--append-system-prompt-file",
        str(system_prompt_file),
        "--messaging-socket-path",
        str(socket_file),
        "--settings",
        str(settings_file),
    ]
    return argv


def settings_document(
    env: Mapping[str, str], base: Mapping[str, Any] | None = None
) -> dict[str, Any]:
    """Return the settings of a run's background session (``--settings``, 09 18.2).

    Exactly two members. ``worktree.bgIsolation`` ``none`` keeps the session in the project
    folder: without it a background session moves into a Git worktree of its own, against
    18.1, and its transcript moves with it. ``env`` carries what must reach the session
    process, because of the launching command's environment only ``PATH`` does.

    Every name of `RUN_ENV_NAMES` is set in every document, a name the turn has no value for
    to the empty text: a session inherits the environment of the supervisor, which lives across
    runs, and a value in the ``env`` block goes before an inherited one (run with the pinned
    CLI). So a run that must not have the SSH agent gets an empty ``SSH_AUTH_SOCK`` whatever
    the supervisor carries.

    The document sets no ``crossSessionInbound``. Without a value the CLI decides per message
    by the permission classes of both sessions, and a stricter value of the host user's or of
    managed settings stays in force; ``accept`` here would switch both off (18.5).

    A hosted turn gives the settings of 26 5.4 as ``base``; the document is then those
    settings with the two members added (they carry no ``env`` and no ``worktree``).

    Args:
        env: The environment of the turn (`claude_code_runner.environment.build_environment`).
        base: The settings of a hosted turn, if any.

    Returns:
        The document; ``env`` holds ``CLAUDE_PRODUCT_RUN``, ``DISABLE_AUTOUPDATER``, the run's
        ``TMPDIR`` and ``SSH_AUTH_SOCK``, which is empty unless the turn's environment has it.
    """
    session_env = {
        "CLAUDE_PRODUCT_RUN": "1",
        "DISABLE_AUTOUPDATER": "1",
        "TMPDIR": env.get("TMPDIR", ""),
        "SSH_AUTH_SOCK": env.get("SSH_AUTH_SOCK", ""),
    }
    return {**(base or {}), "worktree": {"bgIsolation": "none"}, "env": session_env}


def sender_class(tool_class: str) -> str | None:
    """Return the permission class the runner's lines state for a session of a tool class.

    Without a value of ``crossSessionInbound`` the CLI takes a message into a session that
    bypasses permission prompts only when its sender states that it bypasses them too, and
    into every other session unless the sender states exactly that (run with the pinned CLI).
    A turn of class ``host`` runs with ``bypassPermissions``, and the person whose message the
    runner relays was allowed that class by the server (18.5), so its lines state ``bypass``.
    The statement is a claim any sender can make, not a proof.

    Args:
        tool_class: ``chat``, ``scoped`` or ``host``.

    Returns:
        ``bypass`` for ``host``; ``None`` for a class whose sessions prompt (``dontAsk``),
        which take a line that states nothing.

    Examples:
        >>> sender_class("host"), sender_class("scoped")
        ('bypass', None)
    """
    return _BYPASS_CLASS if tool_class == "host" else None


def _fits_envelope(text: str) -> bool:
    """Whether the CLI reads a text in its envelope as the runner wrote it.

    The CLI takes the sender's class only from an envelope it would write itself, and it
    rewrites a body that holds the element's closing tag, which it matches in many spellings
    (other letter cases, look-alike and invisible characters). The runner does not copy that
    rule: a text whose letters ``a`` to ``z``, read without anything between them and in any
    case, hold the element's name is not put into an envelope at all. That covers other
    letter cases and characters between the letters, not a spelling with look-alike letters,
    which passes here and is left to the CLI's own rule.
    """
    letters = "".join(c for c in text.casefold() if "a" <= c <= "z")
    return _ENVELOPE_LETTERS not in letters


def _line_parts(line: bytes | str) -> tuple[str, str] | None:
    """The ``uuid`` and the text of one input line of the runner (a ``user`` JSON line)."""
    try:
        doc = json.loads(line)
        content = doc["message"]["content"]
        ident = doc["uuid"]
    except (ValueError, KeyError, TypeError):
        return None
    if not isinstance(ident, str):
        return None
    if isinstance(content, str):
        return ident, content
    if not isinstance(content, list):
        return None
    texts = [i.get("text") for i in content if isinstance(i, Mapping) and i.get("type") == "text"]
    return ident, "\n\n".join(t for t in texts if isinstance(t, str))


def prompt_text(line: bytes | str) -> str | None:
    r"""Return the prompt of a launch: the text items of an input line joined by a blank line.

    The first input line of a run has two items, the context with trust marks and the trigger
    part (18.2 "Input"); as one text the context comes first, so the prompt never starts with
    ``/`` or ``!``.

    Args:
        line: An input line of the runner (`claude_code_runner.prompt.user_line`).

    Returns:
        The text, or ``None`` for a line that is no such message.

    Examples:
        >>> prompt_text('{"type":"user","uuid":"u","message":{"role":"user","content":'
        ...             '[{"type":"text","text":"context"},{"type":"text","text":"do it"}]}}')
        'context\n\ndo it'
    """
    parts = _line_parts(line)
    return parts[1] if parts is not None else None


def socket_frame(line: bytes | str, *, sender_class: str | None = None) -> bytes | None:
    """Return the inbox line of one input line of the runner (09 18.5).

    One JSON line ``{"type":"user","uuid":"<uuid>","message":{"role":"user","content":"<text>"}}``
    is what a session takes as a message from its inbox socket (run with the pinned CLI); the
    ``uuid`` is kept in the transcript because the socket was named at the launch, so the
    echo of the line is exact and the runner knows its own lines by it.

    With ``sender_class`` the text stands in the CLI's envelope
    ``<cross-session-message from-mode="<class>">``, a line break, the text, a line break and
    the closing tag, which is how a sender states its permission class (`sender_class`).

    Args:
        line: An input line of the runner: a follow-up, a retry line or any other input.
        sender_class: The class the line states; ``None`` for a line that states none.

    Returns:
        The line for the socket with its line break; ``None`` for a line that is no message,
        and for a text the envelope cannot carry unchanged (it holds the element's name).
    """
    parts = _line_parts(line)
    if parts is None:
        return None
    text = parts[1]
    if sender_class is not None:
        if not _fits_envelope(text):
            return None
        text = f'<{_ENVELOPE} from-mode="{sender_class}">\n{text}\n</{_ENVELOPE}>'
    doc = {"type": "user", "uuid": parts[0], "message": {"role": "user", "content": text}}
    return json.dumps(doc, ensure_ascii=False, separators=(",", ":")).encode("utf-8") + b"\n"


def send_frames(socket_file: Path, frames: Sequence[bytes], timeout: float) -> bool:
    """Write lines to a session's inbox socket (blocking; call it from a worker thread).

    Args:
        socket_file: The socket the session was launched with.
        frames: The lines (`socket_frame`).
        timeout: The bound of the connect and of each write in seconds.

    Returns:
        True once every byte was written; False when the socket is missing, refuses the
        connection or does not take the bytes in time.
    """
    try:
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
            sock.settimeout(timeout)
            sock.connect(str(socket_file))
            for frame in frames:
                sock.sendall(frame)
    except (OSError, ValueError):
        return False
    return True


# ---- what the CLI prints -----------------------------------------------------------------------


@dataclass(frozen=True)
class LaunchOutput:
    """What a launch command printed.

    Attributes:
        short_id: The short id of the session (the first eight characters of its session id);
            ``None`` when the command started none.
        refusal: Why the command started nothing: ``untrusted`` (the folder has no workspace
            trust), ``bypass`` (bypass mode without a recorded acceptance), ``failed`` (any
            other error or an output the runner cannot read); ``None`` after a launch.
        started_service: The command started the supervisor (none ran before).
        notes: The count of ``note:`` lines on stderr; one says that the command did not
            continue the session in place but started a copy, or woke it with saved options.
        exit_error: The command ended with an error exit: it is certain that it started no
            session. False after a launch, a command that did not end in time, and an exit 0
            whose output the runner cannot read.
    """

    short_id: str | None
    refusal: str | None = None
    started_service: bool = False
    notes: int = 0
    exit_error: bool = False


def parse_launch(returncode: int | None, stdout: str, stderr: str) -> LaunchOutput:
    """Read the output of ``claude [--resume <id>] --bg``.

    Args:
        returncode: The exit code; ``None`` when the command did not end in time.
        stdout: Its standard output.
        stderr: Its standard error (only matched, never kept).

    Returns:
        The short id of the launched session, or the refusal.

    Examples:
        >>> parse_launch(1, "", "Workspace not trusted. Run `claude` in /p once.").refusal
        'untrusted'
        >>> parse_launch(0, "nothing", "").refusal
        'failed'
    """
    service = _SERVICE in stderr
    notes = sum(1 for line in stderr.splitlines() if line.strip().startswith(_NOTE))
    found = _LAUNCHED.search(stdout)
    if returncode == 0 and found is not None:
        return LaunchOutput(found.group(1), None, service, notes)
    if _UNTRUSTED in stderr:
        refusal = "untrusted"
    elif _BYPASS in stderr:
        refusal = "bypass"
    else:
        refusal = "failed"
    return LaunchOutput(None, refusal, service, notes, returncode not in (None, 0))


@dataclass(frozen=True)
class SessionEntry:
    """One entry of the session list, reduced to what the runner reads (09 18.4).

    Of an entry under another name only its short id, its session, its name and whether it
    has a process are kept: such a session belongs to another program.

    Attributes:
        short_id: ``id``.
        session_id: ``sessionId``.
        name: ``name``.
        own: The name carries `NAME_PREFIX`: the runner launched the session.
        has_process: ``pid`` is present: a process of the session lives.
        pid: ``pid`` of an own entry.
        status: ``status`` of an own entry: ``busy``, ``idle`` or ``waiting``.
        waiting_for: ``waitingFor`` of an own entry: why the session waits for a person.
        kind: ``kind`` of an own entry.
    """

    short_id: str
    session_id: str
    name: str
    own: bool
    has_process: bool
    pid: int | None = None
    status: str | None = None
    waiting_for: str | None = None
    kind: str | None = None


def _text(value: object) -> str:
    return value if isinstance(value, str) else ""


def parse_sessions(stdout: str) -> list[SessionEntry] | None:
    """Read the output of ``claude agents --json --all``.

    ``state`` is never read: a model request writes it at each turn end, and it was wrong or
    stuck in several turns of the pinned CLI.

    Args:
        stdout: The standard output.

    Returns:
        The entries, or ``None`` when the output is no JSON list (the list is unreadable).
    """
    try:
        raw = json.loads(stdout)
    except ValueError:
        return None
    if not isinstance(raw, list):
        return None
    entries: list[SessionEntry] = []
    for item in raw:
        if not isinstance(item, Mapping):
            continue
        name = _text(item.get("name"))
        own = name.startswith(NAME_PREFIX)
        pid = process_id(item.get("pid"))
        entry = SessionEntry(
            short_id=_text(item.get("id")),
            session_id=_text(item.get("sessionId")),
            name=name,
            own=own,
            has_process=item.get("pid") is not None,
        )
        if own:
            entry = SessionEntry(
                entry.short_id,
                entry.session_id,
                name,
                own=True,
                has_process=entry.has_process,
                pid=pid,
                status=_text(item.get("status")) or None,
                waiting_for=_text(item.get("waitingFor")) or None,
                kind=_text(item.get("kind")) or None,
            )
        entries.append(entry)
    return entries


@dataclass(frozen=True)
class Supervisor:
    """The supervisor ``claude daemon status`` reports.

    Attributes:
        pid: Its process id; ``None`` when the output names none.
        version: Its version; ``None`` when the output names none.
    """

    pid: int | None
    version: str | None


def parse_supervisor(returncode: int | None, stdout: str) -> tuple[bool, Supervisor | None]:
    r"""Read the output of ``claude daemon status``.

    Args:
        returncode: The exit code (0 while a supervisor runs, 1 when none does); ``None`` when
            the command did not end.
        stdout: The standard output.

    Returns:
        Whether the answer could be read, and the supervisor when one runs.

    Examples:
        >>> parse_supervisor(1, "not running\n")
        (True, None)
        >>> parse_supervisor(0, "pid:     4711\nversion: 2.1.325\n")[1]
        Supervisor(pid=4711, version='2.1.325')
    """
    if returncode is None:
        return False, None
    pid = _PID.search(stdout)
    version = _VERSION.search(stdout)
    if pid is None and version is None:
        if returncode != 0 and "not running" in stdout:
            return True, None
        return False, None
    return True, Supervisor(
        process_id(int(pid.group(1))) if pid is not None else None,
        version.group(1) if version is not None else None,
    )


def supervisor_verdict(
    found: Supervisor | None,
    recorded: Mapping[str, Any] | None,
    pinned: str,
    lstart: Callable[[int], str | None],
) -> str | None:
    """Decide whether a launch may use the supervisor that runs now (09 18.2).

    Args:
        found: The supervisor ``claude daemon status`` reports; ``None`` when none runs.
        recorded: The supervisor a launch of the runner started (``pid``, ``lstart``), if any.
        pinned: The pinned version of the CLI.
        lstart: Reads the start time of a process (``None`` when it does not exist).

    Returns:
        ``None`` when the launch may go on: no supervisor runs (the launch starts one), or the
        one that runs is the recorded one in the pinned version. Else the code of the
        fallback: ``background_supervisor_foreign`` for a supervisor the runner did not start,
        ``background_supervisor_version`` for its own in another version.
    """
    if found is None:
        return None
    ours = (
        recorded is not None
        and found.pid is not None
        and recorded.get("pid") == found.pid
        and bool(recorded.get("lstart"))
        and lstart(found.pid) == recorded.get("lstart")
    )
    if not ours:
        return "background_supervisor_foreign"
    if found.version != pinned:
        return "background_supervisor_version"
    return None


def load_supervisor(state_dir: Path) -> dict[str, Any] | None:
    """Read the supervisor a launch of the runner started (``<state_dir>/bg/supervisor.json``).

    Args:
        state_dir: ``runner.state_dir``.

    Returns:
        ``pid`` and ``lstart``, or ``None`` without a readable record.
    """
    try:
        raw = json.loads((state_dir / "bg" / _SUPERVISOR_FILE).read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    if not isinstance(raw, dict) or process_id(raw.get("pid")) is None:
        return None
    if not isinstance(raw.get("lstart"), str) or not raw["lstart"]:
        return None
    return {"pid": raw["pid"], "lstart": raw["lstart"]}


def store_supervisor(state_dir: Path, pid: int, lstart: str) -> None:
    """Record the supervisor a launch of the runner started (atomic, mode ``0600``).

    Args:
        state_dir: ``runner.state_dir``.
        pid: Its process id.
        lstart: Its start time as ``ps -o lstart=`` prints it.
    """
    folder = ensure_private_dir(state_dir / "bg")
    doc = json.dumps({"pid": pid, "lstart": lstart}, sort_keys=True)
    write_private(folder / _SUPERVISOR_FILE, doc, atomic=True)


# ---- the control commands ----------------------------------------------------------------------


async def _reap(proc: asyncio.subprocess.Process) -> None:
    """Kill a control command and wait for it (also when the waiting task is cancelled)."""
    with contextlib.suppress(ProcessLookupError):
        proc.kill()
    with contextlib.suppress(asyncio.CancelledError):
        await asyncio.shield(proc.wait())


class SessionCli:
    """The control commands of background sessions, each one short CLI call.

    Every call starts the CLI program with the environment of a ``personal`` turn, stdin
    closed and the bound of ``runner.cli_probe_timeout``; none of them needs a terminal or a
    running supervisor.

    Args:
        claude_bin: The CLI found in the cleaned ``PATH``.
        env: The allow-list environment of 18.2 with ``DISABLE_AUTOUPDATER``.
        cwd: The working directory of the calls (``runner.state_dir``).
        timeout: ``runner.cli_probe_timeout``.
    """

    def __init__(self, claude_bin: str, env: Mapping[str, str], cwd: Path, timeout: float) -> None:
        """Create the caller; nothing runs before a call."""
        self.claude_bin = claude_bin
        self.env = dict(env)
        self.cwd = cwd
        self.timeout = timeout

    async def run(self, *args: str) -> tuple[int | None, str, str]:
        """Run ``<claude_bin> <args>`` and return its exit code, stdout and stderr.

        Args:
            *args: The arguments.

        Returns:
            The exit code (``None`` after a start failure or the time limit), stdout and
            stderr as text.
        """
        try:
            proc = await asyncio.create_subprocess_exec(  # nosec B603 - argument list, no shell
                self.claude_bin,
                *args,
                stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                cwd=self.cwd,
                env=self.env,
                start_new_session=True,
            )
        except OSError:
            return None, "", ""
        try:
            out, err = await asyncio.wait_for(proc.communicate(), self.timeout)
        except TimeoutError:
            await _reap(proc)
            return None, "", ""
        except asyncio.CancelledError:
            await _reap(proc)  # a stopping runner leaves no control command behind
            raise
        return proc.returncode, out.decode("utf-8", "replace"), err.decode("utf-8", "replace")

    async def sessions(self) -> list[SessionEntry] | None:
        """``claude agents --json --all``: the session list, ``None`` when it is unreadable."""
        code, out, _ = await self.run("agents", "--json", "--all")
        if code != 0:
            return None
        return parse_sessions(out)

    async def stop(self, short_id: str) -> bool:
        """``claude stop <short id>``: True when the CLI stopped the session's process."""
        code, _, _ = await self.run("stop", short_id)
        return code == 0

    async def remove(self, short_id: str) -> bool:
        """``claude rm <short id>``: True when the CLI removed the entry (its transcript stays)."""
        code, _, _ = await self.run("rm", short_id)
        return code == 0

    async def supervisor(self) -> tuple[bool, Supervisor | None]:
        """``claude daemon status``: whether it could be read, and the supervisor that runs."""
        code, out, _ = await self.run("daemon", "status")
        return parse_supervisor(code, out)


async def launch(  # noqa: PLR0913 - the parts of one launch
    argv: Sequence[str],
    *,
    prompt: str,
    cwd: Path,
    env: Mapping[str, str],
    stdout: Path,
    stderr: Path,
    timeout: float,
) -> LaunchOutput:
    """Run the launch command with the prompt as its stdin and read what it printed.

    The command returns at once: the session process belongs to the supervisor. Its stdout
    and stderr go to files of the run (mode ``0600``).

    Args:
        argv: The argument list (`background_argv`).
        prompt: The prompt (`prompt_text`).
        cwd: The project folder.
        env: The launch environment: the fixed one of the runner process
            (`claude_code_runner.bgmode.BackgroundMode.env`), never the environment of a
            run. The command starts the supervisor when none runs, and every later session
            inherits what the supervisor was started with.
        stdout: The file of the command's stdout.
        stderr: The file of the command's stderr.
        timeout: ``runner.cli_probe_timeout``.

    Returns:
        The parsed output; the refusal ``failed`` when the command could not be started or
        did not end in time.
    """
    ensure_private_dir(stdout.parent)
    for path in (stdout, stderr):
        write_private(path, b"")
    code: int | None
    try:
        # Two small files of the run on the local disk, opened once right before the command
        # that writes them starts: no asynchronous file API (NOSONAR below,
        # D-FEAT-RUNNER-BG-SESSIONS-17).
        with stdout.open("wb") as out, stderr.open("wb") as err:  # NOSONAR
            proc = await asyncio.create_subprocess_exec(  # nosec B603 - argument list, no shell
                *argv,
                stdin=subprocess.PIPE,
                stdout=out,
                stderr=err,
                cwd=cwd,
                env=dict(env),
                start_new_session=True,
            )
    except OSError:
        return LaunchOutput(None, "failed", exit_error=True)
    try:
        await asyncio.wait_for(proc.communicate(prompt.encode("utf-8")), timeout)
        code = proc.returncode
    except TimeoutError:
        await _reap(proc)
        code = None
    except asyncio.CancelledError:
        await _reap(proc)
        raise
    return parse_launch(
        code, stdout.read_text("utf-8", "replace"), stderr.read_text("utf-8", "replace")
    )


# ---- the session list --------------------------------------------------------------------------


@dataclass(frozen=True)
class Reading:
    """One reading of the session list.

    Attributes:
        at: The monotonic time right before the list command started: everything the list
            shows was true at or after it.
        entries: The entries.
    """

    at: float
    entries: tuple[SessionEntry, ...]

    def entry(self, short_id: str) -> SessionEntry | None:
        """The runner's own entry with this short id, if the list has one."""
        return next((e for e in self.entries if e.own and e.short_id == short_id), None)

    def named(self, name: str) -> SessionEntry | None:
        """The entry with this name, if the list has one."""
        return next((e for e in self.entries if e.name == name), None)

    def of_session(self, session_id: str) -> list[SessionEntry]:
        """Every entry of a session, under whatever name."""
        return [e for e in self.entries if e.session_id == session_id]


class SessionList:
    """The one reader of the session list for every background turn of the process (18.4).

    A reading is shared: a turn that asks for one gets the last when it is younger than
    ``runner.background_status_interval``, and a new one otherwise; a turn that read its turn
    end asks for a reading taken after that moment. Only one list command runs at a time.

    Args:
        cli: The control commands.
        interval: ``runner.background_status_interval``.
        clock: Monotonic seconds (tests).
    """

    def __init__(
        self, cli: SessionCli, interval: float, clock: Callable[[], float] = time.monotonic
    ) -> None:
        """Create the reader; nothing is read before `read`."""
        self.cli = cli
        self.interval = interval
        self._clock = clock
        self._lock = asyncio.Lock()
        self._last: Reading | None = None

    @property
    def last(self) -> Reading | None:
        """The newest reading so far."""
        return self._last

    def now(self) -> float:
        """The reader's clock: a reading asked for ``after`` this moment is a new one."""
        return self._clock()

    def _fresh(self, after: float | None) -> Reading | None:
        last = self._last
        if last is None:
            return None
        if after is not None:
            return last if last.at >= after else None
        return last if self._clock() - last.at < self.interval else None

    async def read(self, *, after: float | None = None) -> Reading | None:
        """Return a reading, taking a new one when the last is too old.

        Args:
            after: A monotonic time; the reading must have started at or after it. Without
                it a reading younger than the interval is enough.

        Returns:
            The reading, or ``None`` when the list could not be read now.
        """
        fresh = self._fresh(after)
        if fresh is not None:
            return fresh
        async with self._lock:
            fresh = self._fresh(after)  # another turn read it while this one waited
            if fresh is not None:
                return fresh
            at = self._clock()
            entries = await self.cli.sessions()
            if entries is None:
                return None
            self._last = Reading(at, tuple(entries))
            return self._last


# ---- one session as a turn process -------------------------------------------------------------


def _group_exists(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
    except (ProcessLookupError, PermissionError):
        return False
    return True


class BgSession:
    """The process of one background session, as the runner drives a turn process (18.6).

    The session process belongs to the supervisor, leads a process group of its own and names
    neither the session nor its flags in its command line, so it is known by its process id
    and start time. Before every signal both must still match (as for an adopted turn, 18.9).

    Args:
        cli: The control commands.
        short_id: The short id of the session.
        session_id: The session id.
        pid: The process the session list named for the session.
        lstart: Its start time as ``ps -o lstart=`` prints it.
        socket_file: The inbox socket the session was launched with.
        timeout: ``runner.cli_probe_timeout`` for ``ps``.
        sender_class: The permission class the runner's lines state for this session
            (`sender_class` of the run's tool class); ``None`` for a line that states none.
    """

    def __init__(  # noqa: PLR0913 - the parts of one session
        self,
        cli: SessionCli,
        *,
        short_id: str,
        session_id: str,
        pid: int,
        lstart: str,
        socket_file: Path,
        timeout: float,
        sender_class: str | None = None,
    ) -> None:
        """Wrap a started session; see the class docstring."""
        self.cli = cli
        self.short_id = short_id
        self.session_id = session_id
        self.pid = pid
        self.pgid = pid
        self.lstart = lstart
        self.socket_file = socket_file
        self.timeout = timeout
        self.sender_class = sender_class
        self.verified = False
        self.stop_requested = False
        """The runner asked the CLI to stop the session (a stop, or the release of a run)."""
        self._gone = False
        self._checked_at = 0.0
        self._escalated: signal.Signals | None = None
        self._stopping: asyncio.Task[None] | None = None

    # ---- liveness and identity ---------------------------------------------------------------

    def matches(self) -> bool:
        """Whether the process id still names the process the session was started as."""
        if self.pid <= 1 or not self.lstart:
            return False
        try:
            if os.getpgid(self.pid) != self.pgid:
                return False
        except (ProcessLookupError, PermissionError):
            return False
        found = read_identity(self.pid, self.timeout)
        return found is not None and found.lstart == self.lstart

    def alive(self) -> bool:
        """Whether the session process lives (its start time checked about once a second)."""
        if self._gone:
            return False
        try:
            leads = os.getpgid(self.pid) == self.pgid
        except (ProcessLookupError, PermissionError):
            leads = False
        now = time.monotonic()
        if leads and now - self._checked_at < _IDENTITY_INTERVAL:
            return True
        self._checked_at = now
        if leads and self.matches():
            self.verified = True
            return True
        self._gone = True
        return False

    def may_signal(self) -> bool:
        """The identity check before a signal to the group (18.9).

        Returns:
            True while the process matches, and afterwards while a process of a group that
            matched still lives (a group id is not given to a new process while the group
            exists).
        """
        if not self._gone and self.matches():
            self.verified = True
            return True
        return self.verified and _group_exists(self.pgid)

    def close_stdin(self) -> None:
        """Nothing to close: a background session has no stdin of the runner."""

    def poll(self) -> int | None:
        """``None`` while the session process lives, else 0."""
        return None if self.alive() else 0

    def exit_label(self) -> str:
        """The end of the process for the box note of 18.9.

        Returns:
            ``running`` while it lives; the name of the signal the stop escalated to; ``0``
            after a stop of the runner; ``unknown`` for a process that ended otherwise.
        """
        if self.alive():
            return "running"
        if self._escalated is not None:
            return self._escalated.name
        return "0" if self.stop_requested else "unknown"

    # ---- inputs ------------------------------------------------------------------------------

    async def write_bytes(self, data: bytes) -> bool:
        """Write complete input lines of the runner to the session's inbox socket.

        Args:
            data: One or more ``user`` lines as the runner writes them to stdin in print mode.

        Returns:
            True once every line reached the socket; False when the session takes no input
            (it is being stopped, the socket is missing or refuses the bytes) or a line
            cannot be written as the session would take it (`socket_frame`); then nothing
            was written.
        """
        if self.stop_requested:
            return False
        frames = [
            socket_frame(raw, sender_class=self.sender_class)
            for raw in data.split(b"\n")
            if raw.strip()
        ]
        if not frames or any(frame is None for frame in frames):
            return False
        checked = [frame for frame in frames if frame is not None]
        return await asyncio.to_thread(send_frames, self.socket_file, checked, _SOCKET_TIMEOUT)

    # ---- stop --------------------------------------------------------------------------------

    def _signal(self, sig: signal.Signals) -> bool:
        if self.pgid <= 1 or not self.may_signal():
            return False
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(self.pgid, sig)
        return True

    def _runs(self) -> bool:
        return self.alive() or (self.verified and _group_exists(self.pgid))

    async def stop(
        self,
        stop_grace: float,
        kill_grace: float,
        poll: float,
        ids: Mapping[str, object] | None = None,
    ) -> None:
        """Stop the session and return once no process of it is left (18.6).

        ``claude stop <short id>`` first; when a process of the session still lives after
        ``runner.stop_grace``, SIGTERM to its process group, after ``runner.kill_grace``
        SIGKILL, each logged with ``stop_escalated``. A second call waits for the first.

        Args:
            stop_grace: ``runner.stop_grace``.
            kill_grace: ``runner.kill_grace``.
            poll: ``runner.poll_interval``.
            ids: IDs for the log line of an escalation.
        """
        if self._stopping is None:
            self.stop_requested = True
            self._stopping = asyncio.ensure_future(
                self._stop(stop_grace, kill_grace, poll, dict(ids or {}))
            )
        await asyncio.shield(self._stopping)

    async def _stop(
        self, stop_grace: float, kill_grace: float, poll: float, ids: Mapping[str, object]
    ) -> None:
        started = time.monotonic()
        if self._runs() and self.short_id:
            await self.cli.stop(self.short_id)
        sent: set[signal.Signals] = set()
        while self._runs():
            due = escalation_due(time.monotonic(), started, True, stop_grace, kill_grace)
            if due is not None and due not in sent:
                sent.add(due)
                if due is signal.SIGKILL:
                    sent.add(signal.SIGTERM)
                _warn("stop_escalated", signal=due.name, **ids)
                if not self._signal(due):
                    break  # the identity changed: the process the runner started is gone
                self._escalated = due
            await asyncio.sleep(poll)
        self._gone = True

    async def remove(self) -> bool:
        """Remove the session's entry from the list (``claude rm``); its transcript stays."""
        if not self.short_id:
            return False
        return await self.cli.remove(self.short_id)

    async def wait(self, timeout: float, poll: float = 0.02) -> bool:
        """Wait for the end of the session process.

        Args:
            timeout: Seconds to wait at most.
            poll: Interval of the checks.

        Returns:
            True when no process of the session is left.
        """
        deadline = time.monotonic() + timeout
        while self._runs():
            if time.monotonic() >= deadline:
                return False
            await asyncio.sleep(poll)
        return True
