# SPDX-License-Identifier: Apache-2.0
"""Gateway connection of an agent after docs/spec/04-sync-protocol.md 6.

`Gateway` opens the WebSocket of ``Discovery.gateway.url``, answers ``hello`` with ``identify``
(API token, DPoP proof with the nonce of ``hello``, consumer, protocol version) or ``resume``
within the resume window, sends the heartbeat (the first after a random part of the interval,
then every interval, with the agent heartbeat as member ``agent``), closes with ``1001`` and
reconnects when a ``heartbeat_ack`` is missing for one interval, and reconnects with full jitter
between ``sync.reconnect_backoff_base`` and ``sync.reconnect_backoff_cap``. Client frames are built
from the generated ``Gateway*`` models; server frames are decoded with them (unknown ops are
logged and skipped).

Close codes for agents (docs/DECISIONS.md D-M2-29-5, after 04 6.4):

* stop, no reconnect: ``1000`` (own close), ``4004``, ``4007``, ``4011``, ``4012``, ``4015``,
  ``4016``, ``4017``, ``4018`` (an agent cannot renew its API token; only the owner issues one);
* ``resume`` after backoff: ``1001``, ``4000``, ``4009``, ``4013`` (first try after a random part
  of ``sync.reconnect_backoff_base``), ``4014``;
* ``identify`` on ``4006``;
* ``4008``: wait the ``retry_after`` of the close reason, else backoff;
* log and reconnect: ``4001``, ``4002``, ``4003``, ``4005`` and unknown codes.

Examples:
    >>> CloseReason(4015, "consumer_taken_over").reconnects
    False
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import random
import re
import secrets
from collections.abc import AsyncIterator, Callable, Sequence
from dataclasses import dataclass
from typing import Any
from urllib.parse import urlsplit, urlunsplit
from uuid import UUID

import websockets
from websockets.asyncio.client import ClientConnection, connect
from websockets.exceptions import ConnectionClosed

from bothy_sdk.auth import Credentials, Secret, dpop_proof, gateway_htu, require_safe_url
from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.errors import GatewayOpError
from bothy_sdk.generated.models import (
    CommandChoice,
    GatewayAck,
    GatewayAutocompleteResult,
    GatewayCursor,
    GatewayError,
    GatewayHeartbeat,
    GatewayHello,
    GatewayIdentify,
    GatewayReady,
    GatewayResume,
    GatewayResumed,
    GatewayResyncRequired,
    GatewayRunUpdate,
    Heartbeat,
    JsonPatchOperation,
)
from bothy_sdk.settings import SdkSettings

__all__ = [
    "IDENTIFY_ON",
    "PROTOCOL_VERSIONS",
    "RESUME_ON",
    "STOP_ON",
    "CloseReason",
    "Gateway",
    "GatewayEvent",
    "gateway_url",
]

PROTOCOL_VERSIONS: tuple[int, ...] = (1,)
"""Gateway protocol versions this SDK speaks (``info.x-protocol-versions``)."""

STOP_ON = frozenset({1000, 4004, 4007, 4011, 4012, 4015, 4016, 4017, 4018})
"""Close codes after which an agent stops without reconnecting."""
RESUME_ON = frozenset({1001, 4000, 4009, 4013, 4014})
"""Close codes after which the gateway resumes after backoff."""
IDENTIFY_ON = frozenset({4006})
"""Close codes after which the gateway identifies anew."""

_LOG = logging.getLogger("bothy_sdk")
_RETRY_AFTER = re.compile(r"retry_after=(\d+)")
_OWN_CLOSE = 1000
_HEARTBEAT_TIMEOUT_CLOSE = 1001
_SERVER_RESTART = 4013
_RATE_LIMITED_CLOSE = 4008
_PRINCIPAL_MISMATCH = "principal_mismatch"
_ABNORMAL = 1006


@dataclass(frozen=True)
class CloseReason:
    """Why a gateway (or an adapter) stopped: the close code and its reason text.

    Attributes:
        code: The WebSocket close code (``1000`` for an own stop).
        reason: The close reason the server sent, or an own reason.
    """

    code: int
    reason: str = ""

    @property
    def reconnects(self) -> bool:
        """Whether an agent reconnects after this code."""
        return self.code not in STOP_ON


@dataclass(frozen=True)
class GatewayEvent:
    """One server frame that matters to an event source.

    Attributes:
        op: ``dispatch``, ``cursor``, ``ready``, ``resumed``, ``resync_required`` or ``error``.
        data: The raw ``d`` member (the envelope of a dispatch).
        model: The decoded generated model of the frame, when the op has one.
    """

    op: str
    data: dict[str, Any]
    model: Any = None


def gateway_url(base_url: str, url: str) -> str:
    """Resolve ``Discovery.gateway.url`` against the API base and turn it into a WebSocket URL.

    Args:
        base_url: The API base, for example ``https://example.com/api/v1``.
        url: The gateway URL of the discovery (absolute, or relative to the origin).

    Returns:
        The ``ws`` or ``wss`` URL.

    Raises:
        ValueError: The result would use clear text off the loopback host.

    Examples:
        >>> gateway_url("https://example.com/api/v1", "/api/v1/gateway")
        'wss://example.com/api/v1/gateway'
    """
    base = urlsplit(base_url)
    target = urlsplit(url)
    scheme = target.scheme or base.scheme
    netloc = target.netloc or base.netloc
    scheme = {"http": "ws", "https": "wss"}.get(scheme.lower(), scheme.lower())
    out = urlunsplit((scheme, netloc, target.path, target.query, ""))
    require_safe_url(out)
    return out


class Gateway:
    """One agent's gateway session with reconnects, as an iterator of server frames.

    Args:
        url: The WebSocket URL (`gateway_url`).
        credentials: The agent's API token and optional DPoP key.
        consumer: The consumer name (``^[a-z0-9][a-z0-9_-]*$``).
        agent_id: The agent's principal ID; ``ready`` for another principal stops the gateway.
        settings: Reconnect backoff and frame size.
        clock: The time source of heartbeats and waits.
        rng: The jitter source (default `secrets.SystemRandom`).
        heartbeat: Returns the agent heartbeat for the member ``agent`` (or ``None``).
        user_agent: The ``User-Agent`` of the upgrade request.
        nonce_sink: Receives each DPoP nonce of ``hello`` (the REST client shares it).
    """

    def __init__(  # noqa: PLR0913 - every input of the connection is explicit
        self,
        url: str,
        credentials: Credentials,
        *,
        consumer: str,
        agent_id: UUID,
        settings: SdkSettings | None = None,
        clock: Clock | None = None,
        rng: random.Random | None = None,
        heartbeat: Callable[[], Heartbeat | None] | None = None,
        user_agent: str | None = None,
        nonce_sink: Callable[[str | None], None] | None = None,
    ) -> None:
        """Create the gateway; nothing connects before `frames` is iterated."""
        require_safe_url(url)
        self.url = url
        self._credentials = credentials
        self.consumer = consumer
        self.agent_id = agent_id
        self.settings = settings or SdkSettings()
        self.clock = clock or SystemClock()
        self._rng = rng or secrets.SystemRandom()
        self._heartbeat = heartbeat
        self._user_agent = user_agent
        self._nonce_sink = nonce_sink
        self._ws: ClientConnection | None = None
        self._session_id: UUID | None = None
        self._resume_window = 0.0
        self._disconnected_at: float | None = None
        self._identify_next = False
        self._last_seq = 0
        self._ack_pending = False
        self._closing = False
        self._send_lock = asyncio.Lock()
        self.close_reason: CloseReason | None = None
        self.connects = 0
        self._beats: asyncio.Task[None] | None = None

    @property
    def session_id(self) -> UUID | None:
        """The gateway session of the last ``ready``, if any."""
        return self._session_id

    @property
    def last_seq(self) -> int:
        """The highest acknowledged ``seq`` (the cursor of a ``resume``)."""
        return self._last_seq

    async def _send(self, op: str, data: dict[str, Any]) -> None:
        ws = self._ws
        if ws is None:
            raise ConnectionError("gateway not connected")
        async with self._send_lock:
            await ws.send(json.dumps({"op": op, "d": data}, separators=(",", ":")))

    async def ack(self, seq: int) -> None:
        """Acknowledge every event up to ``seq`` (op ``ack``, cumulative).

        Args:
            seq: The highest settled ``seq``.
        """
        # No local deduplication: after a reconnect the server's cursor may lie below ours, and
        # an ack at or below the server's cursor changes nothing there (04 15.2).
        self._last_seq = max(self._last_seq, seq)
        with contextlib.suppress(ConnectionError, ConnectionClosed):
            await self._send("ack", GatewayAck(seq=seq).to_dict())

    async def send_run_update(
        self,
        run_id: UUID,
        base_version: int,
        ops: Sequence[JsonPatchOperation],
        ref: str | None = None,
    ) -> None:
        """Send the op ``run.update`` (no success reply; a failure comes as an ``error`` frame).

        Args:
            run_id: The run.
            base_version: The version the patch is based on.
            ops: The RFC 6902 operations.
            ref: A reference the ``error`` frame repeats.
        """
        frame = GatewayRunUpdate(run_id=run_id, base_version=base_version, ops=list(ops))
        if ref is not None:
            frame.ref = ref
        await self._send("run.update", frame.to_dict())

    async def send_autocomplete_result(
        self, request_id: UUID, choices: Sequence[CommandChoice], ref: str | None = None
    ) -> None:
        """Send the op ``command.autocomplete_result``.

        Args:
            request_id: The request of the ``command.autocomplete_requested`` event.
            choices: The choices.
            ref: A reference the ``error`` frame repeats.
        """
        frame = GatewayAutocompleteResult(request_id=request_id, choices=list(choices))
        if ref is not None:
            frame.ref = ref
        await self._send("command.autocomplete_result", frame.to_dict())

    async def close(self) -> None:
        """Close with ``1000``; `frames` ends without reconnecting."""
        self._closing = True
        if self.close_reason is None:
            self.close_reason = CloseReason(_OWN_CLOSE, "closed by the adapter")
        ws = self._ws
        if ws is not None:
            await ws.close(_OWN_CLOSE, "closed by the adapter")

    def _backoff(self, attempt: int) -> float:
        cap = min(
            self.settings.reconnect_backoff_cap,
            self.settings.reconnect_backoff_base * (2**attempt),
        )
        return self._rng.uniform(0.0, cap)

    def _identify_frame(self, hello: GatewayHello) -> tuple[str, dict[str, Any]]:
        token: Secret = self._credentials.api_token
        proof = None
        if self._credentials.dpop is not None:
            proof = dpop_proof(
                self._credentials.dpop,
                htm="GET",
                htu=gateway_htu(self.url),
                token=token,
                nonce=hello.dpop_nonce,
                clock=self.clock,
            )
        common = set(hello.protocol_versions) & set(PROTOCOL_VERSIONS)
        version = max(common) if common else PROTOCOL_VERSIONS[-1]
        resume_ok = (
            not self._identify_next
            and self._session_id is not None
            and self._disconnected_at is not None
            and self.clock.monotonic() - self._disconnected_at < self._resume_window
        )
        if resume_ok and self._session_id is not None:
            resume = GatewayResume(
                gateway_session_id=self._session_id, cursor=self._last_seq, api_token=token.reveal()
            )
            if proof is not None:
                resume.dpop = proof
            return "resume", resume.to_dict()
        identify = GatewayIdentify(
            protocol_version=version, api_token=token.reveal(), consumer=self.consumer
        )
        if proof is not None:
            identify.dpop = proof
        return "identify", identify.to_dict()

    async def frames(self) -> AsyncIterator[GatewayEvent]:
        """Connect, keep the session alive and yield every relevant server frame.

        The iterator reconnects after every close code that allows it and ends when the
        gateway stops; `close_reason` then tells why.

        Yields:
            Server frames: ``ready``, ``resumed``, ``dispatch``, ``cursor``,
            ``resync_required`` and ``error`` (with a `GatewayOpError` as model).
        """
        attempt = 0
        while not self._closing:
            code: int | None = None
            reason = ""
            try:
                async with connect(
                    self.url,
                    max_size=self.settings.ws_max_frame_bytes,
                    ping_interval=None,
                    user_agent_header=self._user_agent,
                    open_timeout=self.settings.http_timeout,
                ) as ws:
                    self._ws = ws
                    self.connects += 1
                    async for raw in ws:
                        event = await self._handle(ws, raw)
                        if event is not None:
                            if event.op in ("ready", "resumed"):
                                attempt = 0
                            yield event
                        if self._stopped():
                            break
                    code, reason = ws.close_code, ws.close_reason or ""
            except ConnectionClosed as exc:
                code = exc.rcvd.code if exc.rcvd is not None else None
                reason = exc.rcvd.reason if exc.rcvd is not None else ""
            except (OSError, websockets.exceptions.InvalidHandshake, TimeoutError) as exc:
                _LOG.info("gateway connect failed: %s", type(exc).__name__)
            finally:
                await self._end_connection()
            if self._stopped():
                break
            wait = self._after_close(code if code is not None else _ABNORMAL, reason, attempt)
            if wait is None:
                break
            attempt += 1
            await self.clock.sleep(wait)

    def _stopped(self) -> bool:
        return self._closing

    async def _end_connection(self) -> None:
        beats = self._beats
        if beats is not None:
            beats.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await beats
        self._beats = None
        self._ws = None
        self._ack_pending = False
        self._disconnected_at = self.clock.monotonic()

    def _after_close(self, code: int, reason: str, attempt: int) -> float | None:
        """Return the wait before the next connect, or ``None`` to stop (D-M2-29-5)."""
        _LOG.info("gateway closed code=%s", code)
        if code in STOP_ON:
            self.close_reason = CloseReason(code, reason)
            return None
        if code in IDENTIFY_ON:
            self._identify_next = True
        if code == _SERVER_RESTART and attempt == 0:
            return self._rng.uniform(0.0, self.settings.reconnect_backoff_base)
        if code == _RATE_LIMITED_CLOSE:
            match = _RETRY_AFTER.search(reason)
            if match is not None:
                return float(match.group(1))
        return self._backoff(attempt)

    async def _handle(  # noqa: PLR0911 - one answer per server op
        self, ws: ClientConnection, raw: str | bytes
    ) -> GatewayEvent | None:
        frame = _decode(raw)
        if frame is None:
            return None
        op, data = frame
        if op == "hello":
            await self._on_hello(ws, GatewayHello.from_dict(data))
            return None
        if op == "ready":
            return await self._on_ready(ws, data)
        if op == "heartbeat_ack":
            self._ack_pending = False
            return None
        if op == "resync_required":
            self._identify_next = True
            self._session_id = None
            return GatewayEvent(op, data, GatewayResyncRequired.from_dict(data))
        if op == "error":
            err = GatewayError.from_dict(data)
            detail = err.detail if isinstance(err.detail, str) else None
            ref = err.ref if isinstance(err.ref, str) else None
            return GatewayEvent(op, data, GatewayOpError(err.op, err.code, detail, ref))
        decoders: dict[str, Callable[[dict[str, Any]], Any] | None] = {
            "dispatch": None,
            "resumed": GatewayResumed.from_dict,
            "cursor": GatewayCursor.from_dict,
        }
        if op in decoders:
            decode = decoders[op]
            return GatewayEvent(op, data, decode(data) if decode is not None else None)
        if op != "catchup.complete":
            _LOG.info("gateway: unknown op %s skipped", op)
        return None

    async def _on_hello(self, ws: ClientConnection, hello: GatewayHello) -> None:
        self._resume_window = hello.resume_window_ms / 1000.0
        if self._nonce_sink is not None:
            self._nonce_sink(hello.dpop_nonce)
        name, payload = self._identify_frame(hello)
        await self._send(name, payload)
        self._beats = asyncio.create_task(self._beat(ws, hello.heartbeat_interval_ms / 1000.0))

    async def _on_ready(self, ws: ClientConnection, data: dict[str, Any]) -> GatewayEvent | None:
        ready = GatewayReady.from_dict(data)
        if ready.principal.id != self.agent_id:
            _LOG.error("gateway ready for another principal; stopping")
            self.close_reason = CloseReason(_OWN_CLOSE, _PRINCIPAL_MISMATCH)
            self._closing = True
            await ws.close(_OWN_CLOSE, _PRINCIPAL_MISMATCH)
            return None
        self._session_id = ready.gateway_session_id
        self._identify_next = False
        return GatewayEvent("ready", data, ready)

    async def _beat(self, ws: ClientConnection, interval: float) -> None:
        """Heartbeat loop of one connection (04 6.3): random first delay, then every interval."""
        await self.clock.sleep(self._rng.random() * interval)
        while True:
            if self._ack_pending:
                _LOG.info("gateway: heartbeat_ack missing; reconnecting")
                await ws.close(_HEARTBEAT_TIMEOUT_CLOSE, "heartbeat_ack missing")
                return
            beat = GatewayHeartbeat(last_seq=self._last_seq)
            agent = self._heartbeat() if self._heartbeat is not None else None
            if agent is not None:
                beat.agent = agent
            self._ack_pending = True
            try:
                await self._send("heartbeat", beat.to_dict())
            except (ConnectionError, ConnectionClosed):
                return
            await self.clock.sleep(interval)


def _decode(raw: str | bytes) -> tuple[str, dict[str, Any]] | None:
    try:
        frame = json.loads(raw)
    except ValueError:
        _LOG.info("gateway: undecodable frame skipped")
        return None
    if not isinstance(frame, dict) or not isinstance(frame.get("op"), str):
        return None
    data = frame.get("d")
    return frame["op"], data if isinstance(data, dict) else {}
