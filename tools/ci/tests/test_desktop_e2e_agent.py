# SPDX-License-Identifier: Apache-2.0
"""The agents of the desktop E2E test (brief M4-34 Design note 1): tools/ci/desktop_e2e_agent.py.

The roles run against a scripted control call (the replies of the Fake-Agent), so the order of
the ops, their arguments and the failure on an unexpected status are checked without the SDK;
the process plumbing runs against a small Python stand-in on stdin and stdout.
"""

from __future__ import annotations

import asyncio
import json
import re
import sys
import textwrap
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import desktop_e2e_agent as agent
import pytest

Reply = dict[str, Any]


class Script:
    """A control call that answers each op from a queue and records the requests."""

    def __init__(self, answers: dict[str, list[Reply]]) -> None:
        self.answers = {op: list(replies) for op, replies in answers.items()}
        self.calls: list[tuple[str, dict[str, Any]]] = []

    async def __call__(self, op: str, args: dict[str, Any] | None) -> Reply:
        self.calls.append((op, args or {}))
        queue = self.answers.get(op)
        if not queue:
            return {"ok": True, "result": {"status": 200}}
        return queue.pop(0) if len(queue) > 1 else queue[0]

    def ops(self) -> list[str]:
        return [op for op, _ in self.calls]


def ok(**result: Any) -> Reply:
    return {"ok": True, "result": result}


async def no_sleep(_: float) -> None:
    return None


INVOCATION = ok(event="agent.invocation", verdict="valid", run_id="run-1", thread_id="thread-1")


def test_format_now_and_uuid7() -> None:
    moment = datetime(2030, 1, 2, 3, 4, 5, 678_901, tzinfo=UTC)
    assert agent.format_now(moment) == "2030-01-02T03:04:05.678Z"
    first, second = agent.uuid7(), agent.uuid7()
    assert first != second
    assert re.fullmatch(
        r"[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}", first
    )


def test_connect_sends_the_credentials_and_steer() -> None:
    script = Script({"connect": [ok(status=200)]})
    entry = {
        "handle": "support-agent",
        "agent_id": "a-1",
        "token": "t",
        "dpop_private_jwk": {"d": "x"},
    }
    asyncio.run(
        agent.connect(script, base_url="https://localhost:1/api/v1", instance_id="i-1", agent=entry)
    )
    assert script.ops() == ["hello", "connect", "heartbeat"]
    hello = script.calls[0][1]
    assert hello["credentials"] == {"token": "t", "dpop_private_jwk": {"d": "x"}}
    assert hello["transport"] == "rest"
    assert hello["instance_id"] == "i-1"
    assert script.calls[2][1] == {"capabilities": {"followups": "steer"}}


def test_connect_fails_on_a_refused_hello() -> None:
    script = Script({"hello": [{"ok": False, "error": {"kind": "usage", "detail": "bad"}}]})
    with pytest.raises(agent.AgentError, match="hello: usage bad"):
        asyncio.run(
            agent.connect(
                script,
                base_url="u",
                instance_id="i",
                agent={"agent_id": "a", "token": "t", "dpop_private_jwk": {}},
            )
        )


def test_receive_polls_until_an_event() -> None:
    slept: list[float] = []

    async def sleep(seconds: float) -> None:
        slept.append(seconds)

    script = Script({"receive": [ok(none=True), ok(none=True), INVOCATION]})
    got = asyncio.run(agent.receive(script, sleep, {"type": "agent.invocation"}, "invocation"))
    assert got["run_id"] == "run-1"
    assert slept == [agent.POLL_INTERVAL_S, agent.POLL_INTERVAL_S]


def test_receive_fails_on_a_refused_poll() -> None:
    script = Script({"receive": [ok(status=401, code="unauthenticated")]})
    with pytest.raises(agent.AgentError, match="status 401 unauthenticated"):
        asyncio.run(agent.receive(script, no_sleep, {}, "invocation"))


def test_support_run_plays_the_whole_run() -> None:
    script = Script(
        {
            "receive": [
                INVOCATION,
                ok(event="agent.followup", message_id="m-2"),
                ok(event="interaction.created", interaction_id="ix-1"),
            ],
            "post": [ok(status=201)],
        }
    )
    assert asyncio.run(agent.support_run(script, no_sleep)) == "run-1"
    assert script.ops() == [
        "receive", "start", "patch", "patch", "patch", "receive", "patch", "patch",
        "post", "receive", "respond", "post", "finish",
    ]  # fmt: skip
    patches = [args["ops"][0] for op, args in script.calls if op == "patch"]
    assert patches[0] == {"op": "add", "path": "/status_line", "value": agent.STATUS_LINE}
    assert patches[1]["path"].startswith("/steps/")
    assert patches[1]["value"] == {
        "kind": "tool_call",
        "name": "search",
        "title": "Search the notes",
    }
    assert [p["value"] for p in patches[3:]] == ["delivered", "absorbed"]
    assert patches[3]["path"] == "/followups/m-2/state"
    card = next(args for op, args in script.calls if op == "post")
    assert card == {
        "thread": "thread-1",
        "run": "run-1",
        "content": agent.APPROVAL_TEXT,
        "card": "approval",
    }
    respond = next(args for op, args in script.calls if op == "respond")
    assert respond == {"interaction": "ix-1", "run": "run-1", "text": agent.APPROVED_TEXT}
    assert script.calls[-1] == ("finish", {"run": "run-1", "state": "completed"})


def test_a_refused_invocation_or_patch_ends_the_run() -> None:
    refused = Script({"receive": [ok(event="agent.invocation", verdict="invalid")]})
    with pytest.raises(agent.AgentError, match="verdict invalid"):
        asyncio.run(agent.support_run(refused, no_sleep))
    patch = Script({"receive": [INVOCATION], "patch": [ok(status=409, code="conflict")]})
    with pytest.raises(agent.AgentError, match="patch: status 409 conflict"):
        asyncio.run(agent.stream_run(patch, no_sleep))


def test_stream_run_streams_the_status_line() -> None:
    slept: list[float] = []

    async def sleep(seconds: float) -> None:
        slept.append(seconds)

    script = Script({"receive": [INVOCATION]})
    assert asyncio.run(agent.stream_run(script, sleep, patches=3)) == "run-1"
    values = [args["ops"][0]["value"] for op, args in script.calls if op == "patch"]
    assert values == ["Streaming 1 of 3", "Streaming 2 of 3", "Streaming 3 of 3"]
    assert slept == [agent.PATCH_INTERVAL_S] * 3
    assert script.ops()[-1] == "finish"


def test_serve_plays_the_role_again_and_again() -> None:
    script = Script({"receive": [INVOCATION]})
    assert asyncio.run(agent.serve(script, "stream-agent", no_sleep, runs=2)) == 2
    assert script.ops().count("finish") == 2


# ---- the process plumbing with a stand-in --------------------------------------------------

# Answers hello, connect and heartbeat, then ends: the role's first receive finds no answer.
STAND_IN = textwrap.dedent(
    """
    import json, sys
    for line in sys.stdin:
        req = json.loads(line)
        if req["op"] == "receive":
            break
        result = {"status": 200} if req["op"] in ("connect", "heartbeat") else {}
        print(json.dumps({"id": req["id"], "ok": True, "result": result}), flush=True)
    """
)


def test_main_reports_an_ended_fake_agent(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    stand_in = tmp_path / "stand_in.py"
    stand_in.write_text(STAND_IN, encoding="utf-8")
    agents = tmp_path / "e2e-agent.json"
    agents.write_text(
        json.dumps(
            {
                "instance_id": "i-1",
                "agents": [
                    {
                        "handle": "stream-agent",
                        "agent_id": "a",
                        "token": "t",
                        "dpop_private_jwk": {},
                    }
                ],
            }
        ),
        encoding="utf-8",
    )
    code = agent.main(
        ["--agents", str(agents), "--base-url", "https://localhost:1/api/v1"],
        agent_argv=[sys.executable, str(stand_in)],
    )
    assert code == 1
    err = capsys.readouterr().err
    assert "e2e-agent: ready stream-agent" in err
    assert "e2e-agent: FAIL: receive: the Fake-Agent ended" in err
