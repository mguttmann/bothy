# SPDX-License-Identifier: Apache-2.0
"""The world of the pass tests: fakes, configuration file, secret files and a runtime."""

from __future__ import annotations

import io
import json
import secrets
from collections.abc import Iterator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import pytest
from fakes import FakeClock, FakeCostApi, FakeServer, FakeStop

from cost_reconciler.__main__ import Runtime, run

A1 = "00000000-0000-7000-8000-0000000000a1"
A2 = "00000000-0000-7000-8000-0000000000a2"
A4 = "00000000-0000-7000-8000-0000000000a4"
W1, W2, W3 = "wrkspc_01e2ew1", "wrkspc_01e2ew2", "wrkspc_01e2ew3"
DAY1 = "2026-10-20T00:00:00.000Z"
DAY2 = "2026-10-19T00:00:00.000Z"


def run_id(n: int) -> str:
    return f"00000000-0000-7000-8000-{n:012d}"


R1, R2, R3, R4, R5 = (run_id(n) for n in range(1, 6))


def estimate(run: str, agent: str, micros: int, currency: str = "USD") -> dict[str, Any]:
    return {
        "run_id": run,
        "agent_id": agent,
        "provider": "anthropic",
        "currency": currency,
        "estimate_micros": micros,
        "reconciled": False,
    }


@dataclass
class World:
    """Fakes, files and a captured stderr for one invocation of the app."""

    tmp: Path
    cost_api: FakeCostApi
    server: FakeServer
    clock: FakeClock
    admin_key: str = field(default_factory=lambda: "canary" + secrets.token_hex(16))
    token: str = field(default_factory=lambda: "canary" + secrets.token_hex(16))
    stderr: io.StringIO = field(default_factory=io.StringIO)
    workspace_map: dict[str, list[str]] = field(default_factory=lambda: {W1: [A1, A2], W2: [A4]})
    extra: str = 'min_request_interval = "60s"\n'
    secrets_written: bool = False

    def write(self) -> Path:
        token_file = self.tmp / "token"
        key_file = self.tmp / "admin_key"
        if not self.secrets_written:
            # Once per test, so a test can change or remove a secret file afterwards.
            for path, value in ((token_file, self.token), (key_file, self.admin_key)):
                path.write_text(value + "\n", encoding="utf-8")
                path.chmod(0o600)
            self.secrets_written = True
        lines = [
            "[reconciler]",
            f'server_url = "{self.server.url}"',
            f'cost_api_url = "{self.cost_api.url}"',
            f'token_ref = "file:{token_file}"',
            f'admin_key_ref = "file:{key_file}"',
            self.extra,
            "[reconciler.workspace_map]",
            *(f"{ws} = {json.dumps(agents)}" for ws, agents in self.workspace_map.items()),
        ]
        config = self.tmp / "config.toml"
        config.write_text("\n".join(lines) + "\n", encoding="utf-8")
        return config

    def runtime(self, stop: FakeStop | None = None) -> Runtime:
        return Runtime(
            wall_now=self.clock.wall_now,
            monotonic=self.clock.monotonic,
            sleep=self.clock.sleep,
            stop=stop or FakeStop(self.clock),
            stderr=self.stderr,
            environ={},
        )

    def run(self, *args: str, stop: FakeStop | None = None) -> int:
        return run(["--config", str(self.write()), *args], self.runtime(stop))

    def logs(self) -> list[dict[str, Any]]:
        return [json.loads(line) for line in self.stderr.getvalue().splitlines()]

    def events(self, name: str) -> list[dict[str, Any]]:
        return [line for line in self.logs() if line["event"] == name]


@pytest.fixture
def world(tmp_path: Path) -> Iterator[World]:
    cost_api = FakeCostApi()
    server = FakeServer()
    server.estimates[DAY1] = [
        estimate(R1, A1, 2_000_000),
        estimate(R2, A1, 0),
        estimate(R3, A2, 1_000_000),
        estimate(R4, A2, 500_000, "EUR"),
        estimate(R5, A4, 1_500_000),
    ]
    try:
        yield World(tmp=tmp_path, cost_api=cost_api, server=server, clock=FakeClock())
    finally:
        server.close()
        cost_api.close()
