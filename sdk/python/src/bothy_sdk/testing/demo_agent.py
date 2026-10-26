# SPDX-License-Identifier: Apache-2.0
"""The demo agent: the Fake-Agent playing the agents of the demo instance (16 16.4, M7-27).

``bothy setup demo --exchange DIR`` writes ``agents.json`` into the exchange directory (the
instance, every agent with its role, API token and private DPoP key, and the named values of the
data set). This module starts one `FakeAgent` per agent and plays its role through the public
API, without a model call:

* ``worker`` starts each run, patches its live box (status line and tool steps), posts a reply
  and finishes the run with the costs of its made-up model;
* ``reviewer`` starts the run, posts an approval card and waits for the click; after the click it
  answers the interaction and finishes the run;
* ``watcher`` keeps its run ``running`` and patches the live box until the process stops.

Credentials come only from the exchange file and live only in memory; logs go to stderr at level
``WARNING`` and never carry a token or a key (D-M2-31-5).

Run it with ``python -m bothy_sdk.testing.demo_agent --exchange DIR --base-url URL``.

Examples:
    >>> values = DemoValues.from_mapping({"worker_steps": 2, "step_interval_ms": 10,
    ...     "watcher_interval_ms": 10, "heartbeat_interval_ms": 10, "tokens_in": 5,
    ...     "tokens_out": 1})
    >>> values.worker_steps
    2
"""

from __future__ import annotations

import argparse
import asyncio
import contextlib
import json
import logging
import signal
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from bothy_sdk.ids import uuid7
from bothy_sdk.testing.control import Request
from bothy_sdk.testing.fake_agent import FakeAgent

__all__ = ["DemoAgent", "DemoEntry", "DemoError", "DemoValues", "load_exchange", "main", "serve"]

_LOG = logging.getLogger("bothy_sdk.demo_agent")

#: The file of the exchange directory the command writes (16 16.5).
AGENTS_FILE = "agents.json"
#: The roles of 16 16.4.
ROLES = frozenset({"worker", "reviewer", "watcher"})

Sleep = Callable[[float], Awaitable[None]]


class DemoError(RuntimeError):
    """An op of the Fake-Agent failed; the message names the op and the code, never a token."""


@dataclass(frozen=True)
class DemoValues:
    """The named values of the data set the demo agent uses (16 16.3).

    Attributes:
        worker_steps: Tool steps of a worker run.
        step_interval: Seconds between two patches of a worker run.
        watcher_interval: Seconds between two live box updates of a watcher run.
        heartbeat_interval: Seconds between two heartbeats of an agent.
        tokens_in: Input tokens a worker run reports with its costs.
        tokens_out: Output tokens a worker run reports with its costs.
    """

    worker_steps: int
    step_interval: float
    watcher_interval: float
    heartbeat_interval: float
    tokens_in: int
    tokens_out: int

    @classmethod
    def from_mapping(cls, raw: Mapping[str, Any]) -> DemoValues:
        """Read the ``values`` object of ``agents.json``.

        Args:
            raw: The object with the members of `AgentValues` of the command.

        Returns:
            The values; intervals in seconds.
        """
        return cls(
            worker_steps=int(raw["worker_steps"]),
            step_interval=int(raw["step_interval_ms"]) / 1000,
            watcher_interval=int(raw["watcher_interval_ms"]) / 1000,
            heartbeat_interval=int(raw["heartbeat_interval_ms"]) / 1000,
            tokens_in=int(raw["tokens_in"]),
            tokens_out=int(raw["tokens_out"]),
        )


@dataclass(frozen=True)
class DemoEntry:
    """One agent of ``agents.json``.

    Attributes:
        handle: The handle of the agent.
        role: ``worker``, ``reviewer`` or ``watcher``.
        provider: The provider of its made-up model.
        model: The made-up model.
        currency: The currency of the price of the model.
        agent_id: The principal ID.
        token: The API token (in memory only).
        dpop_private_jwk: The private DPoP key the token is bound to (in memory only).
    """

    handle: str
    role: str
    provider: str
    model: str
    currency: str
    agent_id: str
    token: str
    dpop_private_jwk: Mapping[str, Any]

    def __repr__(self) -> str:
        """Name the agent without its credentials."""
        return f"DemoEntry(handle={self.handle!r}, role={self.role!r})"


def load_exchange(directory: Path) -> tuple[str, DemoValues, list[DemoEntry]]:
    """Read ``agents.json`` of the exchange directory.

    Args:
        directory: The exchange directory of ``bothy setup demo``.

    Returns:
        The instance ID, the values and the agents.

    Raises:
        DemoError: The file names an unknown role.
        OSError: The file cannot be read.
    """
    doc = json.loads((directory / AGENTS_FILE).read_text(encoding="utf-8"))
    entries = []
    for a in doc["agents"]:
        if a["role"] not in ROLES:
            raise DemoError(f"unknown role of {a['handle']}")
        entries.append(
            DemoEntry(
                handle=str(a["handle"]),
                role=str(a["role"]),
                provider=str(a["provider"]),
                model=str(a["model"]),
                currency=str(a["currency"]),
                agent_id=str(a["agent_id"]),
                token=str(a["token"]),
                dpop_private_jwk=dict(a["dpop_private_jwk"]),
            )
        )
    return str(doc["instance_id"]), DemoValues.from_mapping(doc["values"]), entries


def _now() -> datetime:
    return datetime.now(UTC)


class DemoAgent:
    """One demo agent: a `FakeAgent` and the script of its role."""

    def __init__(  # noqa: PLR0913 - every part of the connection is explicit
        self,
        entry: DemoEntry,
        values: DemoValues,
        *,
        instance_id: str,
        base_url: str,
        settings: Mapping[str, str] | None = None,
        sleep: Sleep = asyncio.sleep,
        clock: Callable[[], datetime] = _now,
    ) -> None:
        """Create the agent; `connect` signs it in.

        Args:
            entry: The agent of ``agents.json``.
            values: The named values.
            instance_id: The ID of the instance (the audience of invocation tokens).
            base_url: The REST base, for example ``https://chat.example.com/api/v1``.
            settings: SDK settings (``sdk.*``) for the hand layer.
            sleep: Waits the given seconds (tests pass a fast one).
            clock: The wall time the requests carry.
        """
        self.entry = entry
        self.values = values
        self._instance_id = instance_id
        self._base_url = base_url
        self._settings = dict(settings or {})
        self._sleep = sleep
        self._clock = clock
        self._fake = FakeAgent()
        self._n = 0
        self._tasks: set[asyncio.Task[None]] = set()
        # The interaction of the waiting approval card per run of a reviewer.
        self._waiting: dict[str, str] = {}

    async def op(self, op: str, **args: Any) -> dict[str, Any]:
        """Send one op to the Fake-Agent.

        Args:
            op: The op of the control protocol.
            **args: Its arguments.

        Returns:
            The result members.

        Raises:
            DemoError: The op failed or the server answered with a problem.
        """
        self._n += 1
        reply = await self._fake.handle(Request(self._n, op, args, self._clock()))
        if not reply.ok:
            kind = reply.error_kind.value if reply.error_kind else "error"
            raise DemoError(f"{self.entry.handle}: {op} failed: {kind} {reply.error_detail}")
        result = reply.result
        status = result.get("status")
        if isinstance(status, int) and status >= 400:
            raise DemoError(f"{self.entry.handle}: {op} answered {status} {result.get('code')}")
        return result

    async def connect(self) -> None:
        """Sign in with the API token over REST and send the first heartbeat."""
        await self.op(
            "hello",
            base_url=self._base_url,
            gateway_url="/api/v1/gateway",
            instance_id=self._instance_id,
            agent_id=self.entry.agent_id,
            consumer="default",
            transport="rest",
            credentials={
                "token": self.entry.token,
                "dpop_private_jwk": dict(self.entry.dpop_private_jwk),
            },
            runtime={
                "billing_mode": "organization",
                "tool_class": "chat",
                "model": self.entry.model,
            },
            settings=self._settings,
        )
        await self.op("connect")
        await self.op("heartbeat")

    async def serve(self, stop: asyncio.Event) -> None:
        """Answer every event until ``stop`` is set, with a heartbeat every interval.

        Args:
            stop: Ends the loop; open watcher runs stop patching.
        """
        last_beat = asyncio.get_running_loop().time()
        while not stop.is_set():
            got = await self.op("receive")
            if not got.get("none"):
                self._dispatch(got, stop)
            now = asyncio.get_running_loop().time()
            if now - last_beat >= self.values.heartbeat_interval:
                await self.op("heartbeat")
                last_beat = now
            if got.get("none"):
                await self._sleep(min(self.values.step_interval, self.values.heartbeat_interval))
        for task in list(self._tasks):
            task.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await task

    def _dispatch(self, got: Mapping[str, Any], stop: asyncio.Event) -> None:
        """Start the script of one delivered event in its own task."""
        event, run = got.get("event"), got.get("run_id")
        if got.get("verdict") not in (None, "valid") or run is None:
            return
        script: Awaitable[None] | None = None
        if event == "agent.invocation":
            thread = str(got.get("thread_id"))
            roles: dict[str, Callable[[], Awaitable[None]]] = {
                "worker": lambda: self.worker(str(run), thread),
                "reviewer": lambda: self.reviewer(str(run), thread),
                "watcher": lambda: self.watcher(str(run), stop),
            }
            script = roles[self.entry.role]()
        elif event == "interaction.created" and str(run) in self._waiting:
            script = self.approved(str(run), str(got.get("interaction_id")))
        if script is None:
            return
        task = asyncio.ensure_future(self._guard(script))
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)

    async def _guard(self, script: Awaitable[None]) -> None:
        """Run a script; a failure is logged without content and ends only this run's script."""
        try:
            await script
        except DemoError as exc:
            _LOG.warning("%s", exc)

    def _costs(self) -> list[dict[str, Any]]:
        e, v = self.entry, self.values
        return [
            {
                "provider": e.provider,
                "model": e.model,
                "tokens_in": v.tokens_in,
                "tokens_out": v.tokens_out,
                "currency": e.currency,
            }
        ]

    def _step(self, name: str, title: str) -> tuple[str, dict[str, Any]]:
        step = str(uuid7())
        value = {
            "kind": "tool_call",
            "name": name,
            "title": title,
            "status": "running",
            "started_at": self._clock().strftime("%Y-%m-%dT%H:%M:%S.000Z"),
        }
        return step, {"op": "add", "path": f"/steps/{step}", "value": value}

    async def worker(self, run: str, thread: str) -> None:
        """Play a worker run: start, steps, reply, finish with costs.

        Args:
            run: The run ID.
            thread: The thread of the run.
        """
        await self.op("start", run=run, model=self.entry.model)
        for i in range(1, self.values.worker_steps + 1):
            step, add = self._step("search", f"Looking up source {i}")
            line = {"op": "replace", "path": "/status_line", "value": f"Checking source {i}"}
            await self.op("patch", run=run, ops=[line, add])
            await self._sleep(self.values.step_interval)
            done = {"op": "replace", "path": f"/steps/{step}/status", "value": "completed"}
            await self.op("patch", run=run, ops=[done])
        text = f"Done. I checked {self.values.worker_steps} sources; the details are above."
        await self.op("post", thread=thread, run=run, content=text)
        await self.op("finish", run=run, summary="Answered the request", costs=self._costs())

    async def reviewer(self, run: str, thread: str) -> None:
        """Play a reviewer run: start, an approval card, wait for the click.

        Args:
            run: The run ID.
            thread: The thread of the run.
        """
        await self.op("start", run=run, model=self.entry.model)
        line = {"op": "replace", "path": "/status_line", "value": "Release notes are ready"}
        await self.op("patch", run=run, ops=[line])
        card = await self.op(
            "post", thread=thread, run=run, content="Publish the release notes?", card="approval"
        )
        self._waiting[run] = str(card.get("message_id"))
        await self.op("await_input", run=run, status_line="Waiting for approval")

    async def approved(self, run: str, interaction: str) -> None:
        """Resume the run with the token of the click, answer it and finish with costs.

        Args:
            run: The run ID.
            interaction: The interaction of the click.
        """
        self._waiting.pop(run, None)
        # The click carries a new invocation token; the run leaves waiting_input with it (09 4.7).
        await self.op("resume", run=run, model=self.entry.model)
        await self.op("respond", interaction=interaction, run=run, text="Published.")
        await self.op("finish", run=run, summary="Published the release notes", costs=self._costs())

    async def watcher(self, run: str, stop: asyncio.Event) -> None:
        """Keep a run running and patch its live box until ``stop`` is set.

        Args:
            run: The run ID.
            stop: Ends the run's updates; the run stays open.
        """
        await self.op("start", run=run, model=self.entry.model)
        n = 0
        while not stop.is_set():
            n += 1
            step, add = self._step("build", f"Build stage {n}")
            line = {"op": "replace", "path": "/status_line", "value": f"Watching build stage {n}"}
            await self.op("patch", run=run, ops=[line, add])
            await self._sleep(self.values.watcher_interval)
            done = {"op": "replace", "path": f"/steps/{step}/status", "value": "completed"}
            await self.op("patch", run=run, ops=[done])

    async def close(self) -> None:
        """Close the Fake-Agent."""
        await self._fake.close()


async def serve(
    directory: Path,
    base_url: str,
    stop: asyncio.Event,
    *,
    sleep: Sleep = asyncio.sleep,
    clock: Callable[[], datetime] = _now,
) -> None:
    """Run every agent of the exchange directory until ``stop`` is set.

    Args:
        directory: The exchange directory.
        base_url: The REST base of the instance.
        stop: Ends every agent.
        sleep: Waits the given seconds.
        clock: The wall time the requests carry.
    """
    instance_id, values, entries = load_exchange(directory)
    agents = [
        DemoAgent(e, values, instance_id=instance_id, base_url=base_url, sleep=sleep, clock=clock)
        for e in entries
    ]
    try:
        for a in agents:
            await a.connect()
        await asyncio.gather(*(a.serve(stop) for a in agents))
    finally:
        for a in agents:
            await a.close()


async def _wait_for_exchange(directory: Path, stop: asyncio.Event) -> None:
    """Wait until ``agents.json`` exists: the command writes it after it created the agents."""
    while not (directory / AGENTS_FILE).exists() and not stop.is_set():
        await asyncio.sleep(1)


def main(argv: list[str] | None = None) -> None:
    """Run the demo agents until SIGINT or SIGTERM.

    Args:
        argv: The command line without the program name.
    """
    parser = argparse.ArgumentParser(description="The demo agents of bothy setup demo.")
    parser.add_argument("--exchange", required=True, type=Path, help="exchange directory")
    parser.add_argument("--base-url", required=True, help="REST base of the instance")
    args = parser.parse_args(argv)
    logging.basicConfig(level=logging.WARNING)

    async def run() -> None:
        stop = asyncio.Event()
        loop = asyncio.get_running_loop()
        for sig in (signal.SIGINT, signal.SIGTERM):
            loop.add_signal_handler(sig, stop.set)
        await _wait_for_exchange(args.exchange, stop)
        if not stop.is_set():
            await serve(args.exchange, args.base_url, stop)

    asyncio.run(run())


if __name__ == "__main__":
    main()
