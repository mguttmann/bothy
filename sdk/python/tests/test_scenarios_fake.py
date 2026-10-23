# SPDX-License-Identifier: Apache-2.0
"""AG-08, AG-09, AG-12, AG-28 and AZ-P6 against the FakeServer (the same scenario functions run
against the real server in tests/realserver)."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest

from bothy_sdk.testing.fakeserver import FakeServer
from bothy_sdk.testing.scenarios import (
    ag08_invalid_tokens,
    ag09_double_delivery,
    ag12_patch_rules,
    ag28_slow_start_and_restart,
    azp6_adapter_never_starts,
)


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer() as server:
        yield server


@pytest.mark.anyio
async def test_ag08_fake(fake: FakeServer) -> None:
    await ag08_invalid_tokens(fake)


@pytest.mark.anyio
async def test_azp6_fake(fake: FakeServer) -> None:
    await azp6_adapter_never_starts(fake)


@pytest.mark.anyio
async def test_ag09_fake(fake: FakeServer) -> None:
    await ag09_double_delivery(fake)


@pytest.mark.anyio
async def test_ag12_fake(fake: FakeServer) -> None:
    await ag12_patch_rules(fake)


@pytest.mark.anyio
async def test_ag28_fake(fake: FakeServer) -> None:
    await ag28_slow_start_and_restart(fake)
