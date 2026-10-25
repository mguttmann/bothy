# SPDX-License-Identifier: Apache-2.0
"""The agents of the desktop E2E test: the Fake-Agent of the Python SDK, driven by a script.

Brief M4-34 Design note 1: the service ``e2e-agent`` of ``infra/compose/test/compose.e2e.yaml``
runs this file. For each agent of ``e2e-agent.json`` (written by ``bothy-smoke e2e-world``) it
starts one Fake-Agent process (``python -m bothy_sdk.testing.fake_agent``, docs/spec/09-agents.md
16.1) and drives it over the JSON-line control protocol of ``bothy_sdk.testing.control``, the way
the conductor of the agent conformance suite does, so this file needs nothing but the standard
library.

The two roles:

* ``support-agent`` answers every mention with the whole run of the suite: the status line, two
  steps, waiting for a follow-up of the person (``agent.followup``, marked delivered and absorbed),
  an approval card, waiting for the click (``interaction.created``, answered), a reply and the end
  of the run.
* ``stream-agent`` answers every mention with a stream of status line patches and ends the run;
  the performance level measures the time from a patch to the drawn frame with it.

Patches go out at most once per ``agent.run_update_min_interval`` (500 ms, 09 5.3), the throttle
every adapter keeps. Inputs: ``--agents FILE`` (the credentials, mode 0600, never printed) and
``--base-url URL`` (server.public_url plus ``/api/v1``). The process runs until it is stopped; a
failed op ends it with exit code 1 and the op and error kind on stderr, never a token.

Examples:
    >>> sorted(ROLES)
    ['stream-agent', 'support-agent']
"""

from __future__ import annotations

import argparse
import asyncio
import json
import os
import secrets
import sys
import time
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

# The throttle of an adapter (agent.run_update_min_interval, 09 5.3).
PATCH_INTERVAL_S = 0.5
# Patches of one run of stream-agent: the samples of patch-to-frame per run (15 19).
STREAM_PATCHES = 30
# Texts of the run of support-agent; the suite waits for them.
STATUS_LINE = "Reading the thread"
APPROVAL_TEXT = "May I post the summary?"
APPROVED_TEXT = "Approved"
REPLY_TEXT = "Here is the summary."
# Settings of the SDK in the Fake-Agent: short waits, the REST transport (09 4.1).
SETTINGS = {"sdk.poll_interval": "100ms", "sdk.retry_backoff_base": "50ms"}
# Wait between two polls that found nothing (the REST receive answers ``none`` then).
POLL_INTERVAL_S = 0.1

Call = Callable[[str, dict[str, Any] | None], Awaitable[dict[str, Any]]]
Sleep = Callable[[float], Awaitable[None]]


class AgentError(Exception):
    """An op of the Fake-Agent failed or answered with an unexpected status."""


def format_now(now: datetime) -> str:
    """Format a time the way the control protocol takes it: RFC 3339 UTC with milliseconds.

    Args:
        now: A timezone aware time.

    Returns:
        The text, for example ``2030-01-01T00:00:00.000Z``.
    """
    utc = now.astimezone(UTC)
    return utc.strftime("%Y-%m-%dT%H:%M:%S.") + f"{utc.microsecond // 1000:03d}Z"


def uuid7() -> str:
    """Return a fresh UUIDv7 (RFC 9562 5.7), the form of a step ID of the run document.

    Returns:
        The canonical lowercase text.
    """
    value = (time.time_ns() // 1_000_000) << 80 | secrets.randbits(80)
    value = value & ~(0xF << 76) | 0x7 << 76
    value = value & ~(0x3 << 62) | 0x2 << 62
    text = f"{value:032x}"
    return f"{text[:8]}-{text[8:12]}-{text[12:16]}-{text[16:20]}-{text[20:]}"


def _ok(reply: dict[str, Any], op: str, *, status: int | None = None) -> dict[str, Any]:
    """The result of a reply; an error reply or another status raises `AgentError`."""
    if not reply.get("ok"):
        error = reply.get("error") or {}
        raise AgentError(f"{op}: {error.get('kind', 'error')} {error.get('detail', '')}".strip())
    result = reply.get("result") or {}
    if not isinstance(result, dict):
        raise AgentError(f"{op}: the result is no object")
    if status is not None and result.get("status") != status:
        raise AgentError(f"{op}: status {result.get('status')} {result.get('code', '')}".strip())
    return result


class AgentProcess:
    """One Fake-Agent process and its control protocol on stdin and stdout.

    Args:
        process: The started child process with pipes for stdin and stdout.
    """

    def __init__(self, process: asyncio.subprocess.Process) -> None:
        """Wrap the process; see the class docstring."""
        self._process = process
        self._next = 0

    @classmethod
    async def start(cls, argv: list[str]) -> AgentProcess:
        """Start the Fake-Agent with ``argv`` and return its handle.

        Args:
            argv: The command line, for example the interpreter with ``-m`` and the module.

        Returns:
            The handle.
        """
        process = await asyncio.create_subprocess_exec(
            *argv, stdin=asyncio.subprocess.PIPE, stdout=asyncio.subprocess.PIPE
        )
        return cls(process)

    async def call(self, op: str, args: dict[str, Any] | None = None) -> dict[str, Any]:
        """Send one request and return its reply.

        Args:
            op: The op, for example ``receive``.
            args: Its arguments.

        Returns:
            The reply object with ``ok`` and ``result`` or ``error``.

        Raises:
            AgentError: The process closed its output.
        """
        stdin, stdout = self._process.stdin, self._process.stdout
        if stdin is None or stdout is None:
            raise AgentError(f"{op}: the process has no pipes")
        self._next += 1
        request = {
            "id": self._next,
            "op": op,
            "args": args or {},
            "now": format_now(datetime.now(UTC)),
        }
        stdin.write((json.dumps(request) + "\n").encode())
        await stdin.drain()
        line = await stdout.readline()
        if not line:
            raise AgentError(f"{op}: the Fake-Agent ended")
        reply = json.loads(line)
        if not isinstance(reply, dict):
            raise AgentError(f"{op}: the reply is no object")
        return reply

    async def close(self) -> None:
        """Close stdin, which ends the Fake-Agent, and wait for it."""
        if self._process.stdin is not None:
            self._process.stdin.close()
        await self._process.wait()


async def connect(call: Call, *, base_url: str, instance_id: str, agent: dict[str, Any]) -> None:
    """Send ``hello`` with the credentials of ``agent`` and ``connect``.

    Args:
        call: The control call of the agent's process.
        base_url: The API base, server.public_url plus ``/api/v1``.
        instance_id: The instance ID.
        agent: One entry of ``e2e-agent.json``.
    """
    hello = {
        "base_url": base_url,
        "gateway_url": base_url,
        "instance_id": instance_id,
        "agent_id": agent["agent_id"],
        "consumer": "default",
        "transport": "rest",
        "credentials": {"token": agent["token"], "dpop_private_jwk": agent["dpop_private_jwk"]},
        "runtime": {"billing_mode": "organization", "tool_class": "chat", "model": "model-test"},
        "settings": SETTINGS,
    }
    _ok(await call("hello", hello), "hello")
    _ok(await call("connect", None), "connect", status=200)


async def _patch(call: Call, sleep: Sleep, run: str, ops: list[dict[str, Any]]) -> None:
    """One patch of the run document, then the throttle of the adapter."""
    _ok(await call("patch", {"run": run, "ops": ops}), "patch", status=200)
    await sleep(PATCH_INTERVAL_S)


async def receive(call: Call, sleep: Sleep, args: dict[str, Any], what: str) -> dict[str, Any]:
    """Poll ``receive`` with ``args`` until an event arrives and return it.

    Over the REST transport the Fake-Agent answers ``{"none": true}`` when the consumer has no
    matching event yet; this waits ``POLL_INTERVAL_S`` and asks again.

    Args:
        call: The control call of the agent's process.
        sleep: The wait between two polls.
        args: The arguments of ``receive`` (``type``, ``run``).
        what: The name of the event in an error.

    Returns:
        The result of the reply that carried the event.
    """
    while True:
        result = _ok(await call("receive", args), f"receive {what}")
        if not result.get("none"):
            if "status" in result and "event" not in result:
                raise AgentError(
                    f"receive {what}: status {result.get('status')} {result.get('code', '')}"
                )
            return result
        await sleep(POLL_INTERVAL_S)


async def _invocation(call: Call, sleep: Sleep) -> tuple[str, str]:
    """Wait for the next mention and start its run; return the run and thread IDs."""
    invocation = await receive(call, sleep, {"type": "agent.invocation"}, "invocation")
    if invocation.get("verdict") != "valid":
        raise AgentError(f"receive invocation: verdict {invocation.get('verdict')}")
    run, thread = str(invocation["run_id"]), str(invocation["thread_id"])
    _ok(await call("start", {"run": run}), "start", status=200)
    return run, thread


def _step(ordinal: int, name: str, title: str) -> dict[str, Any]:
    """The add operation of a running tool step (04 11.1)."""
    step_id = uuid7()
    return {
        "op": "add",
        "path": f"/steps/{step_id}",
        "value": {
            "id": step_id,
            "ordinal": ordinal,
            "kind": "tool_call",
            "name": name,
            "title": title,
            "status": "running",
            "started_at": format_now(datetime.now(UTC)),
        },
    }


async def support_run(call: Call, sleep: Sleep) -> str:
    """Play one run of support-agent (see the module docstring); return the run ID.

    Args:
        call: The control call of the agent's process.
        sleep: The wait of the throttle.

    Returns:
        The run ID.
    """
    run, thread = await _invocation(call, sleep)
    await _patch(call, sleep, run, [{"op": "add", "path": "/status_line", "value": STATUS_LINE}])
    await _patch(call, sleep, run, [_step(1, "search", "Search the notes")])
    await _patch(call, sleep, run, [_step(2, "read", "Read the thread")])
    followup = await receive(call, sleep, {"type": "agent.followup", "run": run}, "followup")
    message = str(followup["message_id"])
    for state in ("delivered", "absorbed"):
        op = {"op": "replace", "path": f"/followups/{message}/state", "value": state}
        await _patch(call, sleep, run, [op])
    card = {"thread": thread, "run": run, "content": APPROVAL_TEXT, "card": "approval"}
    _ok(await call("post", card), "post card", status=201)
    click = await receive(call, sleep, {"type": "interaction.created", "run": run}, "click")
    answer = {"interaction": click["interaction_id"], "run": run, "text": APPROVED_TEXT}
    _ok(await call("respond", answer), "respond", status=200)
    reply = {"thread": thread, "run": run, "content": REPLY_TEXT}
    _ok(await call("post", reply), "post reply", status=201)
    _ok(await call("finish", {"run": run, "state": "completed"}), "finish", status=200)
    return run


async def stream_run(call: Call, sleep: Sleep, patches: int = STREAM_PATCHES) -> str:
    """Play one run of stream-agent: ``patches`` status line patches, then the end.

    Args:
        call: The control call of the agent's process.
        sleep: The wait of the throttle.
        patches: The number of patches.

    Returns:
        The run ID.
    """
    run, _ = await _invocation(call, sleep)
    for n in range(1, patches + 1):
        op = {"op": "add", "path": "/status_line", "value": f"Streaming {n} of {patches}"}
        await _patch(call, sleep, run, [op])
    _ok(await call("finish", {"run": run, "state": "completed"}), "finish", status=200)
    return run


ROLES: dict[str, Callable[[Call, Sleep], Awaitable[str]]] = {
    "support-agent": support_run,
    "stream-agent": stream_run,
}


async def serve(
    call: Call, role: str, sleep: Sleep = asyncio.sleep, runs: int | None = None
) -> int:
    """Play the runs of ``role`` one after the other.

    Args:
        call: The control call of the agent's process (after `connect`).
        role: A key of `ROLES`.
        sleep: The wait of the throttle.
        runs: Stop after this many runs; ``None`` runs until the process is stopped.

    Returns:
        The number of runs played.
    """
    play = ROLES[role]
    done = 0
    while runs is None or done < runs:
        await play(call, sleep)
        done += 1
    return done


async def run_agents(agents_file: Path, base_url: str, argv: list[str]) -> None:
    """Start one Fake-Agent per agent of ``agents_file`` and serve them until one fails.

    Args:
        agents_file: ``e2e-agent.json``.
        base_url: The API base.
        argv: The command line of a Fake-Agent process.
    """
    data = json.loads(agents_file.read_text(encoding="utf-8"))
    processes: list[AgentProcess] = []
    tasks = []
    try:
        for agent in data["agents"]:
            process = await AgentProcess.start(argv)
            processes.append(process)
            await connect(
                process.call, base_url=base_url, instance_id=data["instance_id"], agent=agent
            )
            print(f"e2e-agent: ready {agent['handle']}", file=sys.stderr, flush=True)
            tasks.append(asyncio.create_task(serve(process.call, agent["handle"])))
        done, _ = await asyncio.wait(tasks, return_when=asyncio.FIRST_EXCEPTION)
        for task in done:
            task.result()
    finally:
        for task in tasks:
            task.cancel()
        for process in processes:
            await process.close()


def main(args: list[str] | None = None) -> int:
    """Parse the flags and run the agents; return the exit code.

    Args:
        args: The flags (``sys.argv[1:]`` by default).

    Returns:
        0 when stopped, 1 after a failed op.
    """
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--agents", type=Path, required=True)
    parser.add_argument("--base-url", required=True)
    flags = parser.parse_args(args)
    argv = [sys.executable, "-m", "bothy_sdk.testing.fake_agent"]
    try:
        asyncio.run(run_agents(flags.agents, flags.base_url, argv))
    except AgentError as exc:
        print(f"e2e-agent: FAIL: {exc}", file=sys.stderr, flush=True)
        return 1
    except KeyboardInterrupt:
        return 0
    return 0


if __name__ == "__main__":
    # The container stops the process with SIGTERM; as process 1 it needs a handler.
    import signal

    signal.signal(signal.SIGTERM, lambda *_: os._exit(0))
    sys.exit(main())
