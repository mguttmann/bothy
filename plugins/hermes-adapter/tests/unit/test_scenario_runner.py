# SPDX-License-Identifier: Apache-2.0
"""The scenario runner judges what Hermes does while it shuts down (stand-in gateway)."""

from __future__ import annotations

import shutil
import types
import urllib.request
from typing import Any

import pytest
from hermes_harness import scenarios
from hermes_harness.fake_model import CredentialKind
from hermes_harness.fixtures import make_root
from hermes_harness.scenarios import Scenario, ScenarioRunner

FOREIGN_KEY = "sk-ant-api03-" + "z" * 40


class _ShutdownGateway:
    """Stands in for Hermes: does nothing until ``stop()``, then calls the model."""

    def __init__(self, run: Any) -> None:
        self.run = run

    def start(self) -> None:
        pass

    def listening_sockets(self) -> list[str]:
        return []

    def stop(self) -> None:
        # Hermes' shutdown work (memory extraction, session end hooks) can reach the model.
        request = urllib.request.Request(  # noqa: S310 - loopback URL of the fake model
            self.run.model.openai_base_url + "/models", headers={"x-api-key": FOREIGN_KEY}
        )
        urllib.request.urlopen(request, timeout=5).read()  # noqa: S310 - loopback URL


def test_a_foreign_credential_during_shutdown_fails_the_run(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(scenarios._Run, "prepare", lambda run: _ShutdownGateway(run))  # noqa: PLW0108
    monkeypatch.setattr(scenarios._Run, "wait_ready", lambda self, gateway: None)
    monkeypatch.setattr(scenarios, "SETTLE_S", 0.0)
    scenario = Scenario(
        name="shutdown",
        point="p",
        provider="anthropic",
        credentials={},
        probe={},
        config={},
        allowlist=(),
        texts={},
        model_script=(),
        steps=(),
    )
    root = make_root()  # short enough for the Unix sockets of the Hermes home
    try:
        runner = ScenarioRunner(types.SimpleNamespace(), root=root)  # type: ignore[arg-type]
        result = runner.run(scenario)
    finally:
        shutil.rmtree(root)
    assert [r["credential"] for r in result.model_requests] == [CredentialKind.UNEXPECTED]
    assert any("unexpected credential" in f for f in result.failures), result.failures
    assert FOREIGN_KEY not in repr(result)
