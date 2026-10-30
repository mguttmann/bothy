# SPDX-License-Identifier: Apache-2.0
"""The real server of the adapter's end-to-end tests: sdkserver of services/server/test/sdkserver.

One server per session (``bothy_sdk.testing.sdkserver``): the server of this tree on a disposable
PostgreSQL with the world of 07 "Examples", Lotse's API token bound to a DPoP key. Each test runs
the adapter's own code (`HermesHandler`, `HermesAdapterLoop`) with the fake Hermes runtime of
``adapter_kit`` under a consumer of its own. Without the Go tool chain or a Docker-compatible
runtime the tests fail, they never skip.
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
    return "asyncio"


@pytest.fixture(scope="session")
def sdkserver_binary() -> Iterator[Path]:
    work = Path(tempfile.mkdtemp(prefix="bothy-hermes-sdkserver-"))
    try:
        yield build_sdkserver(work)
    finally:
        shutil.rmtree(work, ignore_errors=True)


@pytest.fixture(scope="session")
async def real(sdkserver_binary: Path) -> AsyncIterator[RealServer]:
    workdir = Path(tempfile.mkdtemp(prefix="bothy-hermes-sdkserver-run-"))
    os.chmod(workdir, 0o700)  # noqa: PTH101 - the token files live here
    # No virus scanner runs here: uploads become final with scan_status skipped (12 3.4).
    log = os.environ.get("BOTHY_E2E_SERVER_LOG")
    server = await start_sdkserver(
        sdkserver_binary,
        workdir,
        config={"file.scan.mode": "off"},
        log=Path(log) if log else None,
    )
    try:
        yield server
    finally:
        await server.stop()
        shutil.rmtree(workdir, ignore_errors=True)
