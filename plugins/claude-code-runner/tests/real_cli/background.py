# SPDX-License-Identifier: Apache-2.0
"""Background sessions of the real CLI against the local stand-in (09 18.2 "Background mode").

The free method of the real-CLI suite for background sessions: no login, no model call. The
CLI gets a ``HOME`` and a ``CLAUDE_CONFIG_DIR`` of its own, ``ANTHROPIC_BASE_URL`` on loopback
with the stand-in behind it, an ``apiKeyHelper`` in that folder's ``settings.json`` that prints
a dummy key (a background session takes no key from the environment), and a trust entry for the
working folder in that folder's ``.claude.json``. The supervisor of such a folder is a separate
instance with its own session list, so nothing here sees or touches a session of the developer.

`World` drives the sessions with the runner's own code (`claude_code_runner.bgsession`), so the
level-one test and the recordings hold exactly the surfaces the runner uses. `Script` is the
stand-in's answer for every scenario, chosen by words in the input that has no answer yet.

A world with ``login_token`` gives the CLI a dummy token in the place of the key helper, so
its requests carry the beta of a stored login (09 18.3) without any login: the token is no
credential, the requests still go to the stand-in, and `Refuser`, a proxy on loopback that
refuses whatever it is asked, is the only way out the CLI has, so the token reaches no host.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import shutil
import signal
import socket
import subprocess
import threading
import time
import uuid
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import Any

from real_cli.stand_in import DUMMY_KEY, Answer, ApiError, cli_env, text

from claude_code_runner.account import AccountError, retry_input
from claude_code_runner.bgsession import (
    LaunchOutput,
    SessionCli,
    SessionEntry,
    SessionList,
    background_argv,
    launch,
    send_frames,
    sender_class,
    settings_document,
    socket_frame,
)
from claude_code_runner.bgtranscript import TranscriptMapper
from claude_code_runner.commandline import HostedArgs
from claude_code_runner.prompt import user_line
from claude_code_runner.usage import find_transcript

__all__ = [
    "DUMMY_TOKEN",
    "MODEL",
    "REFUSED",
    "RETRY",
    "TOOL_SECONDS",
    "UNPARSED_REMOVAL",
    "WAIT",
    "Refuser",
    "Script",
    "World",
    "frame",
    "path_facts",
    "turn_ends",
]

MODEL = "claude-sonnet-4-5"
WAIT = 120.0
"""The bound of every wait (seconds)."""
RETRY = retry_input(AccountError("usage_limit"))
"""The runner's own retry line after an account error."""
REFUSED = ApiError(
    429,
    "rate_limit_error",
    "This request would exceed your account's rate limit.",
    headers={"x-should-retry": "false"},
)
"""The answer to a refused request; without the header the CLI retries a 429 for minutes."""

TOOL_SECONDS = 4
JOB_SECONDS = 8

DUMMY_TOKEN = "stand-in-token-not-a-login"
"""What a world with ``login_token`` gives the CLI as ``CLAUDE_CODE_OAUTH_TOKEN``."""

UNPARSED_REMOVAL = (
    (Path(__file__).parent / "unparsed_removal.txt").read_text(encoding="utf-8").rstrip("\n")
)
"""The command of ``RMSTEP`` (``{folder}`` is `Script.folder`), kept in ``unparsed_removal.txt``:
an agent's edit of a Makefile through a quoted Python heredoc whose text holds a recipe with
``rm -f "$$f"``. The removal is text and never runs, but the CLI's check of removals cannot
parse the whole command with certainty and asks for a person's approval even in bypass mode; a
background session then waits (FIX-RUNNER-BG-NO-PROMPT). The shape is that of a real agent's
command, with the addresses and values replaced."""


class Refuser:
    """A proxy on loopback that refuses: it notes the first line of what it is asked and closes.

    A world with a dummy token names it as the CLI's proxy for every host but loopback, so
    whatever the CLI would send anywhere else ends here and the token cannot leave the
    machine.

    Attributes:
        asked: The first line of every request that reached the proxy.
        url: The proxy as the CLI's environment names it.
    """

    def __init__(self) -> None:
        """Listen on a free loopback port."""
        self.asked: list[str] = []
        self._server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server.bind(("127.0.0.1", 0))
        self._server.listen(16)
        self.url = f"http://127.0.0.1:{self._server.getsockname()[1]}"
        threading.Thread(target=self._serve, daemon=True).start()

    def _serve(self) -> None:
        while True:
            try:
                connection, _ = self._server.accept()
            except OSError:
                return
            with connection, contextlib.suppress(OSError):
                connection.settimeout(2.0)
                first = connection.recv(512).split(b"\r\n", 1)[0]
                self.asked.append(first.decode("latin-1"))
                connection.sendall(b"HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n")

    def close(self) -> None:
        """Stop listening."""
        self._server.close()


class Script:
    """The answers of the stand-in, by words in the user input that has no answer yet.

    * ``TOOLSTEP``: one foreground command of `TOOL_SECONDS` seconds, then a text.
    * ``BGJOB``: one background command of `JOB_SECONDS` seconds, then a text.
    * ``REFUSE``: the request is refused with 429 until the runner's retry line comes.
    * ``ASK``: one question of ``AskUserQuestion``.
    * ``ENVSTEP``: one foreground command that prints the environment of the session.
    * ``FINDTOOLS``: one search for the tools that reach other sessions (``ToolSearch``).
    * ``RMSTEP``: one foreground command `UNPARSED_REMOVAL` in the folder ``folder``.
    * anything else: a text ``Answer <n>``.

    A request without tools is one of the CLI itself (a summary for its agent view), or a
    request of a session of class ``chat``, which has no tool: ``Ok.``.

    Attributes:
        bodies: The request bodies of the conversation, in order (for what a request carried).
        untooled: The bodies of the requests without tools, in order.
        folder: The folder of the ``RMSTEP`` command (the working folder of the session).
    """

    def __init__(self) -> None:
        """Start without a request."""
        self.bodies: list[Mapping[str, Any]] = []
        self.untooled: list[Mapping[str, Any]] = []
        self.folder = "."
        self._calls = 0

    def __call__(self, body: Mapping[str, Any]) -> Answer:  # noqa: PLR0911 - one answer per word
        """Answer one request of the CLI."""
        if not body.get("tools"):
            self.untooled.append(body)
            return [text("Ok.")]
        self.bodies.append(body)
        messages = body.get("messages") or []
        pending: list[str] = []
        for message in messages:
            pending = [] if message.get("role") == "assistant" else pending + _texts(message)
        joined = " ".join(pending)
        answers = sum(1 for m in messages if m.get("role") == "assistant")
        if "<tool_result>" in pending:
            return [text(f"Answer {answers}")]
        if "REFUSE" in joined and RETRY not in joined:
            return REFUSED
        self._calls += 1
        call = f"toolu_{self._calls:02d}"
        if "TOOLSTEP" in joined:
            command = {"command": f"sleep {TOOL_SECONDS}; echo slept", "description": "Sleep"}
            return [{"type": "tool_use", "id": call, "name": "Bash", "input": command}]
        if "ENVSTEP" in joined:
            command = {"command": "env", "description": "Environment"}
            return [{"type": "tool_use", "id": call, "name": "Bash", "input": command}]
        if "RMSTEP" in joined:
            removal = UNPARSED_REMOVAL.replace("{folder}", self.folder)
            command = {"command": removal, "description": "Edit the Makefile"}
            return [{"type": "tool_use", "id": call, "name": "Bash", "input": command}]
        if "FINDTOOLS" in joined:
            query = {"query": "select:SendMessage,ListAgents", "max_results": 5}
            return [{"type": "tool_use", "id": call, "name": "ToolSearch", "input": query}]
        if "BGJOB" in joined and RETRY not in joined:
            job = {
                "command": f"sleep {JOB_SECONDS}; echo done",
                "description": "Job",
                "run_in_background": True,
            }
            return [{"type": "tool_use", "id": call, "name": "Bash", "input": job}]
        if "ASK" in joined:
            question = {
                "question": "Which one?",
                "header": "Pick",
                "options": [
                    {"label": "A", "description": "The first"},
                    {"label": "B", "description": "The second"},
                ],
                "multiSelect": False,
            }
            return [
                {
                    "type": "tool_use",
                    "id": call,
                    "name": "AskUserQuestion",
                    "input": {"questions": [question]},
                }
            ]
        return [text(f"Answer {answers}")]

    def last_user(self) -> list[str]:
        """The texts of the last user message of the newest request."""
        messages = self.bodies[-1].get("messages") or [] if self.bodies else []
        return _texts(messages[-1]) if messages else []

    def system(self) -> str:
        """The system prompt of the newest request, as one text."""
        system = self.bodies[-1].get("system") if self.bodies else None
        if isinstance(system, str):
            return system
        return " ".join(str(b.get("text")) for b in system or [] if isinstance(b, Mapping))

    def tools(self) -> list[str]:
        """The tool names of the newest request."""
        return [str(t.get("name")) for t in (self.bodies[-1].get("tools") or [])]

    def tool_result(self, word: str) -> str:
        """The newest tool result of the conversation that holds ``word``, as one text."""
        for body in reversed(self.bodies):
            messages = body.get("messages") or []
            if not any(word in t for m in messages for t in _texts(m)):
                continue
            for message in reversed(messages):
                for block in message.get("content") or []:
                    if isinstance(block, Mapping) and block.get("type") == "tool_result":
                        return json.dumps(block.get("content"), ensure_ascii=False)
        return ""

    def environment(self, word: str) -> dict[str, str]:
        """What the ``ENVSTEP`` command of the conversation that holds ``word`` printed."""
        return _variables(json.loads(self.tool_result(word) or "null"))

    def deferred(self) -> list[str]:
        """The tools the newest request names as loadable later (the CLI's own reminder)."""
        dumped = json.dumps(self.bodies[-1], ensure_ascii=False) if self.bodies else ""
        found = dumped.partition("deferred tools are now available via ToolSearch")[2]
        listed = found.partition("before calling them:")[2].partition("</system-reminder>")[0]
        return [name for name in listed.replace("\\n", "\n").split() if name.isidentifier()]

    def asked(self, word: str, since: int = 0) -> Mapping[str, Any]:
        """The one request without tools since ``since`` that a session made for ``word``.

        The CLI's own requests for agent view carry the conversation too; a session's own
        request is known by the system prompt of the launch.
        """
        [body] = [
            b
            for b in self.untooled[since:]
            if word in json.dumps(b.get("messages"))
            and "SYSTEM-MARKER" in json.dumps(b.get("system"))
        ]
        return body


def _texts(message: Mapping[str, Any]) -> list[str]:
    content = message.get("content")
    if isinstance(content, str):
        return [content]
    out = []
    for block in content or []:
        if block.get("type") == "text":
            out.append(str(block.get("text")))
        elif block.get("type") == "tool_result":
            out.append("<tool_result>")
    return out


def _variables(content: object) -> dict[str, str]:
    """The ``NAME=value`` lines of a tool result."""
    if isinstance(content, list):
        content = "\n".join(str(b.get("text")) for b in content if isinstance(b, Mapping))
    out: dict[str, str] = {}
    for row in str(content or "").splitlines():
        name, found, value = row.partition("=")
        if found and name.replace("_", "").isalnum():
            out[name] = value
    return out


def turn_ends(records: Sequence[Mapping[str, Any]]) -> int:
    """How many turns of a transcript ended: a ``turn_duration``, or an API error record.

    2.1.326 writes no ``turn_duration`` after an API error; one right after it is the same end.
    """
    count, after_error = 0, False
    for record in records:
        if record.get("type") == "assistant":
            after_error = record.get("isApiErrorMessage") is True
            count += after_error
        elif record.get("subtype") == "turn_duration":
            count += not after_error
            after_error = False
    return count


def path_facts(records: Sequence[Mapping[str, Any]]) -> tuple[set[str], set[str], bool | None]:
    """Which path a turn took, read from its transcript records (09 18.2, 18.3).

    Args:
        records: The records of a session transcript.

    Returns:
        The values of ``entrypoint`` and of ``sessionKind`` in the ``user`` and ``assistant``
        records (a background turn writes ``cli`` and ``bg``, a print turn ``sdk-cli``), and
        whether every request used the stored login as the runner's counter-check reads it;
        ``None`` without a request record.
    """
    spoken = [r for r in records if r.get("type") in ("user", "assistant")]
    mapper = TranscriptMapper("")
    requests = [m for r in records for m in mapper.map(r) if m.get("subtype") == "bg_request"]
    login = all(m["login"] is True for m in requests) if requests else None
    return (
        {str(r.get("entrypoint")) for r in spoken},
        {str(r.get("sessionKind")) for r in spoken},
        login,
    )


def frame(value: str, tool_class: str = "scoped") -> tuple[str, bytes]:
    """Return the ``uuid`` and the inbox line of one input of the runner for a tool class."""
    line, line_uuid = user_line([value])
    converted = socket_frame(line, sender_class=sender_class(tool_class))
    assert converted is not None
    return line_uuid, converted


class World:
    """One isolated configuration folder with its own supervisor and session list.

    Args:
        root: A short folder of the caller without a ``CLAUDE.md`` or ``.claude`` above it.
        program: The pinned CLI.
        url: The base URL of the stand-in.
        login_token: Give the CLI `DUMMY_TOKEN` in the place of the key helper, with a
            `Refuser` as its proxy for every host but loopback (see the module docstring).

    Attributes:
        env: The fixed environment of the folder: the one every control command and every
            launch gets, and so the one of the supervisor. It carries no value of a session.
        refuser: The proxy of a world with ``login_token``; ``None`` otherwise.
    """

    def __init__(self, root: Path, program: str, url: str, *, login_token: bool = False) -> None:
        """Create the folders, the key helper and the trust entry of the working folder."""
        self.root = root
        self.program = program
        self.work = (root / "work").resolve()
        self.work.mkdir(parents=True, exist_ok=True)
        (root / "tmp").mkdir(exist_ok=True)
        self.env = {**cli_env(root, url), "TMPDIR": str(root / "tmp")}
        self.config = root / "config"
        self.refuser: Refuser | None = None
        settings: dict[str, Any] = {"apiKeyHelper": f"/bin/echo {DUMMY_KEY}"}
        if login_token:
            self.refuser = Refuser()
            settings = {}
            del self.env["ANTHROPIC_API_KEY"]
            self.env.update(
                CLAUDE_CODE_OAUTH_TOKEN=DUMMY_TOKEN,
                HTTPS_PROXY=self.refuser.url,
                HTTP_PROXY=self.refuser.url,
                NO_PROXY="127.0.0.1,localhost",
            )
        (self.config / "settings.json").write_text(json.dumps(settings), encoding="utf-8")
        self.trust(self.work)
        self.cli = SessionCli(program, self.env, self.work, WAIT)
        self.sessions = SessionList(self.cli, 0.2)
        self.started: list[str] = []
        self.classes: dict[str, str] = {}

    def trust(self, folder: Path) -> None:
        """Record workspace trust for a folder, as the documented manual entry does."""
        entry = {str(folder): {"hasTrustDialogAccepted": True}}
        self._record(lambda doc: doc.setdefault("projects", {}).update(entry))

    def accept_bypass(self) -> None:
        """Record the acceptance of bypass mode in this folder's own configuration.

        What the operator does once in their own CLI (``claude --dangerously-skip-permissions``);
        the runner never writes it. Here it stands in the configuration folder of the test.
        """
        self._record(lambda doc: doc.update(bypassPermissionsModeAccepted=True))

    def _record(self, change: Any) -> None:
        path = self.config / ".claude.json"
        doc = json.loads(path.read_text("utf-8")) if path.exists() else {}
        change(doc)
        path.write_text(json.dumps(doc), encoding="utf-8")

    def socket(self, name: str) -> Path:
        """The inbox socket of the session ``name``."""
        return self.root / f"{name}.sock"

    async def launch(
        self,
        name: str,
        prompt: str,
        *,
        resume: str | None = None,
        tool_class: str = "scoped",
        tools: Sequence[str] = ("Bash",),
        cwd: Path | None = None,
        add_dir: Path | None = None,
        system: str = "SYSTEM-MARKER of the test.",
        extra: Sequence[str] = (),
        run_env: Mapping[str, str] | None = None,
        without: Sequence[str] = (),
        hosted: HostedArgs | None = None,
        plugin_dirs: Sequence[Path] = (),
        drop_settings: Sequence[str] = (),
    ) -> LaunchOutput:
        """Launch one session with the runner's own argument list and settings file.

        The launch command always gets the fixed environment of the folder, as the runner's
        launch does; ``run_env`` is the environment of the session's run, of which only the
        ``env`` block of the settings file carries anything to the session. ``without`` names
        arguments to leave out of the runner's list, for a session that shows what one of
        them changes; ``drop_settings`` names members of the runner's settings file to leave
        out in the same way.
        """
        prompt_file = self.root / f"{name}.prompt.md"
        prompt_file.write_text(system, encoding="utf-8")
        settings = self.root / f"{name}.settings.json"
        turn_env = {"TMPDIR": str(self.root / "tmp")} if run_env is None else dict(run_env)
        base = None
        if hosted is not None:
            base = json.loads(hosted.settings.read_text("utf-8"))
        # The runner's turn environment names the CLI's configuration folder: the folder of
        # the world, unless ``run_env`` names another.
        document = settings_document({"CLAUDE_CONFIG_DIR": str(self.config), **turn_env}, base)
        for member in drop_settings:
            del document[member]
        settings.write_text(json.dumps(document), encoding="utf-8")
        self.classes[name] = tool_class
        argv = background_argv(
            claude_bin=self.program,
            model=MODEL,
            resume=resume,
            name=f"bothy-{name}",
            tool_class=tool_class,
            tools=list(tools),
            add_dir=add_dir,
            system_prompt_file=prompt_file,
            socket_file=self.socket(name),
            settings_file=settings,
            hosted=hosted,
            plugin_dirs=plugin_dirs,
        )
        out = await launch(
            [*(a for a in argv if a not in without), *extra],
            prompt=prompt,
            cwd=cwd or self.work,
            env=self.env,
            stdout=self.root / f"{name}.out",
            stderr=self.root / f"{name}.err",
            timeout=WAIT,
        )
        if out.short_id is not None:
            self.started.append(out.short_id)
        return out

    def output(self, name: str) -> tuple[str, str]:
        """What the launch command of ``name`` printed: stdout and stderr."""
        return (
            (self.root / f"{name}.out").read_text("utf-8"),
            (self.root / f"{name}.err").read_text("utf-8"),
        )

    async def entry(self, short_id: str) -> SessionEntry | None:
        """The session's entry in a new reading of the list."""
        reading = await self.sessions.read(after=self.sessions.now())
        return reading.entry(short_id) if reading is not None else None

    async def until_entry(self, short_id: str, *, pid: bool = True) -> SessionEntry:
        """Wait until the session has an entry (with a process, unless ``pid`` is false)."""
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            entry = await self.entry(short_id)
            if entry is not None and (entry.pid is not None or not pid):
                return entry
            await asyncio.sleep(0.2)
        raise AssertionError(f"the session {short_id} got no entry with a process")

    async def until_status(self, short_id: str, status: str) -> SessionEntry:
        """Wait until the list shows the session with ``status``."""
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            entry = await self.entry(short_id)
            if entry is not None and entry.status == status:
                return entry
            await asyncio.sleep(0.2)
        raise AssertionError(f"the session {short_id} did not become {status}")

    def transcript(self, session_id: str) -> Path | None:
        """The session's transcript, once the CLI created it."""
        return find_transcript(self.config, session_id)

    def records(self, session_id: str) -> list[dict[str, Any]]:
        """The complete records of the session's transcript so far."""
        path = self.transcript(session_id)
        if path is None:
            return []
        written = path.read_bytes()
        lines = written[: written.rfind(b"\n") + 1].decode("utf-8").splitlines()
        return [json.loads(x) for x in lines if x.startswith("{")]

    async def until_turns(self, session_id: str, count: int) -> list[dict[str, Any]]:
        """Wait until the transcript holds ``count`` turn ends and return its records."""
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            records = self.records(session_id)
            if turn_ends(records) >= count:
                return records
            await asyncio.sleep(0.1)
        raise AssertionError(f"no {count} turn ends in the transcript of {session_id}")

    async def until_record(self, session_id: str, found: Any) -> list[dict[str, Any]]:
        """Wait until a record of the transcript satisfies ``found``."""
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            records = self.records(session_id)
            if any(found(r) for r in records):
                return records
            await asyncio.sleep(0.1)
        raise AssertionError(f"the awaited record is not in the transcript of {session_id}")

    async def send(self, name: str, value: str) -> str:
        """Write one input of the runner to the inbox socket of ``name``; its ``uuid``.

        The line is the one the runner writes for the tool class the session was launched
        with (`claude_code_runner.bgsession.sender_class`).
        """
        line_uuid, line = frame(value, self.classes.get(name, "scoped"))
        written = await asyncio.to_thread(send_frames, self.socket(name), [line], 10.0)
        assert written, "the inbox socket did not take the line"
        return line_uuid

    async def send_raw(self, name: str, content: str) -> str:
        """Write a line of another sender to the inbox socket of ``name``; its ``uuid``.

        The content is written as it is, so a test can state a class in the CLI's envelope
        or none at all, whatever the session's own class is.
        """
        line_uuid = str(uuid.uuid4())
        doc = {"type": "user", "uuid": line_uuid, "message": {"role": "user", "content": content}}
        line = json.dumps(doc, ensure_ascii=False).encode("utf-8") + b"\n"
        written = await asyncio.to_thread(send_frames, self.socket(name), [line], 10.0)
        assert written, "the inbox socket did not take the line"
        return line_uuid

    async def stop(self, short_id: str) -> bool:
        """``claude stop`` and the wait until the session's process is gone.

        The process writes its session totals (``cost-state``) right before it ends, which
        can be a moment after the command returned.
        """
        entry = await self.entry(short_id)
        stopped = await self.cli.stop(short_id)
        deadline = time.monotonic() + WAIT
        while entry is not None and entry.pid is not None and time.monotonic() < deadline:
            try:
                os.kill(entry.pid, 0)
            except ProcessLookupError:
                break
            await asyncio.sleep(0.02)
        return stopped

    async def _socket_folder(self) -> str | None:
        """The folder of the supervisor's sockets, as ``claude daemon status`` names it."""
        _, out, _ = await self.cli.run("daemon", "status")
        for line in out.splitlines():
            name, _, value = line.strip().partition(":")
            if name == "sock dir" and value.strip():
                return value.strip()
        return None

    @staticmethod
    def _hosts(socket_folder: str | None) -> int:
        """The processes of this folder's supervisor that still live (its session hosts).

        The supervisor keeps a spare host process for the next session; it outlives the
        supervisor for some seconds and names a socket below the folder on its command line.
        """
        if socket_folder is None:
            return 0
        program = shutil.which("ps") or "/bin/ps"
        listed = subprocess.run(
            [program, "-axo", "command"], capture_output=True, text=True, check=False
        ).stdout
        return sum(1 for line in listed.splitlines() if socket_folder in line)

    async def close(self) -> None:
        """Stop and remove every session of this folder and wait for its supervisor to end.

        No session, no supervisor and no host process of a test or a recording is left
        running.
        """
        try:
            await self._end()
        finally:
            if self.refuser is not None:
                self.refuser.close()

    async def _end(self) -> None:
        reading = await self.sessions.read(after=self.sessions.now())
        listed = {e.short_id for e in reading.entries} if reading is not None else set()
        for short_id in sorted(listed | set(self.started)):
            await self.cli.stop(short_id)
            await self.cli.remove(short_id)
        socket_folder = await self._socket_folder()
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            readable, supervisor = await self.cli.supervisor()
            if readable and supervisor is None and not self._hosts(socket_folder):
                return
            await asyncio.sleep(0.5)
        _, supervisor = await self.cli.supervisor()
        if supervisor is not None and supervisor.pid is not None:
            # Only the supervisor of this folder, which no session of anyone else uses.
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.kill(supervisor.pid, signal.SIGTERM)
        raise AssertionError("the supervisor of the test folder did not end by itself")


def new_root(base: Path) -> Path:
    """A short folder for one `World` under ``base`` (socket paths are bounded)."""
    root = base / f"bg{uuid.uuid4().hex[:6]}"
    root.mkdir(parents=True)
    root.chmod(0o700)  # the files of a launch are private, like the runner's state
    return root
