# SPDX-License-Identifier: Apache-2.0
"""The real server of the live test: ``sdkserver`` of ``services/server/test/sdkserver``.

Brief M2-50 Design note 20: the fixture finds the repository root (the directory with
``docs/api``), builds the test binary once per session (``go test -c -tags sdkserver``), runs it
with ``-test.run '^TestSDKServer$'`` and drives it over its JSON-line protocol on stdin: ``start``
with ``app_front {host: "jira-app.example.com"}``, then ``create_forum``, ``app_backend`` and
``call``, at the end ``stop`` and then the PID. No token is printed: the program writes the
answers of ``call`` into files of mode ``0600``, which this driver reads and removes. Without
Docker, Go or ``uv`` the live test fails; it never skips. The no-network guard of
``tests/conftest.py`` holds for the whole session.
"""

from __future__ import annotations

import asyncio
import shutil
import subprocess
import tempfile
from collections.abc import AsyncIterator, Iterator
from pathlib import Path

import pytest

from tests.guard import install_guard
from tests.live.sdkserver import LiveServer, repository_root


@pytest.fixture(scope="session")
def anyio_backend() -> str:
    """One event loop for the whole session, so the server fixture can be session-scoped."""
    return "asyncio"


@pytest.fixture(scope="session", autouse=True)
def session_guard() -> Iterator[None]:
    """The no-network guard for the session-scoped fixtures as well."""
    with pytest.MonkeyPatch.context() as patch:
        install_guard(patch)
        yield


@pytest.fixture(scope="session")
def sdkserver_binary() -> Iterator[Path]:
    """Build the test binary of ``sdkserver`` once per session."""
    root = repository_root()
    work = Path(tempfile.mkdtemp(prefix="jira-app-sdkserver-"))
    go = shutil.which("go")
    if go is None:
        raise RuntimeError("the live test needs the Go tool chain (go not on PATH)")
    binary = work / "sdkserver.test"
    try:
        subprocess.run(  # noqa: S603 - fixed arguments, no shell
            [go, "test", "-c", "-tags", "sdkserver", "-o", str(binary), "./test/sdkserver"],
            cwd=root / "services" / "server",
            check=True,
        )
        yield binary
    finally:
        shutil.rmtree(work, ignore_errors=True)


@pytest.fixture(scope="session")
async def live_server(sdkserver_binary: Path) -> AsyncIterator[LiveServer]:
    """A running real server with the TLS front of `HOST`."""
    workdir = Path(tempfile.mkdtemp(prefix="jira-app-live-"))
    workdir.chmod(0o700)
    proc = await asyncio.create_subprocess_exec(
        str(sdkserver_binary),
        "-test.run",
        "^TestSDKServer$",
        "-test.timeout",
        "0",
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.DEVNULL,
        cwd=str(repository_root() / "services" / "server" / "test" / "sdkserver"),
    )
    server = LiveServer(proc, workdir)
    try:
        await server.start()
        yield server
    finally:
        await server.stop()
        shutil.rmtree(workdir, ignore_errors=True)
