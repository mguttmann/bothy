# SPDX-License-Identifier: Apache-2.0
"""The real server of the SDK scenarios: ``sdkserver`` of ``services/server/test/sdkserver``.

The fixture builds the test binary once per session and starts it with short real-time values of
the server keys; `bothy_sdk.testing.sdkserver.RealServer` implements the ``ScenarioServer``
protocol of ``bothy_sdk.testing.scenarios`` over its JSON-line protocol. It needs the Go tool
chain and a Docker-compatible runtime; without them the tests fail, they never skip (brief
M2-29, Boundaries). The process is stopped with its ``stop`` op and waited for; only when it does
not exit is it killed, by PID.
"""

from __future__ import annotations

import os
import shutil
import tempfile
from collections.abc import AsyncIterator, Iterator
from pathlib import Path

import pytest

from bothy_sdk.testing.sdkserver import RealServer, build_sdkserver, start_sdkserver


@pytest.fixture(scope="session")
def anyio_backend() -> str:
    """One event loop for the whole session, so the server fixture can be session-scoped."""
    return "asyncio"


@pytest.fixture(scope="session")
def sdkserver_binary() -> Iterator[Path]:
    """Build the test binary of ``sdkserver`` once per session."""
    work = Path(tempfile.mkdtemp(prefix="bothy-sdkserver-"))
    try:
        yield build_sdkserver(work)
    finally:
        shutil.rmtree(work, ignore_errors=True)


@pytest.fixture(scope="session")
async def real(sdkserver_binary: Path) -> AsyncIterator[RealServer]:
    """A running real server for the session."""
    workdir = Path(tempfile.mkdtemp(prefix="bothy-sdkserver-run-"))
    os.chmod(workdir, 0o700)  # noqa: PTH101 - the token files live here
    server = await start_sdkserver(sdkserver_binary, workdir)
    try:
        yield server
    finally:
        await server.stop()
        shutil.rmtree(workdir, ignore_errors=True)
