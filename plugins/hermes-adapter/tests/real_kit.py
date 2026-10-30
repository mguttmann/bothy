# SPDX-License-Identifier: Apache-2.0
"""Helpers of the end-to-end tests against the real server (``tests/realserver``).

The person side goes through the program's ``call`` op (Alex or the owner over the real HTTP
chain); the adapter side is the adapter's own code from ``adapter_kit.rig`` with the fake Hermes
runtime, pointed at the real server instead of the FakeServer.
"""

from __future__ import annotations

import asyncio
import itertools
import time
from datetime import UTC, datetime
from typing import Any
from uuid import UUID

from bothy_sdk.ids import uuid7
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.sdkserver import ApiAnswer, RealServer

_CONSUMERS = itertools.count(1)
ENDED = ("completed", "failed", "cancelled")


def consumer() -> str:
    """A consumer name of its own for each adapter of a test."""
    return f"e2e{next(_CONSUMERS)}"


def ok(answer: ApiAnswer, *statuses: int) -> Any:
    """The body of a successful call; a failure names status and body."""
    expected = statuses or (200, 201)
    assert answer.status in expected, f"status {answer.status}: {answer.body}"
    return answer.body


async def post(real: RealServer, thread_id: UUID, text: str, **extra: Any) -> UUID:
    """Alex posts ``text`` into the thread; the message ID."""
    body = {"client_msg_id": str(uuid7()), "content_md": text, **extra}
    created = ok(await real.call("createMessage", path={"thread_id": thread_id}, body=body), 201)
    return UUID(created["id"])


async def messages(real: RealServer, thread_id: UUID) -> list[dict[str, Any]]:
    """The messages of the thread, oldest first, as Alex reads them."""
    page = ok(await real.call("listThreadMessages", path={"thread_id": thread_id}, query={}))
    items: list[dict[str, Any]] = list(page["items"])
    return sorted(items, key=lambda m: int(m.get("thread_seq") or 0))


def agent_texts(items: list[dict[str, Any]], agent_id: UUID) -> list[dict[str, Any]]:
    """The text and card messages of the agent (no live box anchor)."""
    return [
        m
        for m in items
        if str((m.get("author") or {}).get("id", m.get("author_id"))) == str(agent_id)
        and m.get("kind") != "run"
    ]


async def until(what: str, check: Any) -> Any:
    """Wait (real time) until the async ``check`` returns a truthy value; that value."""
    deadline = time.monotonic() + PATIENCE
    while True:
        value = await check()
        if value:
            return value
        if time.monotonic() > deadline:
            raise AssertionError(f"not within {PATIENCE} s: {what}")
        await asyncio.sleep(0.1)


async def ended(real: RealServer, run_id: UUID) -> Any:
    """Wait for the end of the run; its view."""

    async def check() -> Any:
        view = await real.run_state(run_id)
        return view if view.state in ENDED else None

    return await until(f"the end of run {run_id}", check)


async def heartbeat(real: RealServer) -> None:
    """Wait for a heartbeat of the agent after now (its capabilities reached the server)."""
    since = datetime.now(UTC)

    async def check() -> bool:
        agent = ok(await real.call("getAgent", as_="owner", path={"agent_id": real.agent_id}))
        at = (agent.get("presence") or {}).get("last_heartbeat_at")
        return at is not None and datetime.fromisoformat(at.replace("Z", "+00:00")) > since

    await until("a heartbeat of the agent", check)
