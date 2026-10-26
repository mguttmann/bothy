# SPDX-License-Identifier: Apache-2.0
"""The Fake-Agent: a scriptable adapter on the SDK for the agent conformance suite.

docs/spec/09-agents.md 16.1 row "Fake agent": the conductor of ``services/server/test/agents``
starts one process per agent and consumer (``python -m bothy_sdk.testing.fake_agent``) and drives
it over the JSON-line control protocol of `bothy_sdk.testing.control` on stdin and stdout. The
Fake-Agent decides nothing on its own: every request names one op (receive, start, patch,
finish, post, ...), which it carries out against the server through the public API, REST or
gateway, with the SDK hand layer (DPoP auth, invocation verifier, ack window, gateway, patcher,
card builders, command declarations, context rendering). Requests the hand layer would never
send (a forged, empty or absent token, a stale ``base_version``, a start past the adapter's own
checks) go out as plain requests of the same authenticated client.

Rules the process keeps:

* Credentials arrive only in ``hello`` on stdin, never in ``argv`` or the environment, and live
  only in memory (docs/DECISIONS.md D-M2-31-5).
* Logs go to stderr at level ``WARNING`` and never carry a token, a credential, message
  content, file content or a transcript; stdout carries only replies, which name a file's
  content only by its byte length. A token is named by `jti_hash`, the first 16 hex
  characters of the SHA-256 of its ``jti``. Only ``export_token`` returns a token.
* The wall time of the SDK is the harness clock: every request carries ``now`` and the
  Fake-Agent sets its clock to it before it acts (token expiry, ``iat`` of DPoP proofs, the
  ``max-age`` of the cached JWKS). Waits of the SDK (retry backoff, reconnects) use real time
  with the small values the conductor passes in ``settings``.

Examples:
    >>> agent = FakeAgent()
    >>> sorted(agent.ops())[:3]
    ['autocomplete_mode', 'await_input', 'connect']
"""

from __future__ import annotations

import asyncio
import base64
import contextlib
import hashlib
import json
import logging
import os
import sys
import time
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import decode_dss_signature

from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import Client
from bothy_sdk.commands import CommandSpec
from bothy_sdk.components import approval_card, button
from bothy_sdk.context import render_context
from bothy_sdk.errors import PROBLEM_STATUS, ProblemError, problem_error
from bothy_sdk.gateway import Gateway, GatewayEvent, gateway_url
from bothy_sdk.generated.models import (
    CommandOption,
    JsonPatchOperation,
    RunContext,
    RunContextAttachment,
)
from bothy_sdk.heartbeat import build_heartbeat
from bothy_sdk.inbox import MemoryInboxStore
from bothy_sdk.invocation import (
    Actual,
    InvocationVerifier,
    TokenVerdict,
    expectation_for,
    stored_claims,
)
from bothy_sdk.runs import RunPatcher
from bothy_sdk.settings import SdkSettings
from bothy_sdk.sources import AckWindow, Envelope, envelope_from
from bothy_sdk.testing.control import (
    ControlError,
    ErrorKind,
    Reply,
    Request,
    encode_reply,
    parse_request,
)
from bothy_sdk.testing.token_faults import VARIANTS, forge

__all__ = ["ControlledClock", "FakeAgent", "MemoryDpopKey", "jti_hash", "main"]

_LOG = logging.getLogger("bothy_sdk.fake_agent")
_CRASH_STATUS = 3
_TOKEN_TYPES = frozenset(
    {
        "agent.invocation",
        "agent.followup",
        "interaction.created",
        "command.invoked",
        "command.autocomplete_requested",
    }
)
# 09 4.9: the reason of the well-behaved finishRun after a refused token.
_REFUSAL_REASON = {
    TokenVerdict.INVALID: "invalid_invocation_token",
    TokenVerdict.BILLING_MODE_MISMATCH: "billing_mode_mismatch",
    TokenVerdict.TOOL_CLASS_MISMATCH: "tool_class_mismatch",
}


def jti_hash(jti: str) -> str:
    """Name a token in output: the first 16 hex characters of the SHA-256 of its ``jti``.

    Args:
        jti: The token ID.

    Returns:
        The hash prefix.
    """
    return hashlib.sha256(jti.encode()).hexdigest()[:16]


def _token_hash(token: Secret) -> str:
    """The `jti_hash` of a token, or of its whole text when it has no readable ``jti``."""
    try:
        return jti_hash(stored_claims(token).jti)
    except ValueError:
        return hashlib.sha256(token.reveal().encode()).hexdigest()[:16]


class ControlledClock:
    """The SDK clock of the Fake-Agent: wall time is the harness clock of the last request.

    `monotonic` is the harness time plus the real time since the process started, so it never
    goes back, follows every jump of the harness clock (the JWKS ``max-age`` expires when the
    conductor moves the clock past it) and still moves while the SDK waits in real time.
    """

    def __init__(self) -> None:
        """Start at the real time until the first `set`."""
        self._now = datetime.now(UTC)
        self._start = time.monotonic()
        self._set = False

    def set(self, now: datetime) -> None:
        """Set the wall time; the clock never goes back.

        After the first call an earlier time than the current one is ignored.

        Args:
            now: The harness clock.
        """
        if not self._set or now > self._now:
            self._now = now
            self._set = True

    def now(self) -> datetime:
        """Return the harness wall time."""
        return self._now

    def monotonic(self) -> float:
        """Return the harness time plus the real time since the start, in seconds."""
        return self._now.timestamp() + (time.monotonic() - self._start)

    async def sleep(self, seconds: float) -> None:
        """Sleep in real time.

        Args:
            seconds: The wait; negative counts as zero.
        """
        await asyncio.sleep(max(0.0, seconds))


def _unb64(text: str) -> bytes:
    return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))


def _b64(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


class MemoryDpopKey:
    """An ES256 DPoP key from a private JWK held only in memory (`DpopKeySource`).

    Args:
        jwk: The private JWK (``kty`` ``EC``, ``crv`` ``P-256``, ``x``, ``y``, ``d``).

    Raises:
        ValueError: The JWK is not a private P-256 key.
    """

    algorithm = "ES256"

    def __init__(self, jwk: Mapping[str, Any]) -> None:
        """Load the key; see the class docstring."""
        if jwk.get("kty") != "EC" or jwk.get("crv") != "P-256" or "d" not in jwk:
            raise ValueError("a DPoP key must be a private P-256 JWK")
        x = int.from_bytes(_unb64(str(jwk["x"])), "big")
        y = int.from_bytes(_unb64(str(jwk["y"])), "big")
        d = int.from_bytes(_unb64(str(jwk["d"])), "big")
        public = ec.EllipticCurvePublicNumbers(x, y, ec.SECP256R1())
        self._key = ec.EllipticCurvePrivateNumbers(d, public).private_key()

    def public_jwk(self) -> dict[str, str]:
        """Return the public JWK."""
        numbers = self._key.public_key().public_numbers()
        return {
            "kty": "EC",
            "crv": "P-256",
            "x": _b64(numbers.x.to_bytes(32, "big")),
            "y": _b64(numbers.y.to_bytes(32, "big")),
        }

    def sign(self, data: bytes) -> bytes:
        """Return the raw ES256 signature (r || s) of ``data``.

        Args:
            data: The JWS signing input.
        """
        r, s = decode_dss_signature(self._key.sign(data, ec.ECDSA(hashes.SHA256())))
        return r.to_bytes(32, "big") + s.to_bytes(32, "big")

    def __repr__(self) -> str:
        """Never show the key."""
        return "MemoryDpopKey(redacted)"


@dataclass
class _Entry:
    """One received event in the inbox of the Fake-Agent."""

    envelope: Envelope
    token: Secret | None
    verdict: str = ""
    held: bool = False
    settled: bool = False
    handled: bool = False


@dataclass
class _Answer:
    """One HTTP answer as the result members of an op."""

    status: int
    body: Any = None
    problem: ProblemError | None = None
    headers: Mapping[str, str] = field(default_factory=dict)
    # The raw body, for downloadFile; never part of a reply.
    content: bytes = field(default=b"", repr=False)

    def members(self) -> dict[str, Any]:
        out: dict[str, Any] = {"status": self.status}
        if self.problem is not None:
            out["code"] = self.problem.code
            if self.problem.reason:
                out["reason"] = self.problem.reason
            if self.problem.errors:
                out["errors"] = [
                    {"code": e.code, "pointer": e.pointer} for e in self.problem.errors
                ]
        return out


def _payload(entry: _Entry) -> dict[str, Any]:
    payload = entry.envelope.raw.get("payload")
    return payload if isinstance(payload, dict) else {}


def _run_of(entry: _Entry) -> str | None:
    p = _payload(entry)
    run = p.get("run_id")
    if run is None and isinstance(p.get("interaction"), dict):
        run = p["interaction"].get("run_id")
    return str(run) if run is not None else None


def _thread_of(entry: _Entry) -> str | None:
    p = _payload(entry)
    thread = p.get("thread_id")
    if thread is None and isinstance(p.get("interaction"), dict):
        thread = p["interaction"].get("thread_id")
    return str(thread) if thread is not None else None


class FakeAgent:
    """The state and the ops of one Fake-Agent process.

    Create it, then call `handle` with each request; the first request must be ``hello``.
    """

    def __init__(self) -> None:
        """Create an agent without a connection."""
        self.clock = ControlledClock()
        self._hello: dict[str, Any] = {}
        self._client: Client | None = None
        self._settings = SdkSettings()
        self._verifier: InvocationVerifier | None = None
        self._replay = MemoryInboxStore()
        self._window = AckWindow()
        self._entries: dict[UUID, _Entry] = {}
        self._order: list[UUID] = []
        self._tokens: dict[str, Secret] = {}
        self._run_token: dict[str, Secret] = {}
        self._started_token: dict[str, Secret] = {}
        self._active: list[str] = []
        self._patchers: dict[str, RunPatcher] = {}
        self._versions: dict[str, list[int]] = {}
        self._gateway: Gateway | None = None
        self._frames: asyncio.Queue[GatewayEvent] | None = None
        self._reader: asyncio.Task[None] | None = None
        self._ready = asyncio.Event()
        self._gateway_errors: list[dict[str, Any]] = []
        self._autocomplete: dict[str, Any] = {"mode": "answer", "choices": []}
        self._requested: list[dict[str, Any]] = []
        # Texts of files read with read_file, for the data blocks of `context` (never replied).
        self._file_texts: dict[UUID, str] = {}
        self._bind = True
        self._ops: dict[str, Callable[[dict[str, Any]], Awaitable[dict[str, Any]]]] = {
            "hello": self._op_hello,
            "connect": self._op_connect,
            "receive": self._op_receive,
            "verify": self._op_verify,
            "start": self._op_start,
            "resume": self._op_resume,
            "renew": self._op_renew,
            "context": self._op_context,
            "patch": self._op_patch,
            "await_input": self._op_await_input,
            "finish": self._op_finish,
            "heartbeat": self._op_heartbeat,
            "post": self._op_post,
            "respond": self._op_respond,
            "update_component": self._op_update_component,
            "set_title": self._op_set_title,
            "report_session": self._op_report_session,
            "register_commands": self._op_register_commands,
            "autocomplete_mode": self._op_autocomplete_mode,
            "read_file": self._op_read_file,
            "submit_transcript": self._op_submit_transcript,
            "export_token": self._op_export_token,
            "import_token": self._op_import_token,
            "crash": self._op_crash,
        }

    def ops(self) -> list[str]:
        """Return the names of the ops."""
        return list(self._ops)

    async def handle(self, request: Request) -> Reply:  # noqa: PLR0911 - one reply per outcome
        """Serve one request.

        Args:
            request: The request.

        Returns:
            The reply; an error never ends the process.
        """
        if request.now is not None:
            self.clock.set(request.now)
        op = self._ops.get(request.op)
        if op is None:
            return Reply.failure(request.id, ErrorKind.USAGE, f"unknown op {request.op}")
        if request.op not in ("hello", "crash") and self._client is None:
            return Reply.failure(request.id, ErrorKind.USAGE, "hello must come first")
        # `bind: false` sends this op's requests without the run's Invocation-Token (07 SR9),
        # as a request outside the run would go.
        self._bind = request.args.get("bind", True) is not False
        try:
            return Reply.success(request.id, await op(request.args))
        except ControlError as exc:
            return Reply.failure(request.id, exc.kind, exc.detail)
        except httpx.TransportError as exc:
            return Reply.failure(request.id, ErrorKind.TRANSPORT, type(exc).__name__)
        except (KeyError, TypeError, ValueError) as exc:
            return Reply.failure(request.id, ErrorKind.USAGE, f"bad argument: {type(exc).__name__}")
        except Exception as exc:
            _LOG.warning("op %s failed: %s", request.op, type(exc).__name__)
            return Reply.failure(request.id, ErrorKind.INTERNAL, type(exc).__name__)

    async def close(self) -> None:
        """Stop the gateway reader and close the HTTP client."""
        if self._gateway is not None:
            await self._gateway.close()
        if self._reader is not None:
            self._reader.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await self._reader
        for patcher in self._patchers.values():
            await patcher.aclose()
        if self._client is not None:
            await self._client.aclose()

    # ---- plumbing ------------------------------------------------------------------------

    @property
    def client(self) -> Client:
        """The REST client of ``hello``."""
        if self._client is None:  # pragma: no cover - handle checks it
            raise ControlError(ErrorKind.USAGE, "hello must come first")
        return self._client

    def _runtime(self, name: str, args: Mapping[str, Any]) -> Any:
        if name in args:
            return args[name]
        return self._hello.get("runtime", {}).get(name)

    async def _call(  # noqa: PLR0913 - every part of one request is explicit
        self,
        method: str,
        path: str,
        body: Any = None,
        *,
        run: str | None = None,
        headers: Mapping[str, str] | None = None,
        params: Mapping[str, Any] | None = None,
        client: Client | None = None,
    ) -> _Answer:
        """Send one request with the SDK client; a run binds the request to its token (SR9)."""
        c = client or self.client
        token = self._run_token.get(run) if run is not None and self._bind else None
        kwargs: dict[str, Any] = {"headers": dict(headers or {})}
        if body is not None:
            kwargs["json"] = body
        if params is not None:
            kwargs["params"] = params
        if token is not None:
            with c.bind_run(UUID(run), token):
                response = await c.http.request(method, path, **kwargs)
        else:
            response = await c.http.request(method, path, **kwargs)
        try:
            data = response.json() if response.content else None
        except ValueError:
            data = None
        if 200 <= response.status_code < 300:
            return _Answer(
                response.status_code, data, headers=response.headers, content=response.content
            )
        return _Answer(
            response.status_code, data, problem_error(response.status_code, data), response.headers
        )

    def _resolve(self, name: Any, run: str | None) -> Secret | None:
        """Resolve a token name.

        ``inbox`` is the newest of the run, ``empty`` an empty text, ``absent`` ``None``,
        ``forged:<variant>`` a forgery of the newest, anything else a kept name.
        """
        name = "inbox" if name is None else str(name)
        if name == "absent":
            return None
        if name == "empty":
            return _EmptyToken()  # type: ignore[return-value]
        if name.startswith("forged:"):
            variant = name.split(":", 1)[1]
            if variant not in VARIANTS:
                raise ControlError(ErrorKind.USAGE, f"unknown forged variant {variant}")
            return forge(variant, self._newest(run))
        if name == "inbox":
            return self._newest(run)
        if name not in self._tokens:
            raise ControlError(ErrorKind.USAGE, f"no token named {name}")
        return self._tokens[name]

    def _newest(self, run: str | None) -> Secret:
        if run is None or run not in self._run_token:
            raise ControlError(ErrorKind.USAGE, "no inbox token of that run")
        return self._run_token[run]

    # ---- hello and connect ---------------------------------------------------------------

    async def _op_hello(self, args: dict[str, Any]) -> dict[str, Any]:
        if self._client is not None:
            raise ControlError(ErrorKind.USAGE, "hello was sent already")
        creds = args["credentials"]
        key = creds.get("dpop_private_jwk")
        credentials = Credentials(Secret(str(creds["token"])), MemoryDpopKey(key) if key else None)
        self._settings = SdkSettings.from_mapping(
            {str(k): str(v) for k, v in (args.get("settings") or {}).items()}
        )
        self._hello = dict(args)
        self._hello.pop("credentials", None)
        self._credentials = credentials
        self._client = Client(
            str(args["base_url"]), credentials, settings=self._settings, clock=self.clock
        )
        self._verifier = InvocationVerifier(
            instance_id=UUID(str(args["instance_id"])),
            agent_id=UUID(str(args["agent_id"])),
            jwks=self._fetch_jwks,
            replay=self._replay,
            settings=self._settings,
            clock=self.clock,
        )
        return {}

    async def _fetch_jwks(self) -> tuple[Mapping[str, Any], float | None]:
        response = await self.client.http.get(
            "/instance/jwks", headers={"Accept": "application/json"}
        )
        if response.status_code != httpx.codes.OK:
            raise ProblemError(response.status_code, "jwks_unavailable")
        max_age = None
        for directive in (response.headers.get("Cache-Control") or "").split(","):
            name, _, value = directive.strip().partition("=")
            if name.lower() == "max-age":
                with contextlib.suppress(ValueError):
                    max_age = max(0.0, float(value.strip('"')))
        return response.json(), max_age

    @property
    def _consumer(self) -> str:
        return str(self._hello.get("consumer") or "default")

    @property
    def _gateway_mode(self) -> bool:
        return self._hello.get("transport") == "gateway"

    async def _op_connect(self, args: dict[str, Any]) -> dict[str, Any]:
        if self._gateway_mode:
            return await self._connect_gateway()
        answer = await self._poll()
        if answer.problem is not None:
            return answer.members()
        return {"head_seq": answer.body.get("head_seq", 0), **answer.members()}

    async def _connect_gateway(self) -> dict[str, Any]:
        url = gateway_url(self.client.base_url, str(self._hello["gateway_url"]))
        self._gateway = Gateway(
            url,
            self._credentials,
            consumer=self._consumer,
            agent_id=UUID(str(self._hello["agent_id"])),
            settings=self._settings,
            clock=self.clock,
            heartbeat=None,
        )
        self._frames = asyncio.Queue()
        self._reader = asyncio.create_task(self._read_frames())
        # The reader ends without ready when the gateway refuses identify or closes for good;
        # then the op answers with an error instead of waiting forever.
        ready = asyncio.create_task(self._ready.wait())
        done, _ = await asyncio.wait({ready, self._reader}, return_when=asyncio.FIRST_COMPLETED)
        if ready not in done:
            ready.cancel()
            raise ControlError(ErrorKind.TRANSPORT, "the gateway ended before ready")
        return {"head_seq": self._head_seq, "status": 200}

    async def _read_frames(self) -> None:
        if self._gateway is None or self._frames is None:
            raise ControlError(ErrorKind.USAGE, "connect must come first")
        async for frame in self._gateway.frames():
            if frame.op in ("ready", "resumed", "resync_required"):
                data = frame.data if isinstance(frame.data, dict) else {}
                self._head_seq = int(data.get("head_seq", data.get("cursor", 0)) or 0)
                self._window.reset()
                self._ready.set()
            elif frame.op == "dispatch":
                await self._frames.put(frame)
            elif frame.op == "error":
                data = frame.data if isinstance(frame.data, dict) else {}
                self._gateway_errors.append(dict(data))
                await self._frames.put(frame)

    _head_seq = 0

    # ---- receive -------------------------------------------------------------------------

    async def _poll(self) -> _Answer:
        path = f"/agents/me/consumers/{quote(self._consumer, safe='')}/events"
        return await self._call("GET", path)

    async def _ack(self, seq: int) -> None:
        if self._gateway is not None:
            await self._gateway.ack(seq)
            return
        path = f"/agents/me/consumers/{quote(self._consumer, safe='')}/ack"
        answer = await self._call("POST", path, {"seq": seq})
        if answer.problem is not None:
            _LOG.warning("ack failed: %s", answer.problem.code)

    async def _settle(self, entry: _Entry) -> None:
        if entry.settled:
            return
        entry.settled = True
        seq = entry.envelope.seq
        if seq is None:
            return
        if seq <= self._window.acked:
            # Delivered again although acknowledged before: the server lost the acknowledgement
            # (a restart before it was stored, 04 15.2 point 4), so acknowledge it again.
            await self._ack(seq)
            return
        point = self._window.settle(seq)
        if point is not None:
            await self._ack(point)

    def _add(self, envelope: Envelope) -> _Entry | None:
        """Note a delivered envelope; a redelivery of a known event replaces its token."""
        known = self._entries.get(envelope.id)
        if envelope.seq is not None:
            self._window.delivered(envelope.seq)
        if known is not None:
            known.envelope = envelope
            if envelope.token is not None:
                known.token = envelope.token
            if known.settled and envelope.seq is not None:
                # The server delivered it again (its acknowledgement was lost): settle again.
                known.settled = False
            known.handled = False
            known.held = False
            if known not in self._pending_list():
                self._order.append(envelope.id)
            return known
        entry = _Entry(envelope, envelope.token)
        self._entries[envelope.id] = entry
        self._order.append(envelope.id)
        return entry

    def _pending_list(self) -> list[_Entry]:
        return [self._entries[i] for i in self._order if not self._entries[i].handled]

    def _match(self, entry: _Entry, args: Mapping[str, Any]) -> bool:
        want = args.get("type")
        if want is not None and entry.envelope.type != want:
            return False
        run = args.get("run")
        return run is None or _run_of(entry) == str(run)

    async def _drain_untokened(self) -> None:
        """Settle every pending event without a token (it needs no acceptance)."""
        for entry in self._pending_list():
            if entry.envelope.type not in _TOKEN_TYPES and not entry.settled:
                await self._settle(entry)

    async def _op_receive(self, args: dict[str, Any]) -> dict[str, Any]:  # noqa: PLR0912 - one loop per transport
        min_head = int(args.get("head") or 0)
        while True:
            for entry in self._pending_list():
                if self._match(entry, args):
                    entry.handled = True
                    self._order.remove(entry.envelope.id)
                    return await self._take(entry, args)
            await self._drain_untokened()
            if self._gateway_mode:
                if self._frames is None:
                    raise ControlError(ErrorKind.USAGE, "connect must come first")
                frame = await self._frames.get()
                if frame.op == "error":
                    continue
                try:
                    envelope = envelope_from(frame.data)
                except (ValueError, TypeError):
                    continue
                self._add(envelope)
                continue
            answer = await self._poll()
            if answer.problem is not None:
                # A refused poll is no "nothing arrived": the problem members, without none.
                return answer.members()
            events = answer.body.get("events") or []
            for item in events:
                try:
                    self._add(envelope_from(item))
                except (ValueError, TypeError):
                    continue
            cursor = answer.body.get("next_cursor")
            if not events:
                if isinstance(cursor, int):
                    point = self._window.cursor(cursor)
                    if point is not None:
                        await self._ack(point)
                if int(answer.body.get("head_seq", 0)) >= min_head:
                    return {"none": True}

    def _actual(self) -> Actual:
        rt = self._hello.get("runtime") or {}
        return Actual(str(rt.get("billing_mode") or "unknown"), str(rt.get("tool_class") or "chat"))

    async def _verify(self, entry: _Entry) -> str:
        if self._verifier is None:
            raise ControlError(ErrorKind.USAGE, "hello must come first")
        if entry.token is None:
            return TokenVerdict.INVALID.value
        expect = expectation_for(entry.envelope.type, entry.envelope.event)
        check = await self._verifier.check(entry.token, expect, self._actual())
        return check.verdict.value

    async def _take(self, entry: _Entry, args: Mapping[str, Any]) -> dict[str, Any]:
        result = self._describe(entry)
        if entry.envelope.type == "command.autocomplete_requested":
            self._requested.append(result)
            if self._autocomplete["mode"] == "answer":
                await self._answer_autocomplete(entry)
        if entry.envelope.type not in _TOKEN_TYPES or entry.envelope.ephemeral:
            await self._settle(entry)
            if entry.token is not None:
                result["verdict"] = await self._verify(entry)
            return result
        if args.get("hold"):
            entry.held = True
            self._held = getattr(self, "_held", {})
            self._held[str(entry.envelope.id)] = entry
            return result
        result.update(await self._accept(entry, args))
        return result

    async def _accept(self, entry: _Entry, args: Mapping[str, Any]) -> dict[str, Any]:
        verdict = await self._verify(entry)
        out: dict[str, Any] = {"verdict": verdict}
        if entry.token is not None:
            out["jti_hash"] = _token_hash(entry.token)
            with contextlib.suppress(ValueError):
                # The payload of agent.followup and interaction.created names no trigger kind;
                # the token does (05 5).
                out.setdefault("trigger_kind", stored_claims(entry.token).trigger_kind)
        if verdict == TokenVerdict.UNKNOWN_KEY.value:
            # 09 4.5: neither accepted nor acknowledged; the event comes again.
            entry.handled = True
            return out
        if args.get("keep_token") and entry.token is not None:
            self._tokens[str(args["keep_token"])] = entry.token
        if not args.get("accept", True):
            return out
        run = _run_of(entry)
        if run is not None and entry.token is not None:
            self._run_token[run] = entry.token
        await self._settle(entry)
        reason = _REFUSAL_REASON.get(TokenVerdict(verdict))
        if reason is not None and run is not None and not args.get("skip_own_checks"):
            # 09 4.9: refuse the token, start nothing, end the run with the reason.
            answer = await self._call(
                "POST", f"/runs/{run}/finish", {"state": "failed", "reason": reason}
            )
            out["finished"] = answer.members()
        return out

    def _describe(self, entry: _Entry) -> dict[str, Any]:
        env = entry.envelope
        p = _payload(entry)
        out: dict[str, Any] = {
            "event": env.type,
            "id": str(env.id),
            "redelivered": env.redelivered,
        }
        if env.seq is not None:
            out["seq"] = env.seq
        run, thread = _run_of(entry), _thread_of(entry)
        if run is not None:
            out["run_id"] = run
        if thread is not None:
            out["thread_id"] = thread
        for name in ("message_id", "command_id", "request_id", "trigger_kind", "session_mode"):
            if p.get(name) is not None:
                out[name] = str(p[name])
        interaction = p.get("interaction")
        if isinstance(interaction, dict) and interaction.get("id") is not None:
            out["interaction_id"] = str(interaction["id"])
            if interaction.get("state") is not None:
                out["interaction_state"] = str(interaction["state"])
        return out

    async def _answer_autocomplete(self, entry: _Entry) -> None:
        request_id = _payload(entry).get("request_id")
        if request_id is None:
            return
        choices = self._autocomplete.get("choices") or []
        if self._gateway is not None:
            from bothy_sdk.generated.models import CommandChoice  # noqa: PLC0415

            await self._gateway.send_autocomplete_result(
                UUID(str(request_id)), [CommandChoice.from_dict(c) for c in choices]
            )
            return
        await self._call(
            "POST", f"/command-autocompletions/{request_id}/answer", {"choices": choices}
        )

    async def _op_verify(self, args: dict[str, Any]) -> dict[str, Any]:
        held = getattr(self, "_held", {})
        if "event" in args:
            entry = held.pop(str(args["event"]), None)
            if entry is None:
                raise ControlError(ErrorKind.USAGE, "no held event with that id")
            entry.held = False
            entry.handled = True
            return await self._accept(entry, args)
        if self._verifier is None:
            raise ControlError(ErrorKind.USAGE, "hello must come first")
        token = self._resolve(args.get("token"), args.get("run"))
        if token is None:
            return {"verdict": TokenVerdict.INVALID.value}
        # A token without its event: the payload points of the check are those of its claims.
        try:
            claims = stored_claims(token)
        except ValueError:
            return {"verdict": TokenVerdict.INVALID.value}
        from bothy_sdk.invocation import Expectation  # noqa: PLC0415

        trigger = {
            "message_id": claims.message_id,
            "interaction_id": claims.interaction_id,
            "command_id": claims.command_id,
        }
        claim = next((k for k, v in trigger.items() if v is not None), "message_id")
        expect = Expectation(claims.thread_id, claims.run_id, claim, trigger.get(claim))
        check = await self._verifier.check(token, expect, self._actual())
        return {"verdict": check.verdict.value, "jti_hash": _token_hash(token)}

    # ---- runs ----------------------------------------------------------------------------

    def _start_body(self, args: Mapping[str, Any], token: Secret | None) -> dict[str, Any]:
        body: dict[str, Any] = {}
        if token is not None:
            body["invocation_token"] = token.reveal()
        mode = (
            self._runtime("billing_mode", args)
            if "billing_mode_used" not in args
            else args["billing_mode_used"]
        )
        tool = (
            self._runtime("tool_class", args)
            if "tool_class_used" not in args
            else args["tool_class_used"]
        )
        body["billing_mode_used"] = mode if mode not in (None, "unknown") else "organization"
        body["tool_class_used"] = tool or "chat"
        evidence = args.get("billing_evidence", self._runtime("billing_evidence", {}))
        if evidence:
            body["billing_evidence"] = evidence
        model = args.get("model", self._runtime("model", {}))
        if model:
            body["model"] = model
        return body

    async def _start_or_resume(self, op: str, args: dict[str, Any]) -> dict[str, Any]:
        run = str(args["run"])
        token = self._resolve(args.get("token"), run)
        if not args.get("skip_own_checks") and token is not None and token.reveal():
            # The adapter's own check of mode and tool class at start time (09 4.5 points 8
            # and 10): a mismatch sends no request.
            try:
                claims = stored_claims(token)
            except ValueError:
                claims = None
            if claims is not None:
                from bothy_sdk.invocation import compare_actual  # noqa: PLC0415

                verdict = compare_actual(claims, self._actual())
                if (
                    verdict is not TokenVerdict.VALID
                    and "billing_mode_used" not in args
                    and "tool_class_used" not in args
                ):
                    return {"refused": verdict.value}
        answer = await self._call("POST", f"/runs/{run}/{op}", self._start_body(args, token))
        out = answer.members()
        if answer.problem is None and isinstance(answer.body, dict):
            out["run_state"] = answer.body.get("state")
            out["run_version"] = answer.body.get("version")
            out["replay"] = answer.headers.get("Idempotent-Replay") == "true"
            if token is not None:
                self._run_token[run] = token
                self._started_token[run] = token
            if run not in self._active:
                self._active.append(run)
            self._versions.setdefault(run, []).append(int(answer.body.get("version", 0)))
        return out

    async def _op_start(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._start_or_resume("start", args)

    async def _op_resume(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._start_or_resume("resume", args)

    async def _op_renew(self, args: dict[str, Any]) -> dict[str, Any]:
        run = str(args["run"])
        token = self._resolve(args.get("token"), run)
        body: dict[str, Any] = {}
        if token is not None:
            body["invocation_token"] = token.reveal()
        client = None
        if args.get("retry") is False:
            client = Client(
                self.client.base_url,
                self._credentials,
                settings=SdkSettings.from_mapping(
                    {
                        **{k: str(v) for k, v in (self._hello.get("settings") or {}).items()},
                        "sdk.retry_max_attempts": "1",
                    }
                ),
                clock=self.clock,
            )
            client.auth.remember_nonce(self.client.auth.nonce)
        try:
            answer = await self._call("POST", f"/runs/{run}/invocation-token", body, client=client)
        finally:
            if client is not None:
                await client.aclose()
        out = answer.members()
        if answer.problem is None and isinstance(answer.body, dict):
            new = Secret(str(answer.body["invocation_token"]))
            self._run_token[run] = new
            out["jti_hash"] = _token_hash(new)
            if args.get("keep_token"):
                self._tokens[str(args["keep_token"])] = new
        return out

    async def _op_context(self, args: dict[str, Any]) -> dict[str, Any]:
        run = str(args["run"])
        answer = await self._call("GET", f"/runs/{run}/context", run=run)
        out = answer.members()
        if answer.problem is not None:
            return out
        ctx = RunContext.from_dict(answer.body)
        rendered = render_context(ctx, file_texts=self._file_texts)
        prompt = rendered.system + "\n" + rendered.trigger
        elements = []
        messages = (
            list(ctx.channel_context or []) + list(ctx.thread_context) + [ctx.trigger_message]
        )
        for m in messages:
            trust = str(m.trust)
            element: dict[str, Any] = {
                "message_id": str(m.message_id),
                "role": str(m.role),
                "trust": trust,
                "truncated": not _unset(m.truncated) and bool(m.truncated),
                "bytes": len(m.content_md.encode()),
            }
            if trust not in ("trigger", "self"):
                element["marked_only"] = _only_marked(rendered.system, m.content_md)
            element["attachments"] = [
                self._placed(a, prompt, rendered.trigger) for a in m.attachments
            ]
            elements.append(element)
        out["elements"] = elements
        out["by_message"] = {e["message_id"]: e for e in elements}
        # The sum agent.context_max_bytes bounds: every content but the triggering message's.
        out["context_bytes"] = sum(e["bytes"] for e in elements[:-1])
        trigger = ctx.trigger_message
        out["trigger_complete"] = _unset(trigger.truncated) or not trigger.truncated
        if not _unset(ctx.truncated_before_seq):
            out["truncated_before_seq"] = ctx.truncated_before_seq
        return out

    def _placed(self, item: RunContextAttachment, prompt: str, trigger: str) -> dict[str, Any]:
        """Where the texts of one attachment stand in the composed prompt (ADR 0054 point 4).

        ``content_marked_only`` for a file read with ``read_file`` (its text is always data),
        ``transcript_marked_only`` and ``transcript_in_trigger`` for a transcript (it carries
        the mark of its message); the texts themselves never leave the process.
        """
        out: dict[str, Any] = {"file_id": str(item.file_id), "transcript": False}
        text = self._file_texts.get(item.file_id)
        if text:
            out["content_marked_only"] = _only_marked(prompt, text)
        if not _unset(item.transcript):
            out["transcript"] = True
            if item.transcript:
                said = str(item.transcript)
                out["transcript_marked_only"] = _only_marked(prompt, said)
                out["transcript_in_trigger"] = said in trigger
        return out

    async def _op_patch(self, args: dict[str, Any]) -> dict[str, Any]:
        run = str(args["run"])
        ops = list(args.get("ops") or [])
        if self._gateway is not None and not args.get("rest"):
            return await self._patch_gateway(run, ops, args)
        base = await self._current_version(run)
        stale = args.get("base_version") == "stale"
        sent = base - 1 if stale else base
        answer = await self._call(
            "PATCH", f"/runs/{run}", {"base_version": sent, "ops": ops}, run=run
        )
        out = answer.members()
        if answer.problem is None:
            out["current_version"] = (
                answer.body.get("version") if isinstance(answer.body, dict) else None
            )
            self._versions.setdefault(run, []).append(int(out["current_version"] or 0))
            return out
        current = answer.problem.current
        if current is not None:
            out["current_version"] = current.get("version")
        if answer.problem.code == "base_version_conflict" and args.get("rebase", True):
            # The SDK's rebase: the patcher sends again on the version of `current`.
            patcher = RunPatcher(
                self.client, UUID(run), sent, settings=self._settings, clock=self.clock
            )
            try:
                with self.client.bind_run(UUID(run), self._run_token[run]):
                    version = await patcher.send_ops([JsonPatchOperation.from_dict(o) for o in ops])
                out["rebased"] = {"status": 200, "current_version": version}
                self._versions.setdefault(run, []).append(version)
            except ProblemError as exc:
                out["rebased"] = _Answer(exc.status, None, exc).members()
            finally:
                await patcher.aclose()
        return out

    async def _current_version(self, run: str) -> int:
        answer = await self._call("GET", f"/runs/{run}", run=run)
        if answer.problem is not None or not isinstance(answer.body, dict):
            known = self._versions.get(run)
            return known[-1] if known else 1
        return int(answer.body.get("version", 1))

    async def _patch_gateway(
        self, run: str, ops: list[Any], args: Mapping[str, Any]
    ) -> dict[str, Any]:
        """Patch with the gateway op ``run.update``.

        A ``base_version_conflict`` frame is followed by the SDK's rebase (load ``getRun``, send
        again on its version) unless ``rebase: false``.
        """
        base = await self._current_version(run)
        sent = base - 1 if args.get("base_version") == "stale" else base
        out = await self._gateway_update(run, sent, ops, base)
        if out.get("code") == "base_version_conflict":
            current = await self._current_version(run)
            out["current_version"] = current
            if args.get("rebase", True):
                out["rebased"] = await self._gateway_update(run, current, ops, current)
        return out

    async def _gateway_update(
        self, run: str, sent: int, ops: list[Any], before: int
    ) -> dict[str, Any]:
        """Send one ``run.update`` and wait for its outcome.

        The outcome is an ``error`` frame with its ``ref``, or a stored version above ``before``
        (the gateway has no success reply). The conductor bounds the wait.
        """
        if self._gateway is None:
            raise ControlError(ErrorKind.USAGE, "connect must come first")
        ref = f"{run}:{_uuid7()}"
        await self._gateway.send_run_update(
            UUID(run), sent, [JsonPatchOperation.from_dict(o) for o in ops], ref=ref
        )
        while True:
            for err in self._gateway_errors:
                if err.get("ref") == ref:
                    code = str(err.get("code"))
                    return {"status": PROBLEM_STATUS.get(code, 0), "code": code}
            version = await self._current_version(run)
            if version > before:
                return {"status": 200, "current_version": version}
            await asyncio.sleep(0.02)

    async def _op_await_input(self, args: dict[str, Any]) -> dict[str, Any]:
        run = str(args["run"])
        body = {"status_line": str(args.get("status_line") or "Waiting for input")}
        return (await self._call("POST", f"/runs/{run}/await-input", body, run=run)).members()

    async def _op_finish(self, args: dict[str, Any]) -> dict[str, Any]:
        run = str(args["run"])
        body: dict[str, Any] = {"state": str(args.get("state") or "completed")}
        # ``costs`` is the list of cost reports of 09 5.4 (the demo agent of M7-27 reports them).
        for name in ("reason", "summary", "costs"):
            if args.get(name):
                body[name] = args[name]
        answer = await self._call("POST", f"/runs/{run}/finish", body, run=run)
        if answer.problem is None and run in self._active:
            self._active.remove(run)
        return answer.members()

    async def _op_heartbeat(self, args: dict[str, Any]) -> dict[str, Any]:
        capabilities = args.get("capabilities", self._runtime("capabilities", {}))
        details: dict[str, Any] = {"active_runs": list(args.get("active_runs", self._active))}
        if capabilities:
            details["capabilities"] = capabilities
        mode = args.get("billing_mode", self._runtime("billing_mode", {}))
        evidence = args.get("billing_evidence", self._runtime("billing_evidence", {}))
        tool = args.get("tool_class", self._runtime("tool_class", {}))
        beat = build_heartbeat(
            reported_billing_mode=mode if mode not in (None, "unknown") else None,
            billing_evidence=evidence or None,
            reported_tool_class=tool or None,
            details=details,
        )
        return (await self._call("POST", "/agents/me/heartbeat", beat.to_dict())).members()

    # ---- messages, cards and the rest ----------------------------------------------------

    async def _op_post(self, args: dict[str, Any]) -> dict[str, Any]:
        run = args.get("run")
        body: dict[str, Any] = {"content_md": str(args.get("content") or "")}
        body["client_msg_id"] = str(_uuid7())
        if run is not None:
            body["run_id"] = str(run)
        card = args.get("card")
        if card is not None:
            body["kind"] = "card"
            if card == "approval":
                components = approval_card(settings=self._settings)
            else:
                components = [
                    button(
                        str(b["custom_id"]),
                        str(b["label"]),
                        str(b.get("value", b["custom_id"])),
                        single_use=args.get("single_use"),
                        restricted_to=args.get("restricted_to"),
                        settings=self._settings,
                    )
                    for b in args.get("buttons") or []
                ]
            body["components"] = [c.to_dict() for c in components]
        if args.get("new_thread"):
            thread_body: dict[str, Any] = {"first_message": body}
            if args.get("title"):
                thread_body["title"] = args["title"]
            if args.get("parent_message_id"):
                thread_body["parent_message_id"] = args["parent_message_id"]
            answer = await self._call(
                "POST", f"/channels/{args['channel']}/threads", thread_body, run=run
            )
        else:
            answer = await self._call("POST", f"/threads/{args['thread']}/messages", body, run=run)
        out = answer.members()
        if answer.problem is None and isinstance(answer.body, dict):
            message = (
                answer.body.get("message", answer.body) if args.get("new_thread") else answer.body
            )
            if args.get("new_thread"):
                out["thread_id"] = answer.body.get("thread", {}).get("id")
                message = answer.body.get("first_message", message)
            out["message_id"] = message.get("id")
            out["component_ids"] = {
                str(c.get("custom_id")): c.get("id") for c in message.get("components") or []
            }
        return out

    async def _op_respond(self, args: dict[str, Any]) -> dict[str, Any]:
        """``respondToInteraction`` with ``state`` (default ``acknowledged``) and ``text``."""
        body: dict[str, Any] = {"state": str(args.get("state") or "acknowledged")}
        if args.get("text"):
            body["response"] = {"text": str(args["text"])}
        path = f"/interactions/{args['interaction']}/respond"
        return (await self._call("POST", path, body, run=args.get("run"))).members()

    async def _op_update_component(self, args: dict[str, Any]) -> dict[str, Any]:
        """``updateComponent`` with ``If-Match`` of the given ``version``."""
        reserved = ("component", "run", "version", "bind")
        body = {k: v for k, v in args.items() if k not in reserved}
        headers = {"If-Match": f'"{int(args["version"])}"'} if "version" in args else {}
        path = f"/components/{args['component']}"
        answer = await self._call("PATCH", path, body, run=args.get("run"), headers=headers)
        return answer.members()

    async def _op_set_title(self, args: dict[str, Any]) -> dict[str, Any]:
        thread = str(args["thread"])
        current = await self._call("GET", f"/threads/{thread}")
        if current.problem is not None:
            return current.members()
        etag = current.headers.get("ETag", "")
        answer = await self._call(
            "PUT",
            f"/threads/{thread}/title",
            {"title": str(args["title"])},
            headers={"If-Match": etag},
            run=args.get("run"),
        )
        out = answer.members()
        if answer.problem is None and isinstance(answer.body, dict):
            out["title_source"] = answer.body.get("title_source")
        return out

    async def _op_report_session(self, args: dict[str, Any]) -> dict[str, Any]:
        thread = str(args["thread"])
        body = {k: v for k, v in args.items() if k not in ("thread", "run", "if_match", "bind")}
        headers = {}
        if args.get("if_match") is not None:
            headers["If-Match"] = str(args["if_match"])
        else:
            # The compare-and-swap over agent_sessions.version of the thread's current session;
            # without one the report creates the row and sends no If-Match (06, reportAgentSession).
            listed = await self._call("GET", f"/threads/{thread}/agent-sessions")
            items = (listed.body or {}).get("items", []) if listed.problem is None else []
            current = next((i for i in items if i.get("is_current")), None)
            if current is not None:
                headers["If-Match"] = f'"{int(current["version"])}"'
        path = f"/threads/{thread}/agent-sessions/report"
        return (
            await self._call("POST", path, body, headers=headers, run=args.get("run"))
        ).members()

    async def _op_register_commands(self, args: dict[str, Any]) -> dict[str, Any]:
        registered = []
        first_refusal: dict[str, Any] | None = None
        for decl in args.get("commands") or []:
            spec = CommandSpec(
                name=str(decl["name"]),
                description=str(decl.get("description") or decl["name"]),
                options=tuple(CommandOption.from_dict(o) for o in decl.get("options") or []),
                scope_kind=decl.get("scope_kind"),
                scope_id=UUID(str(decl["scope_id"])) if decl.get("scope_id") else None,
            )
            answer = await self._call(
                "POST", "/commands", spec.to_register(self._settings).to_dict()
            )
            if answer.problem is None:
                registered.append({"name": spec.name, "id": (answer.body or {}).get("id")})
            elif first_refusal is None:
                first_refusal = answer.members()
        out = dict(first_refusal or {"status": 201})
        out["registered"] = registered
        if registered:
            out["command_id"] = registered[0]["id"]
        return out

    async def _op_autocomplete_mode(self, args: dict[str, Any]) -> dict[str, Any]:
        mode = str(args.get("mode") or "answer")
        if mode not in ("answer", "ignore"):
            raise ControlError(ErrorKind.USAGE, "mode is answer or ignore")
        self._autocomplete = {"mode": mode, "choices": list(args.get("choices") or [])}
        return {}

    async def _op_read_file(self, args: dict[str, Any]) -> dict[str, Any]:
        """``downloadFile`` with the agent's credentials, bound to ``run`` when given.

        The reply holds the status, the problem members and ``bytes`` (the length); the content
        stays in the process. A UTF-8 text without NUL is kept for the data blocks of `context`.
        """
        file_id = UUID(str(args["file"]))
        run = args.get("run")
        answer = await self._call(
            "GET", f"/files/{file_id}/content", run=str(run) if run is not None else None
        )
        out = answer.members()
        if answer.problem is None:
            out["bytes"] = len(answer.content)
            with contextlib.suppress(UnicodeDecodeError):
                text = answer.content.decode("utf-8")
                if "\x00" not in text:
                    self._file_texts[file_id] = text
        return out

    async def _op_submit_transcript(self, args: dict[str, Any]) -> dict[str, Any]:
        """``submitTranscript`` with ``text`` and ``language``.

        ``If-Match`` is the ``ETag`` of ``getFile`` unless ``if_match`` names a value to send
        instead (``absent`` sends none), for the ``412`` and ``428`` answers.
        """
        file_id = UUID(str(args["file"]))
        run = str(args["run"]) if args.get("run") is not None else None
        if_match = args.get("if_match")
        if if_match is None:
            current = await self._call("GET", f"/files/{file_id}", run=run)
            if current.problem is not None:
                return current.members()
            if_match = current.headers.get("ETag", "")
        headers = {} if if_match == "absent" else {"If-Match": str(if_match)}
        body: dict[str, Any] = {"text": str(args.get("text") or "")}
        if args.get("language"):
            body["language"] = str(args["language"])
        path = f"/files/{file_id}/transcript"
        return (await self._call("PUT", path, body, run=run, headers=headers)).members()

    async def _op_export_token(self, args: dict[str, Any]) -> dict[str, Any]:
        token = self._resolve(args.get("token"), args.get("run"))
        if token is None:
            raise ControlError(ErrorKind.USAGE, "no token")
        return {"jti_hash": _token_hash(token), "token": token.reveal()}

    async def _op_import_token(self, args: dict[str, Any]) -> dict[str, Any]:
        token = Secret(str(args["token"]))
        self._tokens[str(args["name"])] = token
        if args.get("run"):
            self._run_token[str(args["run"])] = token
        return {"jti_hash": _token_hash(token)}

    async def _op_crash(self, args: dict[str, Any]) -> dict[str, Any]:
        # Exit at once: no reply, no finishRun, no flush of anything (AG-10).
        os._exit(_CRASH_STATUS)


class _EmptyToken:
    """The empty token of AG-08 "missing": `Secret` refuses an empty text, the request sends one."""

    def reveal(self) -> str:
        """Return the empty text."""
        return ""


def _unset(value: Any) -> bool:
    from bothy_sdk.generated.types import Unset  # noqa: PLC0415

    return isinstance(value, Unset)


def _uuid7() -> UUID:
    from bothy_sdk.ids import uuid7  # noqa: PLC0415

    return uuid7()


def _only_marked(system: str, text: str) -> bool:
    """Whether every occurrence of ``text`` in ``system`` lies inside a marked data block."""
    if not text:
        return True
    quoted = json.dumps(text, ensure_ascii=False)[1:-1]
    found = False
    for needle in {text, quoted}:
        start = 0
        while True:
            at = system.find(needle, start)
            if at < 0:
                break
            found = True
            before = system[:at]
            opened = before.rfind("<<<DATA ")
            closed = before.rfind("<<<END DATA ")
            if opened < 0 or closed > opened:
                return False
            start = at + 1
    return found


async def _serve(reader: asyncio.StreamReader, write: Callable[[str], None]) -> None:
    agent = FakeAgent()
    try:
        while True:
            line = await reader.readline()
            if not line:
                return
            text = line.decode("utf-8", errors="replace").strip()
            if not text:
                continue
            try:
                request = parse_request(text)
            except ControlError as exc:
                write(encode_reply(Reply.failure(0, exc.kind, exc.detail)))
                continue
            write(encode_reply(await agent.handle(request)))
    finally:
        await agent.close()


def main() -> None:
    """Run the Fake-Agent on stdin and stdout until stdin closes."""
    logging.basicConfig(stream=sys.stderr, level=logging.WARNING)

    def write(line: str) -> None:
        sys.stdout.write(line + "\n")
        sys.stdout.flush()

    async def run() -> None:
        loop = asyncio.get_running_loop()
        reader = asyncio.StreamReader()
        await loop.connect_read_pipe(lambda: asyncio.StreamReaderProtocol(reader), sys.stdin)
        await _serve(reader, write)

    asyncio.run(run())


if __name__ == "__main__":
    main()
