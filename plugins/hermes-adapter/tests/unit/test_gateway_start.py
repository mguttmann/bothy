# SPDX-License-Identifier: Apache-2.0
"""A Fake-Hermes gateway that reported a failed tool import at its start is started again.

The pinned Hermes warms its turn machinery on an executor thread while its main thread still
builds the platform adapters (``_start_startup_warmup``: "to overlap the connects"). The thread
imports every tool module, the main thread imports the adapter and what it needs of Hermes, and
on a loaded machine one of the overlapping first imports ends in a ``KeyError`` of the import
system. Hermes logs the tool module it could not import and goes on. The Fake-Hermes tests
that failed on main after such a start all had that line in their gateway log; how the failed
import leads to the failed test is not shown (docs/decisions.d/FIX-MAIN-RED-1705.md), so the
start loop of the kit goes by the line. Stand-ins for the gateway process and the FakeServer
show what the loop does with such a start.
"""

from __future__ import annotations

from collections.abc import Callable, Mapping, Sequence
from pathlib import Path
from types import SimpleNamespace
from typing import Any

import fake_hermes_kit
import pytest
from fake_hermes_kit import HermesRig

pytestmark = pytest.mark.anyio

# The line of the gateway log in the main run of 2026-11-11 whose test
# `test_failed_result_turn_ends_the_run_failed` waited in vain for the first answer.
LOST = (
    "2026-11-11 19:48:36,384 WARNING tools.registry: Could not import tool module "
    "tools.delegate_tool: 'tools.terminal_tool_lifecycle'\n"
)


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


class _Gateway:
    """Stand-in for one gateway process: its start plays one scripted start."""

    def __init__(self, play: Callable[[_Gateway], None], log: Path) -> None:
        self.play = play
        self.log = log
        self.alive = True
        self.stopped = False

    def start(self) -> None:
        self.play(self)

    def write(self, text: str) -> None:
        with self.log.open("a", encoding="utf-8") as handle:
            handle.write(text)

    def running(self) -> bool:
        return self.alive

    def stop(self) -> None:
        self.stopped = True


class _Starts:
    """The gateways the start loop made, each with the next scripted start."""

    def __init__(self, plays: Sequence[Callable[[_Gateway], None]]) -> None:
        self.plays = list(plays)
        self.made: list[_Gateway] = []

    def __call__(
        self, command: Sequence[str], *, env: Mapping[str, str], cwd: Path, log: Path
    ) -> _Gateway:
        gateway = _Gateway(self.plays[len(self.made)], log)
        self.made.append(gateway)
        return gateway


def _rig(tmp_path: Path) -> HermesRig:
    install = SimpleNamespace(venv=tmp_path / "venv", root=tmp_path / "i")
    world: Any = SimpleNamespace(install=install, python=tmp_path / "venv" / "bin" / "python")
    fake: Any = SimpleNamespace(frames=[], heartbeats=[], origin="http://127.0.0.1:4711")
    homes: Any = SimpleNamespace(root=tmp_path / "01", work=tmp_path / "01" / "work")
    homes.work.mkdir(parents=True)
    model: Any = SimpleNamespace(port=4712)
    egress: Any = SimpleNamespace(port=4713)
    creds: Any = None
    return HermesRig(world, fake, model, creds, homes, homes.root / "hermes", egress)


def _plays(rig: HermesRig) -> SimpleNamespace:
    def identify(_gateway: _Gateway) -> None:
        rig.fake.frames.extend([{"op": "identify"}, {"op": "heartbeat"}])

    def failed_import(gateway: _Gateway) -> None:
        gateway.write(LOST)

    def failed_import_seen_after_the_identify(gateway: _Gateway) -> None:
        gateway.write(LOST)
        identify(gateway)

    def exits(gateway: _Gateway) -> None:
        gateway.alive = False

    return SimpleNamespace(
        identify=identify,
        failed_import=failed_import,
        failed_import_seen_after_the_identify=failed_import_seen_after_the_identify,
        exits=exits,
    )


async def test_a_start_that_reported_a_failed_tool_import_is_started_again(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    rig = _rig(tmp_path)
    plays = _plays(rig)
    starts = _Starts([plays.failed_import, plays.identify])
    monkeypatch.setattr(fake_hermes_kit, "HermesGateway", starts)
    await fake_hermes_kit._start_gateway(rig)
    first, second = starts.made
    # The discarded start ended before the adapter connected: the server never saw it.
    assert first.stopped
    assert not second.stopped
    assert rig.gateway is second
    assert LOST not in fake_hermes_kit.gateway_log(rig)


async def test_only_the_identify_of_the_new_start_counts_after_a_discarded_start(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    rig = _rig(tmp_path)
    plays = _plays(rig)
    starts = _Starts([plays.failed_import_seen_after_the_identify, plays.exits, plays.identify])
    monkeypatch.setattr(fake_hermes_kit, "HermesGateway", starts)
    await fake_hermes_kit._start_gateway(rig)
    # The identify of the discarded first start must not pass for the second, which exited.
    assert [gateway.stopped for gateway in starts.made] == [True, True, False]
    assert rig.gateway is starts.made[2]


async def test_a_gateway_that_fails_a_tool_import_at_every_start_fails_with_its_log(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    rig = _rig(tmp_path)
    plays = _plays(rig)
    starts = _Starts([plays.failed_import] * fake_hermes_kit._START_ATTEMPTS)
    monkeypatch.setattr(fake_hermes_kit, "HermesGateway", starts)
    with pytest.raises(AssertionError) as failure:
        await fake_hermes_kit._start_gateway(rig)
    message = str(failure.value)
    assert "never started with the adapter's platform and every tool module imported" in message
    assert LOST.strip() in message
    assert len(starts.made) == fake_hermes_kit._START_ATTEMPTS
    assert all(gateway.stopped for gateway in starts.made)


async def test_a_start_without_a_failed_import_runs_once(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    rig = _rig(tmp_path)
    starts = _Starts([_plays(rig).identify])
    monkeypatch.setattr(fake_hermes_kit, "HermesGateway", starts)
    await fake_hermes_kit._start_gateway(rig)
    assert len(starts.made) == 1
    assert not starts.made[0].stopped
