# SPDX-License-Identifier: Apache-2.0
"""Every phase of G7 draws its cases from ``CONTRACT_SEED`` (flake list #238)."""

from __future__ import annotations

import importlib
import shutil
import sys
from collections.abc import Iterator
from pathlib import Path
from types import ModuleType

import pytest

REPO = Path(__file__).resolve().parents[3]
SEED = 238


@pytest.fixture(scope="module")
def live() -> Iterator[ModuleType]:
    """``live_contract`` imported with the environment the Go test sets; nothing is called."""
    env = {
        "BOTHY_CONTRACT_BASE_URL": "http://127.0.0.1:9/api/v1",
        "BOTHY_CONTRACT_BROKER_URL": "http://127.0.0.1:9",
        "BOTHY_CONTRACT_SPEC": str(REPO / "docs" / "api" / "openapi.yaml"),
        "BOTHY_CONTRACT_OPERATIONS": str(REPO / "services" / "server" / "api" / "operations.yaml"),
        "CONTRACT_SEED": str(SEED),
    }
    with pytest.MonkeyPatch.context() as mp:
        for key, value in env.items():
            mp.setenv(key, value)
        sys.modules.pop("live_contract", None)
        module = importlib.import_module("live_contract")
    try:
        yield module
    finally:
        sys.modules.pop("live_contract", None)
        shutil.rmtree(module.KEY_DIR, ignore_errors=True)


def test_passes_use_the_seed(live: ModuleType) -> None:
    for test in (live.test_user_pass, live.test_owner_pass):
        assert test._hypothesis_internal_use_seed == SEED


def test_stateful_phase_uses_the_seed(live: ModuleType) -> None:
    # run_state_machine_as_test takes the seed from the state machine class.
    assert live._OwnerMachine.__dict__.get("_hypothesis_internal_use_seed") == SEED
