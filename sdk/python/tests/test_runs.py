# SPDX-License-Identifier: Apache-2.0
"""Run patcher and run handle: paths, throttle, coalescing, rebase, steps, follow-ups, finish."""

from __future__ import annotations

import json
import re
from collections.abc import Callable
from pathlib import Path
from typing import Any
from uuid import UUID

import httpx
import pytest
from conftest import REPO

from bothy_sdk import errors
from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import Client
from bothy_sdk.errors import GatewayOpError
from bothy_sdk.generated.models import JsonPatchOperation, JsonPatchOperationOp, Run
from bothy_sdk.runs import PATCH_PATHS, RunHandle, RunPatcher
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing.doubles import FakeClock

RUN_JSON = json.loads(
    (REPO / "sdk/python/src/bothy_sdk/testing/fixtures/run.json").read_text(encoding="utf-8")
)
RUN_ID = UUID(RUN_JSON["id"])
CORR = "0192f4a0-7b1c-7000-8a00-000000000009"


def _run(version: int, **extra: Any) -> dict[str, Any]:
    return {**RUN_JSON, "state": "running", "version": version, **extra}


def _problem(status: int, code: str, **extra: Any) -> dict[str, Any]:
    return {
        "type": "about:blank",
        "title": code,
        "status": status,
        "code": code,
        "correlation_id": CORR,
        **extra,
    }


class Recorder:
    """A MockTransport handler that records requests and answers from a script."""

    def __init__(self, answer: Callable[[httpx.Request, int], httpx.Response]) -> None:
        self.answer = answer
        self.requests: list[tuple[str, str, Any]] = []

    def __call__(self, request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content) if request.content else None
        self.requests.append((request.method, request.url.path, body))
        return self.answer(request, len(self.requests))


def _client(rec: Recorder, clock: FakeClock | None = None, **settings: Any) -> Client:
    return Client(
        "https://example.com/api/v1",
        Credentials(Secret("runs-test-token")),
        transport=httpx.MockTransport(rec),
        clock=clock,
        settings=SdkSettings(**settings),
    )


def _patch_ok(request: httpx.Request, n: int) -> httpx.Response:
    if request.method == "PATCH":
        base = json.loads(request.content)["base_version"]
        return httpx.Response(200, json={"version": base + 1})
    return httpx.Response(200, json=_run(1))


def test_patch_paths_match_spec() -> None:
    text = (REPO / "docs/spec/09-agents.md").read_text(encoding="utf-8")
    section = text[text.index("### 5.3 ") : text.index("### 5.4 ")]
    expected: set[tuple[str, frozenset[str]]] = set()
    for line in section.splitlines():
        if not line.startswith("| `/"):
            continue
        cells = [c.strip() for c in line.strip("|").split("|")]
        ops = frozenset(re.findall(r"`(add|replace|remove)`", cells[2]))
        if not ops:
            continue  # "never": the server owns these paths
        path = cells[0].strip("`")
        if path == "/steps/<id>":
            expected.add((path, frozenset({"add"})))
            for member in re.findall(r"`(status|title|detail|finished_at)`", cells[2]):
                expected.add((f"/steps/<id>/{member}", frozenset({"replace"})))
            continue
        expected.add((path, ops))
    assert {(p.pattern, p.ops) for p in PATCH_PATHS} == expected


@pytest.mark.anyio
async def test_forbidden_path_raises_before_request() -> None:
    rec = Recorder(_patch_ok)
    patcher = RunPatcher(_client(rec), RUN_ID, 1)
    for op, path in (
        ("replace", "/state"),
        ("add", "/model"),
        ("replace", "/steps/x"),
        ("remove", "/cost"),
        ("replace", "/version"),
    ):
        item = JsonPatchOperation(op=JsonPatchOperationOp(op), path=path, value="x")
        with pytest.raises(ValueError, match="not allowed"):
            await patcher.send_ops([item])
    assert rec.requests == []


@pytest.mark.anyio
async def test_throttle_and_coalescing() -> None:
    clock = FakeClock()
    rec = Recorder(_patch_ok)
    patcher = RunPatcher(_client(rec, clock, run_update_min_interval=0.5), RUN_ID, 1, clock=clock)
    patcher.set_status_line("one")
    patcher.set_status_line("two")
    step = patcher.add_step("tool_call", "search")
    patcher.update_step(step, status="completed")
    await clock.advance(0)
    patches = [r for r in rec.requests if r[0] == "PATCH"]
    assert len(patches) == 1
    ops = patches[0][2]["ops"]
    assert {"op": "replace", "path": "/status_line", "value": "two"} in ops
    adds = [o for o in ops if o["path"] == f"/steps/{step}"]
    assert len(adds) == 1
    assert adds[0]["op"] == "add"
    assert adds[0]["value"]["status"] == "completed"
    patcher.set_model("m1")
    await clock.advance(0.2)
    assert len([r for r in rec.requests if r[0] == "PATCH"]) == 1  # still throttled
    await clock.advance(0.31)
    patches = [r for r in rec.requests if r[0] == "PATCH"]
    assert len(patches) == 2
    assert patches[1][2]["base_version"] == 2


@pytest.mark.anyio
async def test_rebase_on_base_version_conflict_rest() -> None:
    def answer(request: httpx.Request, n: int) -> httpx.Response:
        body = json.loads(request.content)
        if body["base_version"] != 7:
            return httpx.Response(409, json=_problem(409, "base_version_conflict", current=_run(7)))
        return httpx.Response(200, json={"version": 8})

    rec = Recorder(answer)
    patcher = RunPatcher(_client(rec), RUN_ID, 3)
    patcher.set_model("model-x")
    await patcher.flush()
    assert [r[2]["base_version"] for r in rec.requests] == [3, 7]
    assert rec.requests[0][2]["ops"] == rec.requests[1][2]["ops"]
    assert patcher.base_version == 8


@pytest.mark.anyio
async def test_rebase_after_gateway_error_loads_run() -> None:
    sent: list[tuple[int, list[JsonPatchOperation]]] = []

    class Gw:
        async def send_run_update(
            self, run_id: UUID, base: int, ops: list[JsonPatchOperation], ref: str | None = None
        ) -> None:
            sent.append((base, ops))

    rec = Recorder(lambda request, n: httpx.Response(200, json=_run(9)))
    patcher = RunPatcher(_client(rec), RUN_ID, 2)
    patcher.use_gateway(Gw())
    patcher.set_model("model-gw")
    await patcher.flush()
    assert patcher.base_version == 3  # assumed: no success reply over the gateway
    await patcher.on_gateway_error(GatewayOpError("run.update", "base_version_conflict"))
    assert [r[0] for r in rec.requests] == ["GET"]
    assert [s[0] for s in sent] == [2, 9]
    assert patcher.base_version == 10


@pytest.mark.anyio
async def test_step_ids_uuid7_without_ordinal() -> None:
    rec = Recorder(_patch_ok)
    patcher = RunPatcher(_client(rec), RUN_ID, 1)
    ids = [patcher.add_step("tool_call", f"step-{i}") for i in range(3)]
    assert all(i.version == 7 for i in ids)
    assert ids == sorted(ids)
    await patcher.flush()
    for op in rec.requests[0][2]["ops"]:
        assert "ordinal" not in op["value"]
        assert op["value"]["id"] == op["path"].rsplit("/", 1)[1]


def test_followup_states_forward_only() -> None:
    patcher = RunPatcher(_client(Recorder(_patch_ok)), RUN_ID, 1)
    m1, m2 = UUID(int=1), UUID(int=2)
    patcher.followup_delivered(m1)
    patcher.followup_absorbed(m1)
    with pytest.raises(ValueError, match="forward"):
        patcher.followup_delivered(m1)
    patcher.followup_absorbed(m2)
    with pytest.raises(ValueError, match="forward"):
        patcher.followup_absorbed(m2)


@pytest.mark.anyio
async def test_finish_flushes_first() -> None:
    def answer(request: httpx.Request, n: int) -> httpx.Response:
        if request.method == "PATCH":
            return httpx.Response(200, json={"version": 2})
        return httpx.Response(200, json=_run(2, state="completed"))

    rec = Recorder(answer)
    handle = RunHandle(_client(rec), RUN_ID, UUID(RUN_JSON["thread_id"]), outbox=None)
    handle.started(Run.from_dict(_run(1)), "jti-1")
    handle.patch.set_status_line("almost done")
    await handle.finish("completed", summary="done")
    assert [(m, p.rsplit("/", 1)[-1]) for m, p, _ in rec.requests] == [
        ("PATCH", str(RUN_ID)),
        ("POST", "finish"),
    ]
    assert rec.requests[1][2]["state"] == "completed"
    await handle.finish("completed")  # once only
    assert len(rec.requests) == 2


@pytest.mark.parametrize(
    "problem",
    [
        _problem(409, "conflict", reason="run_not_active"),
        _problem(403, "invocation_token_invalid", reason="run_inactive"),
    ],
)
@pytest.mark.anyio
async def test_run_not_active_or_inactive_stops_patcher(problem: dict[str, Any]) -> None:
    rec = Recorder(lambda request, n: httpx.Response(problem["status"], json=problem))
    handle = RunHandle(_client(rec), RUN_ID, UUID(RUN_JSON["thread_id"]), outbox=None)
    handle.started(Run.from_dict(_run(1)), "jti-1")
    handle.patch.set_model("m")
    await handle.patch.flush()
    assert handle.patch.ended
    assert handle.cancelled.is_set()
    handle.patch.set_model("again")
    await handle.patch.flush()
    assert len(rec.requests) == 1


@pytest.mark.anyio
async def test_finish_after_run_inactive_not_raised() -> None:
    problem = _problem(403, "invocation_token_invalid", reason="run_inactive")
    rec = Recorder(lambda request, n: httpx.Response(403, json=problem))
    handle = RunHandle(_client(rec), RUN_ID, UUID(RUN_JSON["thread_id"]), outbox=None)
    handle.started(Run.from_dict(_run(1)), "jti-1")
    await handle.finish("completed")
    assert handle.finished


def test_patch_before_start_refused() -> None:
    handle = RunHandle(_client(Recorder(_patch_ok)), RUN_ID, UUID(int=3), outbox=None)
    with pytest.raises(RuntimeError, match="not started"):
        _ = handle.patch
    assert Path(__file__).exists()


@pytest.mark.anyio
async def test_refused_patch_does_not_block_later_updates() -> None:
    def answer(request: httpx.Request, n: int) -> httpx.Response:
        ops = json.loads(request.content)["ops"]
        if any(op["path"] == "/cost" for op in ops):
            problem = _problem(
                400, "validation_failed", errors=[{"pointer": "/ops/0", "code": "invalid"}]
            )
            return httpx.Response(400, json=problem)
        return httpx.Response(
            200, json={"version": json.loads(request.content)["base_version"] + 1}
        )

    rec = Recorder(answer)
    patcher = RunPatcher(_client(rec), RUN_ID, 1)
    patcher.set_cost(5, "usd")
    with pytest.raises(errors.ValidationFailed):
        await patcher.flush()
    patcher.set_status_line("still moving")
    await patcher.flush()
    assert [[op["path"] for op in r[2]["ops"]] for r in rec.requests] == [
        ["/cost"],
        ["/status_line"],
    ]


@pytest.mark.anyio
async def test_transient_patch_failure_keeps_the_changes() -> None:
    def answer(request: httpx.Request, n: int) -> httpx.Response:
        if n == 1:
            return httpx.Response(503, json=_problem(503, "unavailable"))
        return httpx.Response(200, json={"version": 2})

    rec = Recorder(answer)
    patcher = RunPatcher(_client(rec, retry_max_attempts=1), RUN_ID, 1)
    patcher.set_model("m1")
    with pytest.raises(errors.Unavailable):
        await patcher.flush()
    await patcher.flush()
    assert [op["path"] for op in rec.requests[1][2]["ops"]] == ["/model"]


@pytest.mark.anyio
async def test_change_during_a_timed_patch_goes_out_later() -> None:
    clock = FakeClock()
    patcher: RunPatcher | None = None

    def answer(request: httpx.Request, n: int) -> httpx.Response:
        if n == 1:
            assert patcher is not None
            patcher.set_status_line("late")  # arrives while the first patch is on its way
        return _patch_ok(request, n)

    rec = Recorder(answer)
    patcher = RunPatcher(_client(rec, clock, run_update_min_interval=0.5), RUN_ID, 1, clock=clock)
    patcher.set_status_line("first")
    await clock.advance(0)
    assert len(rec.requests) == 1
    await clock.advance(0.6)
    assert len(rec.requests) == 2
    assert {"op": "replace", "path": "/status_line", "value": "late"} in rec.requests[1][2]["ops"]
