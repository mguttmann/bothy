# SPDX-License-Identifier: Apache-2.0
"""The pinned CLI after a refused request: the process, its work and the next line (09 18.9).

Collected only with ``CLAUDE_CODE_RUNNER_REAL_CLI=1`` (``tests/conftest.py``); free: no login and no
model call. The CLI runs against a local stand-in for the API on ``127.0.0.1``
(``real_cli/stand_in.py``: ``ANTHROPIC_BASE_URL`` on loopback, a dummy key, its own
``CLAUDE_CONFIG_DIR``).

The retry after an account error (FIX-RUNNER-LIMIT-KEEPS-RUN) rests on three things the CLI
does, which these tests run:

1. A request the API refuses with ``429`` ends the turn with an API error message whose member
   ``error`` is ``rate_limit`` and an error ``result``; the process stays, and so does the
   background task an earlier turn started.
2. A further input line then starts a turn of its own in the same process, and it is answered.
3. The request of that turn still carries the input of the failed turn, in the same user
   message as the new line: a message of a person, or the notification of a background task
   for which the CLI had started the failed turn by itself. So the retry line of the runner
   makes the model answer what was left without an answer.

The stand-in answers the refused request with ``x-should-retry: false``, as the API does for a
used-up usage window: without it the CLI tries a ``429`` again by itself for minutes before it
gives the error to the turn.
"""

from __future__ import annotations

import contextlib
import json
import os
import shutil
import signal
import subprocess
import tempfile
import time
import uuid
from collections.abc import Callable, Iterator, Mapping
from pathlib import Path
from typing import Any

from real_cli.stand_in import Answer, ApiError, cli_env, stand_in, text, user_line
from real_cli.test_real_cli import claude_bin

from claude_code_runner.account import AccountError, account_error, retry_input

WAIT = 120.0
"""The bound of every wait of this test (seconds)."""
QUESTION = "FOLLOW-UP-WITHOUT-AN-ANSWER: what is the state?"
RETRY = retry_input(AccountError("usage_limit"))
"""The runner's own retry line."""
REFUSED = ApiError(
    429,
    "rate_limit_error",
    "This request would exceed your account's rate limit.",
    headers={"x-should-retry": "false"},
)


class _Api:
    """The answers of the stand-in: a background command, texts, and what a test refuses.

    Args:
        job: The background command of the first turn.
        refuse: Tells for the last user message of a request whether the API refuses it.
    """

    def __init__(self, job: str, refuse: Callable[[list[str]], bool]) -> None:
        self.marker = f"job-{uuid.uuid4().hex}"
        """A text in the command line of the background command, to find what is left of it."""
        self.job = f"{job} # {self.marker}"
        self.refuse = refuse
        self.last_user: list[str] = []

    def __call__(self, body: Mapping[str, Any]) -> Answer:
        if not body.get("tools"):
            return [text("Ok.")]  # a request of the CLI itself, not of the conversation
        messages = body.get("messages") or []
        self.last_user = _texts(messages[-1]) if messages else []
        answers = sum(1 for m in messages if m.get("role") == "assistant")
        if answers == 0:
            job = {"command": self.job, "description": "Job", "run_in_background": True}
            return [{"type": "tool_use", "id": "toolu_01", "name": "Bash", "input": job}]
        if answers == 1:
            return [text("Started.")]
        if self.refuse(self.last_user):
            return REFUSED
        return [text("Answered.")]


def _texts(message: Mapping[str, Any]) -> list[str]:
    content = message.get("content")
    if isinstance(content, str):
        return [content]
    return [str(b.get("text")) for b in content or [] if b.get("type") == "text"]


def _is_retry(texts: list[str]) -> bool:
    return any(t == RETRY for t in texts)


class _Cli:
    """One ``claude -p`` process with stream-json on both sides, as the runner starts a turn."""

    def __init__(self, root: Path, url: str) -> None:
        self.out = root / "stream.jsonl"
        argv = [
            claude_bin(),
            "-p",
            "--input-format",
            "stream-json",
            "--output-format",
            "stream-json",
            "--verbose",
            "--replay-user-messages",
            "--session-id",
            str(uuid.uuid4()),
            "--model",
            "claude-haiku-4-5",
            "--permission-mode",
            "bypassPermissions",
            "--permission-prompts",
            "none",
        ]
        with self.out.open("wb") as stdout:
            self.process = subprocess.Popen(
                argv,
                cwd=root,
                env=cli_env(root, url),
                stdin=subprocess.PIPE,
                stdout=stdout,
                stderr=subprocess.DEVNULL,
                start_new_session=True,  # a group of its own, as the runner starts a turn
            )

    def write(self, value: str) -> str:
        """Write one user line and return its ``uuid``."""
        line_uuid, line = user_line(value)
        assert self.process.stdin is not None
        self.process.stdin.write(line)
        self.process.stdin.flush()
        return line_uuid

    def messages(self) -> list[dict[str, Any]]:
        return [json.loads(x) for x in self.out.read_text().splitlines() if x.startswith("{")]

    def results(self, count: int) -> list[dict[str, Any]]:
        """Wait for ``count`` results and return them."""
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            results = [m for m in self.messages() if m.get("type") == "result"]
            if len(results) >= count:
                return results
            time.sleep(0.1)
        raise AssertionError(f"no {count} results; stream: {self.out.read_text()[-2000:]}")

    def running_tasks(self) -> int:
        lists = [m for m in self.messages() if m.get("subtype") == "background_tasks_changed"]
        return len(lists[-1].get("tasks") or []) if lists else 0

    def end(self, marker: str) -> None:
        """End the CLI as a stop of the runner does (09 18.6), then what is left of its job.

        The CLI runs a background command in a process group of its own, which a signal to
        the CLI's group does not reach; only a group whose command line carries the test's
        own marker is ended here.
        """
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(self.process.pid, signal.SIGINT)
        try:
            self.process.wait(timeout=20)
        except subprocess.TimeoutExpired:
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.killpg(self.process.pid, signal.SIGKILL)
            self.process.wait()
        if self.process.stdin is not None:
            with contextlib.suppress(OSError):
                self.process.stdin.close()
        pgrep = shutil.which("pgrep")
        if pgrep is None:
            return
        found = subprocess.run([pgrep, "-f", marker], capture_output=True, text=True, check=False)
        for pid in (int(x) for x in found.stdout.split()):
            with contextlib.suppress(ProcessLookupError, PermissionError):
                group = os.getpgid(pid)
                if group != os.getpgid(0):
                    os.killpg(group, signal.SIGKILL)


@contextlib.contextmanager
def _session(api: _Api) -> Iterator[tuple[_Cli, Any]]:
    """The CLI against the stand-in with ``api`` as its script, ended afterwards."""
    with tempfile.TemporaryDirectory() as tmp, stand_in(api) as (url, handler):
        cli = _Cli(Path(tmp), url)
        try:
            yield cli, handler
        finally:
            cli.end(api.marker)


def _the_error(cli: _Cli) -> dict[str, Any]:
    [message] = [m for m in cli.messages() if m.get("is_api_error_message") is True]
    return message


def test_a_refused_request_keeps_the_process_and_the_next_line_is_answered() -> None:
    api = _Api("sleep 300", lambda last: QUESTION in last and not _is_retry(last))
    with _session(api) as (cli, handler):
        cli.write("Start the job in the background, then answer.")
        cli.results(1)
        assert cli.running_tasks() == 1  # the turn left a background task
        cli.write(QUESTION)
        failed = cli.results(2)[1]
        assert failed.get("is_error") is True
        # 1. The member the runner reads: the class of the error, never its text.
        assert account_error(_the_error(cli)) == AccountError("usage_limit")
        time.sleep(2.0)
        assert cli.process.poll() is None  # the process stays, its stdin open
        assert cli.running_tasks() == 1  # and the task was not stopped
        # 2. The runner's retry line starts a turn of its own, and it is answered.
        retry_uuid = cli.write(RETRY)
        answered = cli.results(3)[2]
        assert (answered.get("is_error"), answered.get("result")) == (False, "Answered.")
        named = answered.get("user_message_uuids") or [answered.get("user_message_uuid")]
        assert retry_uuid in named
        assert answered.get("num_turns") != 0  # a turn with a request to the model
        # 3. Its request carried the input the failed turn left without an answer.
        assert QUESTION in api.last_user
        assert _is_retry(api.last_user)
        assert cli.process.poll() is None
        assert cli.running_tasks() == 1
        assert handler.answers == ["Bash", "Started.", "error 429", "Answered."]


def test_a_refused_notification_turn_is_answered_by_the_retry_line() -> None:
    # The reported failure: the job ends, the CLI starts a turn by itself for the task's
    # notification, and the request of that turn is refused. The task list is empty by then.
    api = _Api("sleep 2", lambda last: not _is_retry(last))
    with _session(api) as (cli, handler):
        cli.write("Start the job in the background, then answer.")
        first, failed = cli.results(2)  # the answer, then the notification turn
        assert (first.get("is_error"), first.get("result")) == (False, "Started.")
        assert failed.get("is_error") is True
        assert account_error(_the_error(cli)) == AccountError("usage_limit")
        assert cli.running_tasks() == 0
        notified = [m for m in cli.messages() if m.get("subtype") == "task_notification"]
        assert [m.get("status") for m in notified] == ["completed"]
        time.sleep(2.0)
        assert cli.process.poll() is None  # the process stays without a task, its stdin open
        cli.write(RETRY)
        answered = cli.results(3)[2]
        assert (answered.get("is_error"), answered.get("result")) == (False, "Answered.")
        assert answered.get("num_turns") != 0
        # The request of the retry carried the notification the failed turn was started for.
        assert _is_retry(api.last_user)
        assert any("<task-notification>" in t for t in api.last_user)
        assert handler.answers == ["Bash", "Started.", "error 429", "Answered."]
