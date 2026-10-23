# SPDX-License-Identifier: Apache-2.0
"""The real server of the SDK scenarios: ``sdkserver`` of ``services/server/test/sdkserver``.

The fixture builds the test binary once per session (``go test -c -tags sdkserver``), starts it
with short real-time values of the server keys, and implements the ``ScenarioServer`` protocol of
``bothy_sdk.testing.scenarios`` as `RealServer` over its JSON-line protocol. It needs the Go tool
chain and a Docker-compatible runtime; without them the tests fail, they never skip (brief
M2-29, Boundaries). The process is stopped with its ``stop`` op and waited for; only when it does
not exit is it killed, by PID.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import shutil
import subprocess
import tempfile
from collections.abc import AsyncIterator, Iterator, Mapping
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest

from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.clock import SystemClock
from bothy_sdk.settings import SdkSettings, parse_duration
from bothy_sdk.testing.scenarios import RunView, Triggered

_ANSWER = "SDKSERVER "
# Short real-time values (services/server/test/sdkserver/README.md "Timing values"); the
# adapter leeway stays at most auth.clock_leeway (09 4.5 "Keys and rotation" point 2).
TIMINGS = {
    "agent.invocation_token_ttl": "5s",
    "auth.clock_leeway": "1s",
    "agent.ack_timeout": "2s",
    "agent.ack_persist_interval": "100ms",
    "sync.heartbeat_interval": "2s",
}
SDK_SETTINGS = SdkSettings(
    invocation_token_leeway=1.0,
    reconnect_backoff_base=0.1,
    reconnect_backoff_cap=0.5,
    retry_backoff_base=0.1,
    retry_backoff_cap=1.0,
    poll_interval=0.2,
)
_STOP_WAIT = 60.0


def repository_root() -> Path:
    """The directory that holds ``docs/api`` above this file."""
    for parent in Path(__file__).resolve().parents:
        if (parent / "docs" / "api").is_dir():
            return parent
    raise RuntimeError("repository root not found")


class RealServer:
    """``ScenarioServer`` over a running ``sdkserver`` process."""

    def __init__(self, proc: asyncio.subprocess.Process, workdir: Path) -> None:
        """Wrap a started process; call `start` next."""
        self._proc = proc
        self._dir = workdir
        self._lock = asyncio.Lock()
        self.clock = SystemClock()
        self.sdk_settings = SDK_SETTINGS
        self.timings: Mapping[str, float] = {k: parse_duration(v) for k, v in TIMINGS.items()}
        self.origin = ""
        self.base_url = ""
        self.instance_id = UUID(int=0)
        self.agent_id = UUID(int=0)
        self.other_agent_id = UUID(int=0)
        self.credentials = Credentials(Secret("unset"))

    async def _call(self, request: dict[str, Any], timeout: float = 300.0) -> dict[str, Any]:
        async with self._lock:
            assert self._proc.stdin is not None
            assert self._proc.stdout is not None
            self._proc.stdin.write((json.dumps(request) + "\n").encode())
            await self._proc.stdin.drain()
            while True:
                line = await asyncio.wait_for(self._proc.stdout.readline(), timeout)
                if not line:
                    raise RuntimeError(f"sdkserver ended during {request['op']}")
                text = line.decode(errors="replace").strip()
                if text.startswith(_ANSWER):
                    answer: dict[str, Any] = json.loads(text[len(_ANSWER) :])
                    if answer.get("op") == "error":
                        raise RuntimeError(f"sdkserver {request['op']}: {answer.get('detail')}")
                    return answer

    def _read_token(self, path: str) -> Secret:
        token = Secret(Path(path).read_text(encoding="utf-8").strip())
        Path(path).unlink()
        return token

    async def start(self) -> None:
        """Start the server and read Lotse's credentials."""
        key = FileDpopKey.generate(self._dir / "dpop.pem")
        ready = await self._call(
            {"op": "start", "config": TIMINGS, "agent_jkt": key.jkt, "dir": str(self._dir)}
        )
        self.origin = ready["origin"]
        self.base_url = self.origin + "/api/v1"
        self.instance_id = UUID(ready["instance_id"])
        self.agent_id = UUID(ready["agent_id"])
        self.other_agent_id = UUID(ready["other_agent_id"])
        self.credentials = Credentials(self._read_token(ready["token_file"]), key)

    async def trigger(self, text: str = "Hello @Lotse") -> Triggered:
        """Alex opens a thread in Support/Allgemein with ``text``; Lotse's run is created."""
        a = await self._call({"op": "trigger", "text": text})
        return Triggered(UUID(a["run_id"]), UUID(a["thread_id"]), UUID(a["message_id"]))

    async def forge_token(self, run_id: UUID, variant: str) -> Secret:
        """A forged token of ``variant`` for ``run_id``."""
        a = await self._call({"op": "forge_token", "run_id": str(run_id), "variant": variant})
        return self._read_token(a["token_file"])

    async def run_state(self, run_id: UUID) -> RunView:
        """State, reason, version and starts of a run."""
        a = await self._call({"op": "run", "run_id": str(run_id)})
        return RunView(a["state"], a.get("state_reason"), int(a["version"]), int(a["starts"]))

    async def restart(self, consumer: str | None = None, acked_seq: int | None = None) -> None:
        """Restart the server on the same database, rewinding ``consumer`` to ``acked_seq``."""
        await self._call({"op": "restart", "consumer": consumer, "acked_seq": acked_seq})

    async def pass_time(self, seconds: float) -> None:
        """Let real time pass."""
        await asyncio.sleep(seconds)

    async def stop(self) -> None:
        """Stop the program with ``stop`` and wait for its exit; kill it by PID only if needed."""
        with contextlib.suppress(Exception):
            await self._call({"op": "stop"}, timeout=_STOP_WAIT)
        try:
            await asyncio.wait_for(self._proc.wait(), _STOP_WAIT)
        except TimeoutError:
            self._proc.kill()
            await self._proc.wait()


@pytest.fixture(scope="session")
def anyio_backend() -> str:
    """One event loop for the whole session, so the server fixture can be session-scoped."""
    return "asyncio"


@pytest.fixture(scope="session")
def sdkserver_binary() -> Iterator[Path]:
    """Build the test binary of ``sdkserver`` once per session."""
    root = repository_root()
    work = Path(tempfile.mkdtemp(prefix="bothy-sdkserver-"))
    binary = work / "sdkserver.test"
    go = shutil.which("go")
    if go is None:
        raise RuntimeError("the real-server tests need the Go tool chain (go not on PATH)")
    subprocess.run(  # noqa: S603 - fixed arguments, no shell
        [go, "test", "-c", "-tags", "sdkserver", "-o", str(binary), "./test/sdkserver"],
        cwd=root / "services" / "server",
        check=True,
    )
    try:
        yield binary
    finally:
        shutil.rmtree(work, ignore_errors=True)


@pytest.fixture(scope="session")
async def real(sdkserver_binary: Path) -> AsyncIterator[RealServer]:
    """A running real server for the session."""
    workdir = Path(tempfile.mkdtemp(prefix="bothy-sdkserver-run-"))
    os.chmod(workdir, 0o700)  # noqa: PTH101 - the token files live here
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
    server = RealServer(proc, workdir)
    try:
        await server.start()
        yield server
    finally:
        await server.stop()
        shutil.rmtree(workdir, ignore_errors=True)
