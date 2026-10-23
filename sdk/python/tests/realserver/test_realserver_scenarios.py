# SPDX-License-Identifier: Apache-2.0
"""The scenario functions against the real server of this tree (ADR 0026 "Confirmation").

`test_record_fixtures` writes the fixtures of ``bothy_sdk.testing`` only when
``BOTHY_SDK_RECORD_FIXTURES=1`` is set; otherwise it checks that every member of the live
``agent.invocation`` envelope and payload (except the delivery fields and ``trace_context``) also
occurs in the recorded fixture.
"""

from __future__ import annotations

import json
import os
import re
from pathlib import Path
from typing import Any

import pytest

from bothy_sdk.adapter import Invocation, StartOptions
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.discovery import get_discovery
from bothy_sdk.generated.api.runs import get_run, get_run_context
from bothy_sdk.runs import RunHandle
from bothy_sdk.testing import RecordingHandler, scenarios
from bothy_sdk.testing.scenarios import (
    ScenarioServer,
    ag08_invalid_tokens,
    ag09_double_delivery,
    ag12_patch_rules,
    ag28_slow_start_and_restart,
    azp6_adapter_never_starts,
)

FIXTURES = Path(__file__).resolve().parents[2] / "src" / "bothy_sdk" / "testing" / "fixtures"
_DELIVERY = {"invocation_token", "redelivered", "trace_context"}
_UUID = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
_TIME = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z$")


@pytest.mark.anyio
async def test_ag08_real(real: ScenarioServer) -> None:
    await ag08_invalid_tokens(real)


@pytest.mark.anyio
async def test_azp6_real(real: ScenarioServer) -> None:
    await azp6_adapter_never_starts(real)


@pytest.mark.anyio
async def test_ag09_real(real: ScenarioServer) -> None:
    await ag09_double_delivery(real)


@pytest.mark.anyio
async def test_ag12_real(real: ScenarioServer) -> None:
    await ag12_patch_rules(real)


@pytest.mark.anyio
async def test_ag28_real(real: ScenarioServer) -> None:
    await ag28_slow_start_and_restart(real)


def _normalize(value: Any, ids: dict[str, str]) -> Any:
    """Fixed IDs in order of appearance, fixed times, ``seq`` 1, no delivery fields."""
    if isinstance(value, dict):
        out = {}
        for key in sorted(value):
            if key in _DELIVERY:
                continue
            out[key] = 1 if key == "seq" else _normalize(value[key], ids)
        return out
    if isinstance(value, list):
        return [_normalize(v, ids) for v in value]
    if isinstance(value, str) and _UUID.match(value):
        return ids.setdefault(value, f"01900000-0000-7000-8000-{len(ids) + 1:012d}")
    if isinstance(value, str) and _TIME.match(value):
        return "2026-01-01T00:00:00.000Z"
    return value


def _members(value: Any, prefix: str = "") -> set[str]:
    out: set[str] = set()
    if isinstance(value, dict):
        for key, item in value.items():
            if key in _DELIVERY:
                continue
            out.add(prefix + key)
            if key == "payload":
                out |= _members(item, prefix + key + ".")
    return out


class _Capturing(RecordingHandler):
    """Records the raw run context in prepare and the raw run in execute."""

    def __init__(self) -> None:
        super().__init__()
        self.raw: dict[str, Any] = {}

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        try:
            response = await run.client.call_detailed(
                get_run_context.asyncio_detailed, run_id=run.run_id
            )
            self.raw["run_context"] = json.loads(response.content)
        except ProblemError:
            pass  # getRunContext is the server side of M2-13; until then nothing to record
        return await super().prepare(inv, run)

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        response = await run.client.call_detailed(get_run.asyncio_detailed, run_id=run.run_id)
        self.raw["run"] = json.loads(response.content)
        await super().execute(inv, run)


@pytest.mark.anyio
async def test_record_fixtures(real: ScenarioServer) -> None:
    handler = _Capturing()
    adapter = scenarios._adapter(real, handler, "fixtures-adapter")
    async with scenarios._running(adapter) as (rec, _):
        await scenarios._until(real, scenarios._ready(rec), "the adapter to connect")
        triggered = await real.trigger("Fixtures @Lotse")
        await scenarios._until(real, lambda: handler.count("execute") == 1, "execute")
        envelope = scenarios._envelope_for(rec, triggered.run_id, "agent.invocation")[0]
        answer = await adapter.client.call_detailed(get_discovery.asyncio_detailed)
        discovery = json.loads(answer.content)
    live = dict(envelope.raw)
    recorded = {"agent.invocation": live, "run": handler.raw["run"], "discovery": discovery}
    if "run_context" in handler.raw:
        recorded["run_context"] = handler.raw["run_context"]
    if os.environ.get("BOTHY_SDK_RECORD_FIXTURES") == "1":
        for name, body in recorded.items():
            ids: dict[str, str] = {}
            text = json.dumps(_normalize(body, ids), indent=2, sort_keys=True) + "\n"
            (FIXTURES / f"{name}.json").write_text(text, encoding="utf-8")
        return
    stored = json.loads((FIXTURES / "agent.invocation.json").read_text(encoding="utf-8"))
    missing = _members(live) - _members(stored)
    assert not missing, f"members missing in the recorded fixture: {sorted(missing)}"
