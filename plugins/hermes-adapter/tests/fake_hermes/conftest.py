# SPDX-License-Identifier: Apache-2.0
"""Session fixture of the Fake-Hermes tests: the pinned Hermes with the adapter (Design note 20)."""

from __future__ import annotations

from collections.abc import Iterator

import pytest
from fake_hermes_kit import HermesWorld, provision_world


@pytest.fixture(scope="session")
def world(request: pytest.FixtureRequest) -> Iterator[HermesWorld]:
    """Provision the pinned version once and add the adapter (fails without network)."""
    yield from provision_world(request)
