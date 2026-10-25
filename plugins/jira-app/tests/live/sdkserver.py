# SPDX-License-Identifier: Apache-2.0
"""The driver of ``sdkserver`` (``services/server/test/sdkserver``) for the live test.

`LiveServer` speaks the JSON-line protocol of the program on stdin (answers ``SDKSERVER <json>``
on stdout) with the app ops of M2-49: ``start`` with ``app_front``, ``create_forum``,
``app_backend`` and ``call``. No token is printed: the program writes the answers of ``call``
into files of mode ``0600``, which this driver reads and removes.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from bothy_sdk.auth import FileDpopKey

_ANSWER = "SDKSERVER "
_CALL_WAIT = 300.0
_STOP_WAIT = 60.0
HOST = "jira-app.example.com"
# Generous bounds of the server: an answer of the app is never late on a loaded machine.
SERVER_CONFIG = {
    "app.interaction_timeout": "20s",
    "command.autocomplete_timeout": "20s",
    "http.request_timeout": "60s",
}


def repository_root() -> Path:
    """The directory that holds ``docs/api`` above this file."""
    for parent in Path(__file__).resolve().parents:
        if (parent / "docs" / "api").is_dir():
            return parent
    raise RuntimeError("repository root not found")


@dataclass(frozen=True)
class CallResult:
    """Status and JSON body of a ``call``."""

    status: int
    body: dict[str, Any]


class LiveServer:
    """A running ``sdkserver`` with the app ops of M2-49."""

    def __init__(self, proc: asyncio.subprocess.Process, workdir: Path) -> None:
        self._proc = proc
        self.dir = workdir
        self._lock = asyncio.Lock()
        self.origin = ""
        self.front_port = 0

    async def _send(self, request: dict[str, Any], timeout: float = _CALL_WAIT) -> dict[str, Any]:
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

    async def start(self) -> None:
        """Start the server with the TLS front for `HOST`."""
        key = FileDpopKey.generate(self.dir / "dpop.pem")
        ready = await self._send(
            {
                "op": "start",
                "config": SERVER_CONFIG,
                "agent_jkt": key.jkt,
                "dir": str(self.dir),
                "app_front": {"host": HOST},
            }
        )
        self.origin = str(ready["origin"])
        self.front_port = int(ready["front_port"])
        token = Path(str(ready["token_file"]))
        token.unlink(missing_ok=True)

    async def create_forum(self, name: str) -> str:
        """The owner creates a forum in the space Support; returns its ID."""
        return str((await self._send({"op": "create_forum", "name": name}))["channel_id"])

    async def app_backend(self, port: int) -> None:
        """Let the TLS front forward to the app's listener."""
        await self._send({"op": "app_backend", "port": port})

    async def call(
        self,
        who: str,
        operation_id: str,
        *,
        path: dict[str, str] | None = None,
        query: dict[str, str] | None = None,
        body: Any = None,
    ) -> CallResult:
        """Perform an operation of the public API as ``owner`` or ``alex``."""
        request: dict[str, Any] = {
            "op": "call",
            "as": who,
            "operation_id": operation_id,
            "path_params": path or {},
            "query": query or {},
        }
        if body is not None:
            request["body"] = body
        answer = await self._send(request)
        file = Path(str(answer["file"]))
        raw = json.loads(file.read_text(encoding="utf-8"))
        file.unlink()
        found = raw.get("body")
        return CallResult(int(raw["status"]), found if isinstance(found, dict) else {})

    async def stop(self) -> None:
        """Stop the program with ``stop`` and wait; kill it by PID only if it does not exit."""
        with contextlib.suppress(Exception):
            await self._send({"op": "stop"}, timeout=_STOP_WAIT)
        try:
            await asyncio.wait_for(self._proc.wait(), _STOP_WAIT)
        except TimeoutError:
            self._proc.kill()
            await self._proc.wait()
