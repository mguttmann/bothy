# SPDX-License-Identifier: Apache-2.0
"""The FakeServer's gateway for agents (private).

Frames are ``{"op": ..., "d": ...}`` (docs/api/asyncapi.yaml, channel ``gateway``;
docs/spec/04-sync-protocol.md 6): ``hello``, ``identify`` or ``resume`` with the API token and a
DPoP proof (``htm = GET``, ``htu`` the HTTPS form of the gateway URL as the server builds it from
its public URL, the nonce of ``hello``), ``ready``, the consumer's dispatches with
``invocation_token`` and ``redelivered``, ``cursor``, ``catchup.complete``, ``heartbeat_ack``,
``error`` frames for failing ops and the close codes of the server (``4015`` for a replaced
consumer, ``4013`` on a restart).
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import re
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Any

from aiohttp import WSMsgType, web

from bothy_sdk.ids import uuid7
from bothy_sdk.testing._fake_base import FakeProblem
from bothy_sdk.testing._fake_dpop import Authenticator
from bothy_sdk.testing._fake_hub import Connection, Hub
from bothy_sdk.testing._fake_world import World, consumer_ok

_REGISTERED = frozenset(
    {
        "ack",
        "focus",
        "heartbeat",
        "identify",
        "reauth",
        "resume",
        "run.update",
        "typing.start",
        "typing.stop",
        "command.autocomplete_result",
    }
)
# Close codes after which a session stays resumable within sync.resume_window.
_RESUMABLE = frozenset({1000, 1001, 4000, 4001, 4002, 4009, 4011, 4013, 4014})
_IDENTIFY = frozenset(
    {
        "protocol_version",
        "access_token",
        "api_token",
        "dpop",
        "device_id",
        "consumer",
        "cursor",
        "sync_epoch",
        "client",
    }
)
_RESUME = frozenset(
    {"gateway_session_id", "access_token", "api_token", "dpop", "cursor", "sync_epoch"}
)
_UUID = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")


class _Close(Exception):  # noqa: N818 - a control-flow signal, not an error
    def __init__(self, code: int, reason: str = "") -> None:
        super().__init__(code)
        self.code = code
        self.reason = reason


@dataclass
class GatewaySession:
    """A gateway session that ``resume`` can continue."""

    consumer: str
    detached_at: float | None = None
    owner: Connection | None = None


def _int(value: object, minimum: int = 0) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value >= minimum


class Gateway:
    """Serves ``/api/v1/gateway``.

    Args:
        world: Data and rules.
        hub: Consumers and deliveries.
        auth: The credential checks.
        htu: The ``htu`` a proof must name.
        take_fault: Returns True when a fault is queued for a gateway op (it drops the
            connection).
        record: Records a received client frame.
    """

    def __init__(  # noqa: PLR0913, PLR0917 - the parts the gateway needs
        self,
        world: World,
        hub: Hub,
        auth: Authenticator,
        htu: Callable[[], str],
        take_fault: Callable[[str], bool],
        record: Callable[[dict[str, Any]], None],
    ) -> None:
        self.world = world
        self.hub = hub
        self.auth = auth
        self.htu = htu
        self.take_fault = take_fault
        self.record = record
        self.sessions: dict[str, GatewaySession] = {}
        self.sockets: dict[web.WebSocketResponse, Callable[[int, str], Awaitable[None]]] = {}
        self.draining = False
        self.frame_limit = 1 << 20

    async def handle(self, request: web.Request) -> web.StreamResponse:
        """One WebSocket connection from ``hello`` to its close."""
        ws = web.WebSocketResponse(max_msg_size=0, autoping=True)
        await ws.prepare(request)
        state: dict[str, Any] = {"conn": None, "sent_code": None}
        lock = asyncio.Lock()

        async def send(frame: dict[str, Any]) -> None:
            async with lock:
                if not ws.closed:
                    await ws.send_str(json.dumps(frame))

        async def close(code: int, reason: str) -> None:
            # The code the server sent decides whether the session stays resumable; the code the
            # peer echoes back may differ.
            if state["sent_code"] is None:
                state["sent_code"] = code
            await ws.close(code=code, message=reason.encode())

        self.sockets[ws] = close
        try:
            if self.draining:
                raise _Close(4013, "server_restart")
            await send(
                {
                    "op": "hello",
                    "d": {
                        "heartbeat_interval_ms": int(
                            self.world.timings["sync.heartbeat_interval"] * 1000
                        ),
                        "resume_window_ms": int(self.world.timings["sync.resume_window"] * 1000),
                        "protocol_versions": [1],
                        "dpop_nonce": self.auth.nonces.current(),
                        "server_time": self.world.now_iso(),
                        "sync_epoch": str(self.world.sync_epoch),
                    },
                }
            )
            async for msg in ws:
                if msg.type != WSMsgType.TEXT:
                    raise _Close(4002, "binary_frame")
                if len(msg.data.encode()) > self.frame_limit:
                    raise _Close(4002, "frame_too_large")
                await self._frame(msg.data, state, send, close, request)
        except _Close as closing:
            state["sent_code"] = closing.code
            await self._end(state, closing.code)
            await close(closing.code, closing.reason or "closed")
        except _Dropped:
            await self._end(state, 1006)
            if request.transport is not None:
                request.transport.close()
        finally:
            await self._end(state, state["sent_code"] or ws.close_code or 1006)
            self.sockets.pop(ws, None)
        return ws

    async def _end(self, state: dict[str, Any], code: int) -> None:
        conn: Connection | None = state.get("conn")
        if conn is None:
            return
        state["conn"] = None
        await self.hub.detach(conn)
        session = self.sessions.get(conn.session_id)
        if session is not None and session.owner is conn:
            session.owner = None
            if code in _RESUMABLE or code == 1006:
                session.detached_at = self.world.clock.monotonic()
            else:
                self.sessions.pop(conn.session_id, None)

    async def _frame(  # noqa: PLR0912 - the dispatch table of the ops
        self,
        text: str,
        state: dict[str, Any],
        send: Callable[[dict[str, Any]], Any],
        close: Callable[[int, str], Any],
        request: web.Request,
    ) -> None:
        try:
            frame = json.loads(text)
        except ValueError:
            raise _Close(4002, "decode_error") from None
        if not isinstance(frame, dict) or set(frame) - {"op", "d"}:
            raise _Close(4002, "decode_error")
        op = frame.get("op")
        if not isinstance(op, str) or not op:
            raise _Close(4002, "decode_error")
        self.record(frame)
        if op not in _REGISTERED:
            raise _Close(4001, "unknown_op")
        identified = state["conn"] is not None
        if not identified and op not in ("identify", "resume"):
            raise _Close(4003, "not_identified")
        if identified and op in ("identify", "resume"):
            raise _Close(4005, "already_identified")
        d = frame.get("d")
        if not isinstance(d, dict):
            raise _Close(4002, "decode_error")
        if self.take_fault(op):
            raise _Dropped
        if op == "identify":
            await self._identify(d, state, send, close)
        elif op == "resume":
            await self._resume(d, state, send, close)
        elif op == "heartbeat":
            await self._heartbeat(d, send)
        elif op == "ack":
            if set(d) - {"seq"} or not _int(d.get("seq")):
                raise _Close(4002, "decode_error")
            if not self.hub.gateway_ack(state["conn"], d["seq"]):
                await send({"op": "error", "d": {"op": "ack", "code": "validation_failed"}})
        elif op == "run.update":
            await self._run_update(d, send)
        elif op == "command.autocomplete_result":
            self._autocomplete(d)

    def _authenticate(self, d: dict[str, Any]) -> None:
        tokens = [n for n in ("access_token", "api_token") if n in d]
        if len(tokens) != 1 or ("cursor" in d and not _int(d["cursor"])):
            raise _Close(4002, "decode_error")
        if tokens[0] != "api_token" or not isinstance(d["api_token"], str):
            raise _Close(4004, "authentication_failed")
        scheme = "DPoP" if "dpop" in d else "Bearer"
        proofs = [d["dpop"]] if isinstance(d.get("dpop"), str) else []
        try:
            self.auth.authenticate(
                method="GET",
                htu=self.htu(),
                authorization=[f"{scheme} {d['api_token']}"],
                proofs=proofs,
            )
        except FakeProblem:
            raise _Close(4004, "authentication_failed") from None

    async def _identify(
        self, d: dict[str, Any], state: dict[str, Any], send: Any, close: Any
    ) -> None:
        consumer = d.get("consumer", "default")
        if set(d) - _IDENTIFY or not isinstance(consumer, str) or not consumer_ok(consumer):
            raise _Close(4002, "decode_error")
        if not _int(d.get("protocol_version"), 0):
            raise _Close(4002, "decode_error")
        if d["protocol_version"] != 1:
            raise _Close(4007, "protocol_version_unsupported")
        self._authenticate(d)
        session_id = str(uuid7())
        self.sessions[session_id] = GatewaySession(consumer)
        principal = self.world.lotse.ref()
        head = self.hub.head
        snapshot = {
            "head_seq": head,
            "permission_version": self.world.permission_version,
            "sync_epoch": str(self.world.sync_epoch),
            "principal": principal,
            "is_owner": False,
            "spaces": [],
            "channels": [],
            "sidebar": [],
            "unread_threads": [],
            "active_runs": [],
            "active_threads": [],
            "agents": [],
            "settings": [],
            "drafts": [],
            "features": [],
        }
        await send(
            {
                "op": "ready",
                "d": {
                    "gateway_session_id": session_id,
                    "principal": principal,
                    "permission_version": self.world.permission_version,
                    "head_seq": head,
                    "snapshot": snapshot,
                    "sync_epoch": str(self.world.sync_epoch),
                },
            }
        )
        state["conn"] = await self.hub.attach(consumer, session_id, send, close)
        self.sessions[session_id].owner = state["conn"]

    async def _resume(
        self, d: dict[str, Any], state: dict[str, Any], send: Any, close: Any
    ) -> None:
        session_id = d.get("gateway_session_id")
        if set(d) - _RESUME or not isinstance(session_id, str) or not _int(d.get("cursor")):
            raise _Close(4002, "decode_error")
        self._authenticate(d)
        session = self.sessions.get(session_id)
        window = self.world.timings["sync.resume_window"]
        if session is None or (
            session.detached_at is not None
            and self.world.clock.monotonic() - session.detached_at >= window
        ):
            self.sessions.pop(session_id, None)
            raise _Close(4006, "session_invalid")
        old = session.owner
        if old is not None:
            session.owner = None
            await self.hub.detach(old)
            with contextlib.suppress(Exception):
                await old.close(4015, "consumer_replaced")
        session.detached_at = None
        await send(
            {
                "op": "resumed",
                "d": {
                    "gateway_session_id": session_id,
                    "head_seq": self.hub.head,
                    "sync_epoch": str(self.world.sync_epoch),
                },
            }
        )
        holder = self.hub.holders.get(session.consumer)
        if holder is not None and holder != session_id:
            # The fenced save of the resumed session loses: a newer holder took the consumer.
            self.sessions.pop(session_id, None)
            raise _Close(4015, "consumer_replaced")
        state["conn"] = await self.hub.attach(session.consumer, session_id, send, close)
        session.owner = state["conn"]

    async def _heartbeat(self, d: dict[str, Any], send: Any) -> None:
        if set(d) - {"last_seq", "agent"} or not _int(d.get("last_seq")):
            raise _Close(4002, "decode_error")
        await send({"op": "heartbeat_ack", "d": {"server_time": self.world.now_iso()}})
        if "agent" in d:
            try:
                self.world.heartbeat(d["agent"])
            except FakeProblem as problem:
                await send({"op": "error", "d": {"op": "heartbeat", "code": problem.code}})

    async def _run_update(self, d: dict[str, Any], send: Any) -> None:
        if (
            set(d) - {"run_id", "base_version", "ops", "ref"}
            or not isinstance(d.get("run_id"), str)
            or not _UUID.match(d["run_id"])
            or not _int(d.get("base_version"), -(2**63))
            or not isinstance(d.get("ops"), list)
            or ("ref" in d and not isinstance(d["ref"], str))
        ):
            raise _Close(4002, "decode_error")
        try:
            self.world.patch(d["run_id"], d["base_version"], d["ops"])
        except FakeProblem as problem:
            error: dict[str, Any] = {"op": "run.update", "code": problem.code}
            if problem.code == "validation_failed" and problem.errors:
                first = problem.errors[0]
                error["detail"] = f"{first['pointer']}: {first['code']}"
            elif problem.reason and problem.code != "base_version_conflict":
                error["detail"] = problem.reason
            if d.get("ref"):
                error["ref"] = d["ref"]
            await send({"op": "error", "d": error})

    def _autocomplete(self, d: dict[str, Any]) -> None:
        if set(d) - {"request_id", "choices", "ref"} or not isinstance(d.get("choices"), list):
            raise _Close(4002, "decode_error")
        try:
            from uuid import UUID  # noqa: PLC0415 - only here

            request_id = UUID(str(d.get("request_id")))
        except ValueError:
            raise _Close(4002, "decode_error") from None
        pending = self.world.autocomplete.get(request_id)
        if pending is None or self.world.now().timestamp() > pending["expires"]:
            return
        self.world.autocomplete_answers.setdefault(request_id, list(d["choices"]))

    async def close_all(self, code: int, reason: str) -> None:
        """Close every socket, identified or not."""
        await self.hub.close_all(code, reason)
        for close in list(self.sockets.values()):
            with contextlib.suppress(Exception):
                await close(code, reason)

    def forget_sessions(self) -> None:
        """Drop every session (a process restart)."""
        self.sessions.clear()


class _Dropped(Exception):  # noqa: N818 - a control-flow signal for the disconnect fault
    pass
