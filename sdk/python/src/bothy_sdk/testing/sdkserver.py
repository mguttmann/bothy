# SPDX-License-Identifier: Apache-2.0
"""The real server of this tree for tests: a driver of ``services/server/test/sdkserver``.

`build_sdkserver` builds the test binary of sdkserver (``go test -c -tags sdkserver``) and
`start_sdkserver` runs it and starts the server with short real-time values. `RealServer` speaks
its JSON-line protocol (``services/server/test/sdkserver/README.md``): it implements the
``ScenarioServer`` protocol of `bothy_sdk.testing.scenarios` and offers the world of the program
(the space Support, the channel Allgemein and its main thread, Alex and the owner) and the op
``call``, any operation of ``docs/api/openapi.yaml`` as Alex or the owner, so the end-to-end
tests of the SDK and of the adapters under ``plugins/`` drive the same server. It needs the Go
tool chain and a Docker-compatible runtime; without them the tests fail, they never skip.

No token is ever printed: the program writes tokens into files of mode ``0600``, and `RealServer`
reads each once and removes it.

Examples:
    >>> TIMINGS["auth.clock_leeway"]
    '1s'
"""

from __future__ import annotations

import asyncio
import base64
import contextlib
import hashlib
import json
import shutil
import struct
import subprocess  # nosec B404 - the test driver builds and starts the sdkserver program
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any
from uuid import UUID

from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.clock import SystemClock
from bothy_sdk.settings import SdkSettings, parse_duration
from bothy_sdk.testing.scenarios import RunView, Triggered

__all__ = [
    "SDK_SETTINGS",
    "TIMINGS",
    "ApiAnswer",
    "RealServer",
    "World",
    "build_sdkserver",
    "m4a",
    "repository_root",
    "start_sdkserver",
]

_ANSWER = "SDKSERVER "
TIMINGS: Mapping[str, str] = {
    "agent.invocation_token_ttl": "5s",  # nosec B105 - a duration, not a secret
    "auth.clock_leeway": "1s",
    "agent.ack_timeout": "2s",
    "agent.ack_persist_interval": "100ms",
    "sync.heartbeat_interval": "2s",
}
"""Short real-time values of the server keys (sdkserver README "Timing values")."""
SDK_SETTINGS = SdkSettings(
    invocation_token_leeway=1.0,
    reconnect_backoff_base=0.1,
    reconnect_backoff_cap=0.5,
    retry_backoff_base=0.1,
    retry_backoff_cap=1.0,
    poll_interval=0.2,
)
"""The SDK side of `TIMINGS`; the adapter leeway stays at most ``auth.clock_leeway`` (09 4.5)."""
_STOP_WAIT = 60.0
_OP_WAIT = 300.0


def repository_root() -> Path:
    """The directory that holds ``docs/api`` above this file.

    Raises:
        RuntimeError: This file is not inside a checkout of the repository.
    """
    for parent in Path(__file__).resolve().parents:
        if (parent / "docs" / "api").is_dir() and (parent / "services" / "server").is_dir():
            return parent
    raise RuntimeError("repository root not found")


def build_sdkserver(work: Path) -> Path:
    """Build the test binary of sdkserver into ``work``.

    Args:
        work: An existing directory.

    Returns:
        The binary.

    Raises:
        RuntimeError: The Go tool chain is not on ``PATH``.
    """
    go = shutil.which("go")
    if go is None:
        raise RuntimeError("the real-server tests need the Go tool chain (go not on PATH)")
    binary = work / "sdkserver.test"
    subprocess.run(  # noqa: S603 - fixed arguments, no shell  # nosec B603
        [go, "test", "-c", "-tags", "sdkserver", "-o", str(binary), "./test/sdkserver"],
        cwd=repository_root() / "services" / "server",
        check=True,
    )
    return binary


@dataclass(frozen=True)
class World:
    """The parts of the world of 07 "Examples" that the program names in ``ready``."""

    space_id: UUID
    channel_id: UUID
    main_thread_id: UUID
    alex_id: UUID
    owner_id: UUID


@dataclass(frozen=True)
class ApiAnswer:
    """Status and JSON body of one ``call``."""

    status: int
    body: Any

    def member(self, *path: str) -> Any:
        """The member at ``path`` of the body (``None`` when one is missing)."""
        value = self.body
        for key in path:
            if not isinstance(value, dict):
                return None
            value = value.get(key)
        return value


class RealServer:
    """``ScenarioServer`` over a running sdkserver process, with the program's other ops."""

    def __init__(
        self,
        proc: asyncio.subprocess.Process,
        workdir: Path,
        config: Mapping[str, str] | None = None,
    ) -> None:
        """Wrap a started process; call `start` next (``config`` adds server keys)."""
        self._proc = proc
        self._dir = workdir
        self._config = {**TIMINGS, **(config or {})}
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
        self.dpop_key_file = workdir / "dpop.pem"
        self.world = World(UUID(int=0), UUID(int=0), UUID(int=0), UUID(int=0), UUID(int=0))

    async def op(self, request: dict[str, Any], timeout: float = _OP_WAIT) -> dict[str, Any]:
        """Send one request line and return its answer.

        Raises:
            RuntimeError: The program answered ``error`` or ended.
        """
        async with self._lock:
            stdin, stdout = self._proc.stdin, self._proc.stdout
            if stdin is None or stdout is None:
                raise RuntimeError("sdkserver was started without pipes")
            stdin.write((json.dumps(request) + "\n").encode())
            await stdin.drain()
            while True:
                line = await asyncio.wait_for(stdout.readline(), timeout)
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
        """Start the server and read Lotse's credentials and the world."""
        key = FileDpopKey.generate(self.dpop_key_file)
        ready = await self.op(
            {"op": "start", "config": self._config, "agent_jkt": key.jkt, "dir": str(self._dir)}
        )
        self.origin = ready["origin"]
        self.base_url = self.origin + "/api/v1"
        self.instance_id = UUID(ready["instance_id"])
        self.agent_id = UUID(ready["agent_id"])
        self.other_agent_id = UUID(ready["other_agent_id"])
        self.credentials = Credentials(self._read_token(ready["token_file"]), key)
        w = ready["world"]
        self.world = World(
            UUID(w["space_id"]),
            UUID(w["channel_id"]),
            UUID(w["main_thread_id"]),
            UUID(w["alex_id"]),
            UUID(w["owner_id"]),
        )

    async def trigger(self, text: str = "Hello @Lotse") -> Triggered:
        """Alex opens a thread in Support/Allgemein with ``text``; Lotse's run is created."""
        a = await self.op({"op": "trigger", "text": text})
        return Triggered(UUID(a["run_id"]), UUID(a["thread_id"]), UUID(a["message_id"]))

    async def wait_run(self, message_id: UUID) -> tuple[UUID, UUID]:
        """The run that ``message_id`` triggered and its thread (an auto thread too)."""
        a = await self.op({"op": "wait_run", "message_id": str(message_id)})
        return UUID(a["run_id"]), UUID(a["thread_id"])

    async def create_forum(self, name: str) -> UUID:
        """The owner creates a forum ``name`` in the space Support; Lotse operates it."""
        a = await self.op({"op": "create_forum", "name": name})
        return UUID(a["channel_id"])

    async def call(  # noqa: PLR0913 - the parts of one API request
        self,
        operation_id: str,
        *,
        as_: str = "alex",
        path: Mapping[str, Any] | None = None,
        query: Mapping[str, Any] | None = None,
        body: Any = None,
        raw: bytes | None = None,
        header: Mapping[str, str] | None = None,
    ) -> ApiAnswer:
        """Perform an operation of ``docs/api/openapi.yaml`` as ``alex`` or ``owner``.

        Args:
            operation_id: The operation.
            as_: ``alex`` or ``owner`` (the owner with a fresh step-up).
            path: The path parameters.
            query: The query parameters.
            body: The JSON body.
            raw: A body sent as it is instead of ``body`` (``uploadPart``).
            header: Extra request headers (``Content-Type`` of ``raw``, ``If-Match``).

        Returns:
            Status and body; the answer file is removed.
        """
        a = await self.op(
            {
                "op": "call",
                "as": as_,
                "operation_id": operation_id,
                "path_params": {k: str(v) for k, v in (path or {}).items()},
                "query": {k: str(v) for k, v in (query or {}).items()},
                "body": body,
                "raw_body": base64.b64encode(raw).decode() if raw is not None else "",
                "header": dict(header or {}),
            }
        )
        file = Path(a["file"])
        result = json.loads(file.read_text(encoding="utf-8"))
        file.unlink()
        return ApiAnswer(int(result["status"]), result.get("body"))

    async def upload(
        self, channel_id: UUID, name: str, data: bytes, mime: str, *, as_: str = "alex"
    ) -> UUID:
        """Upload one file for the channel (``createUpload``, ``uploadPart``, complete).

        Returns:
            The file, final once the scan (or ``file.scan.mode`` ``off``) has ended.
        """
        digest = hashlib.sha256(data)
        session = await self.call(
            "createUpload",
            as_=as_,
            path={"channel_id": channel_id},
            body={
                "name": name,
                "size_bytes": len(data),
                "sha256": digest.hexdigest(),
                "declared_mime_type": mime,
            },
        )
        _expect(session, 201)
        file_id = UUID(session.body["file_id"])
        part = session.body["parts"][0]
        header = {h["name"]: h["value"] for h in part.get("headers") or []}
        header["Content-Type"] = "application/octet-stream"
        header["Content-Digest"] = "sha-256=:" + base64.b64encode(digest.digest()).decode() + ":"
        done = await self.call(
            "uploadPart",
            as_=as_,
            path={"file_id": file_id, "part_number": 1},
            raw=data,
            header=header,
        )
        _expect(done, 200)
        completed = await self.call(
            "completeUpload",
            as_=as_,
            path={"file_id": file_id},
            body={"parts": [{"part_number": 1, "etag": done.body["etag"]}]},
        )
        _expect(completed, 200)
        return file_id

    async def forge_token(self, run_id: UUID, variant: str) -> Secret:
        """A forged token of ``variant`` for ``run_id``."""
        a = await self.op({"op": "forge_token", "run_id": str(run_id), "variant": variant})
        return self._read_token(a["token_file"])

    async def run_state(self, run_id: UUID) -> RunView:
        """State, reason, version and starts of a run."""
        a = await self.op({"op": "run", "run_id": str(run_id)})
        return RunView(a["state"], a.get("state_reason"), int(a["version"]), int(a["starts"]))

    async def restart(self, consumer: str | None = None, acked_seq: int | None = None) -> None:
        """Restart the server on the same database, rewinding ``consumer`` to ``acked_seq``."""
        await self.op({"op": "restart", "consumer": consumer, "acked_seq": acked_seq})

    async def pass_time(self, seconds: float) -> None:
        """Let real time pass."""
        await asyncio.sleep(seconds)

    async def stop(self) -> None:
        """Stop the program with ``stop`` and wait for its exit; kill it by PID only if needed."""
        with contextlib.suppress(Exception):
            await self.op({"op": "stop"}, timeout=_STOP_WAIT)
        try:
            await asyncio.wait_for(self._proc.wait(), _STOP_WAIT)
        except TimeoutError:
            self._proc.kill()
            await self._proc.wait()


def _expect(answer: ApiAnswer, status: int) -> None:
    if answer.status != status:
        raise RuntimeError(f"status {answer.status}, want {status}: {answer.body}")


def m4a(seconds: int = 2) -> bytes:
    """A minimal MP4 audio file a voice message accepts (12 8.1).

    ``ftyp`` M4A, a movie header with the duration and one sound track, no video track.

    Examples:
        >>> m4a()[4:12]
        b'ftypM4A '
    """

    def box(kind: bytes, *parts: bytes) -> bytes:
        body = b"".join(parts)
        return struct.pack(">I", 8 + len(body)) + kind + body

    def full(kind: bytes, *parts: bytes) -> bytes:
        return box(kind, b"\0\0\0\0", *parts)

    mvhd = full(b"mvhd", struct.pack(">IIII", 0, 0, 1000, seconds * 1000), bytes(80))
    tkhd = full(b"tkhd", struct.pack(">IIIII", 0, 0, 1, 0, 0), bytes(60))
    hdlr = full(b"hdlr", b"\0\0\0\0", b"soun", bytes(12), b"Voice\0")
    trak = box(b"trak", tkhd, box(b"mdia", box(b"mdhd", bytes(24)), hdlr, box(b"minf")))
    return box(b"ftyp", b"M4A ", b"\0\0\0\0") + box(b"moov", mvhd, trak) + box(b"mdat", bytes(64))


async def start_sdkserver(
    binary: Path,
    workdir: Path,
    *,
    config: Mapping[str, str] | None = None,
    log: Path | None = None,
) -> RealServer:
    """Run ``binary`` as the program and start the server; `RealServer.stop` ends it.

    Args:
        binary: The binary of `build_sdkserver`.
        workdir: An existing directory of mode ``0700`` for the token files.
        config: Server keys of ``docs/operations/configuration.md`` over `TIMINGS` (for
            example ``file.scan.mode`` ``off`` where no scanner runs).
        log: A file for the program's standard error (the server log); none by default.

    Returns:
        The started server.
    """
    stderr = log.open("ab") if log is not None else None
    proc = await asyncio.create_subprocess_exec(
        str(binary),
        "-test.run",
        "^TestSDKServer$",
        "-test.timeout",
        "0",
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=stderr if stderr is not None else asyncio.subprocess.DEVNULL,
        cwd=str(repository_root() / "services" / "server" / "test" / "sdkserver"),
    )
    if stderr is not None:
        stderr.close()  # the child holds its own descriptor
    server = RealServer(proc, workdir, config)
    try:
        await server.start()
    except BaseException:
        await server.stop()
        raise
    return server
