# SPDX-License-Identifier: Apache-2.0
"""Scenario functions that run unchanged against the FakeServer and the real server.

`ScenarioServer` names only what the scenarios need from a server; ``FakeServer`` and the
``RealServer`` of ``tests/realserver`` implement it (ADR 0026 "Confirmation": the same scenarios
against both). Every scenario connects with its own consumer names, so a run one scenario leaves
``queued`` never reaches another: a new consumer starts at ``head_seq`` (04 15.1 point 2).

Scenarios (docs/spec/09-agents.md 16.2, 07 AZ-P6): `ag08_invalid_tokens`, `ag09_double_delivery`,
`ag12_patch_rules`, `ag28_slow_start_and_restart`, `azp6_adapter_never_starts`. A failed check
raises `ScenarioFailed`; its message never contains a token.
"""

from __future__ import annotations

import asyncio
import contextlib
import inspect
import json
import logging
import time
from collections.abc import AsyncIterator, Awaitable, Callable, Mapping
from dataclasses import dataclass
from typing import Any, Protocol, cast
from uuid import UUID

import httpx

from bothy_sdk.adapter import ActualBilling, Adapter, AdapterConfig
from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import Client
from bothy_sdk.clock import Clock
from bothy_sdk.errors import (
    BaseVersionConflict,
    GatewayOpError,
    InvocationTokenInvalid,
    ProblemError,
    ValidationFailed,
)
from bothy_sdk.gateway import Gateway, gateway_url
from bothy_sdk.generated.api.agent_runtime import (
    get_consumer_state,
    patch_run,
    renew_invocation_token,
    start_run,
)
from bothy_sdk.generated.api.runs import get_run
from bothy_sdk.generated.models import (
    BillingMode,
    ConsumerState,
    Heartbeat,
    InvocationTokenRenew,
    JsonPatchOperation,
    JsonPatchOperationOp,
    Run,
    RunPatch,
    RunStart,
    ToolClass,
)
from bothy_sdk.generated.types import UNSET
from bothy_sdk.inbox import EntryState
from bothy_sdk.settings import SdkSettings
from bothy_sdk.sources import Envelope, EventSource, SourceReady
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.doubles import (
    KEEP,
    REMOVE,
    RecordingHandler,
    RecordingSource,
    TamperingSource,
)

__all__ = [
    "FORGED_VARIANTS",
    "RunView",
    "ScenarioFailed",
    "ScenarioServer",
    "Triggered",
    "ag08_invalid_tokens",
    "ag09_double_delivery",
    "ag12_patch_rules",
    "ag28_slow_start_and_restart",
    "azp6_adapter_never_starts",
]

_LOG = logging.getLogger("bothy_sdk")

FORGED_VARIANTS = ("bad_signature", "alg_hs256", "aud_other_agent", "typ_access")
"""Forged token variants both servers build (09 16.2 AG-08)."""

_MISSING = "missing"


class ScenarioFailed(AssertionError):  # noqa: N818 - the name of the brief
    """A scenario check failed; the message names the check, never a token."""


@dataclass(frozen=True)
class Triggered:
    """A run a scenario triggered: Alex mentions Lotse in a new thread."""

    run_id: UUID
    thread_id: UUID
    message_id: UUID


@dataclass(frozen=True)
class RunView:
    """The server's view of a run for the checks; ``starts`` counts transitions to running."""

    state: str
    state_reason: str | None
    version: int
    starts: int


class ScenarioServer(Protocol):
    """What the scenarios need from a server (FakeServer or the real server).

    Attributes:
        origin: The origin, for example ``http://127.0.0.1:<port>``.
        base_url: The API base.
        instance_id: The instance.
        agent_id: Lotse, the agent under test.
        other_agent_id: Texter, another agent.
        credentials: Lotse's API token and DPoP key.
        clock: The clock the adapter under test must use.
        timings: The server keys the server plays (``agent.ack_timeout``, ...), in seconds.
        sdk_settings: The SDK settings the adapter under test uses.
    """

    origin: str
    base_url: str
    instance_id: UUID
    agent_id: UUID
    other_agent_id: UUID
    credentials: Credentials
    clock: Clock
    timings: Mapping[str, float]
    sdk_settings: SdkSettings

    async def trigger(self, text: str = ...) -> Any:
        """Let Alex mention Lotse in a new thread; return run, thread and message IDs."""
        ...

    async def forge_token(self, run_id: UUID, variant: str) -> Secret:
        """Return a forged token of ``variant`` for ``run_id``."""
        ...

    async def run_state(self, run_id: UUID) -> Any:
        """Return state, reason, version and the number of starts of a run."""
        ...

    async def restart(self, consumer: str | None = None, acked_seq: int | None = None) -> None:
        """Restart the server on the same data, optionally rewinding a consumer cursor."""
        ...

    async def pass_time(self, seconds: float) -> None:
        """Let ``seconds`` pass (fake or real)."""
        ...


async def _aw(value: Any) -> Any:
    """Await ``value`` when it is awaitable (the FakeServer answers some helpers at once)."""
    return await value if inspect.isawaitable(value) else value


def _check(condition: bool, message: str) -> None:
    if not condition:
        raise ScenarioFailed(message)


async def _until(
    server: ScenarioServer,
    predicate: Callable[[], Any],
    what: str,
    *,
    limit: float = 60.0,
    step: float = 0.05,
) -> None:
    """Wait until ``predicate`` holds, letting time pass on the server's clock.

    It fails only after ``limit`` seconds of server time and `PATIENCE` of real time both passed,
    so a loaded machine that needs more loop turns per server second never fails a correct run.
    """
    waited = 0.0
    deadline = time.monotonic() + PATIENCE
    while True:
        result = predicate()
        if inspect.isawaitable(result):
            result = await result
        if result:
            return
        if waited >= limit and time.monotonic() > deadline:
            raise ScenarioFailed(f"timed out waiting for {what}")
        await server.pass_time(step)
        await asyncio.sleep(0.01)
        waited += step


def _adapter(
    server: ScenarioServer,
    handler: RecordingHandler,
    consumer: str,
    *,
    http_transport: httpx.AsyncBaseTransport | None = None,
) -> Adapter:
    config = AdapterConfig(
        base_url=server.base_url,
        instance_id=server.instance_id,
        agent_id=server.agent_id,
        credentials=server.credentials,
        consumer=consumer,
        adapter_name="scenario-adapter",
        adapter_version="1.0.0",
        runtime_name="scenario-runtime",
        settings=server.sdk_settings,
    )
    handler.clock = server.clock
    return _ScenarioAdapter(config, handler, clock=server.clock, http_transport=http_transport)


class _ScenarioAdapter(Adapter):
    """An adapter whose heartbeat leaves the tool class out.

    The server keeps the reported tool class of the last heartbeat (09 4.8); a reported ``host``
    makes it refuse the triggers of a person who is not the owner (SR2, ``tool_class_owner_only``)
    before the adapter sees a token. The scenarios test the adapter's own check of point 10, so
    their heartbeats must not race the trigger with the class.
    """

    def heartbeat(self) -> Heartbeat:
        """Return the heartbeat without ``reported_tool_class``."""
        beat = super().heartbeat()
        beat.reported_tool_class = UNSET
        return beat


class _Counting(httpx.AsyncBaseTransport):
    """Counts requests by method and path suffix (renewals and starts of AG-28 and AZ-P6)."""

    def __init__(self) -> None:
        self.inner = httpx.AsyncHTTPTransport()
        self.seen: list[tuple[str, str, dict[str, Any] | None]] = []

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        body = None
        if request.content:
            with contextlib.suppress(ValueError):
                body = json.loads(request.content)
        self.seen.append((request.method, request.url.path, body))
        return await self.inner.handle_async_request(request)

    def count(self, method: str, suffix: str) -> int:
        return sum(1 for m, p, _ in self.seen if m == method and p.endswith(suffix))

    def bodies(self, method: str, suffix: str) -> list[dict[str, Any]]:
        return [b for m, p, b in self.seen if m == method and p.endswith(suffix) and b]

    async def aclose(self) -> None:
        await self.inner.aclose()


@contextlib.asynccontextmanager
async def _running(
    adapter: Adapter, wrap: Callable[[RecordingSource], Any] | None = None
) -> AsyncIterator[Any]:
    """Run an adapter in a task with an optional wrapped source; stop it at the end."""
    source = await adapter.make_source()
    recording = RecordingSource(source)
    adapter.source = cast(EventSource, wrap(recording) if wrap is not None else recording)
    task = asyncio.ensure_future(adapter.run())
    try:
        yield recording, task
    finally:
        await adapter.stop()
        with contextlib.suppress(asyncio.CancelledError, Exception):
            await asyncio.wait_for(task, PATIENCE)


def _ready(recording: RecordingSource) -> Callable[[], bool]:
    return lambda: any(isinstance(d, SourceReady) for d in recording.deliveries)


def _envelope_for(recording: RecordingSource, run_id: UUID, event_type: str) -> list[Envelope]:
    out = []
    for env in recording.envelopes(event_type):
        payload = getattr(env.event, "payload", None)
        if getattr(payload, "run_id", None) == run_id:
            out.append(env)
    return out


async def _consumer(client: Client, consumer: str) -> ConsumerState:
    return await client.call_as(
        ConsumerState, get_consumer_state.asyncio_detailed, consumer=consumer
    )


def _client(server: ScenarioServer) -> Client:
    return Client(
        server.base_url, server.credentials, settings=server.sdk_settings, clock=server.clock
    )


async def _expect_token_invalid(call: Callable[[], Awaitable[Any]], reason: str, what: str) -> None:
    try:
        await call()
    except InvocationTokenInvalid as exc:
        _check(exc.reason == reason, f"{what}: reason {exc.reason}, expected {reason}")
        return
    except ProblemError as exc:
        raise ScenarioFailed(f"{what}: {exc.status} {exc.code}, expected {reason}") from None
    raise ScenarioFailed(f"{what}: accepted, expected invocation_token_invalid ({reason})")


# ---- AG-08 ------------------------------------------------------------------------------------


async def ag08_invalid_tokens(server: ScenarioServer) -> None:
    """AG-08 adapter side: no start on a missing or forged token; the run fails and is acked.

    Raises:
        ScenarioFailed: A check failed.
    """
    forged: dict[UUID, asyncio.Future[Secret | Any]] = {}

    def slot(run_id: UUID) -> asyncio.Future[Secret | Any]:
        return forged.setdefault(run_id, asyncio.get_running_loop().create_future())

    async def rewrite(env: Envelope) -> Secret | Any:
        if env.type != "agent.invocation":
            return KEEP
        run_id = getattr(getattr(env.event, "payload", None), "run_id", None)
        if not isinstance(run_id, UUID):
            return KEEP
        return await slot(run_id)

    handler = RecordingHandler()
    adapter = _adapter(server, handler, "ag08-adapter")
    client = _client(server)
    try:
        async with _running(adapter, lambda inner: TamperingSource(inner, rewrite)) as (rec, _):
            await _until(server, _ready(rec), "the adapter to connect")
            for variant in (_MISSING, *FORGED_VARIANTS):
                triggered = await _aw(server.trigger(f"AG-08 {variant} @Lotse"))
                token = (
                    REMOVE
                    if variant == _MISSING
                    else await _aw(server.forge_token(triggered.run_id, variant))
                )
                slot(triggered.run_id).set_result(token)
                run_id = triggered.run_id

                async def failed(rid: UUID = run_id) -> bool:
                    view = await _aw(server.run_state(rid))
                    return bool(view.state == "failed")

                await _until(server, failed, f"run of {variant} to fail")
                view = await _aw(server.run_state(run_id))
                _check(
                    view.state_reason == "invalid_invocation_token",
                    f"{variant}: reason {view.state_reason}",
                )
                _check(view.starts == 0, f"{variant}: the run started")
                _check(handler.count("prepare", run_id) == 0, f"{variant}: prepare was called")
                _check(handler.count("execute", run_id) == 0, f"{variant}: execute was called")
                envs = _envelope_for(rec, run_id, "agent.invocation")
                _check(bool(envs), f"{variant}: no delivery recorded")
                seq = envs[0].seq or 0

                async def acked(s: int = seq) -> bool:
                    return (await _consumer(client, "ag08-adapter")).acked_seq >= s

                await _until(server, acked, f"the acknowledgement of {variant}")
        # Direct calls with the same tokens for a run no adapter of this scenario receives.
        queued = await _aw(server.trigger("AG-08 direct @Lotse"))
        for variant in (_MISSING, *FORGED_VARIANTS):
            text = (
                ""
                if variant == _MISSING
                else (await _aw(server.forge_token(queued.run_id, variant))).reveal()
            )
            body = RunStart(
                invocation_token=text,
                billing_mode_used=BillingMode.ORGANIZATION,
                tool_class_used=ToolClass.CHAT,
            )

            async def start(b: RunStart = body) -> object:
                return await client.call(start_run.asyncio_detailed, run_id=queued.run_id, body=b)

            async def renew(t: str = text) -> object:
                return await client.call(
                    renew_invocation_token.asyncio_detailed,
                    run_id=queued.run_id,
                    body=InvocationTokenRenew(invocation_token=t),
                )

            await _expect_token_invalid(start, "invalid", f"startRun with {variant}")
            await _expect_token_invalid(renew, "invalid", f"renewInvocationToken with {variant}")
        view = await _aw(server.run_state(queued.run_id))
        _check(view.state == "queued", f"direct calls changed the run to {view.state}")
    finally:
        await client.aclose()


# ---- AZ-P6 ------------------------------------------------------------------------------------


async def _refused_with(
    server: ScenarioServer, handler: RecordingHandler, consumer: str, reason: str
) -> None:
    adapter = _adapter(server, handler, consumer)
    async with _running(adapter) as (rec, _):
        await _until(server, _ready(rec), "the adapter to connect")
        triggered = await _aw(server.trigger(f"AZ-P6 {consumer} @Lotse"))

        async def failed() -> bool:
            return bool((await _aw(server.run_state(triggered.run_id))).state == "failed")

        await _until(server, failed, f"the refusal {reason}")
        view = await _aw(server.run_state(triggered.run_id))
        _check(view.state_reason == reason, f"{consumer}: reason {view.state_reason}")
        _check(view.starts == 0, f"{consumer}: the run started")
        _check(handler.count("execute") == 0, f"{consumer}: execute was called")


async def azp6_adapter_never_starts(server: ScenarioServer) -> None:
    """AZ-P6: the runtime never starts on an invalid, mismatching, replayed or expired token.

    The forged and missing tokens are AG-08 (`ag08_invalid_tokens`); here: a billing mode
    different from the claim and ``unknown``, a tool class above the claim, a replayed ``jti``
    under a new event ID (only a server with ``deliver_with_token`` can send that), and an
    expired token that is renewed before the start.

    Raises:
        ScenarioFailed: A check failed.
    """
    await _refused_with(
        server, RecordingHandler(billing="unknown"), "azp6-unknown", "billing_mode_mismatch"
    )
    await _refused_with(server, _OtherMode(), "azp6-mode", "billing_mode_mismatch")
    await _refused_with(
        server, RecordingHandler(tool_class=ToolClass.HOST), "azp6-host", "tool_class_mismatch"
    )
    await _expired_then_renewed(server, "azp6-expired")
    if hasattr(server, "deliver_with_token"):
        await _replayed_jti(server)


class _OtherMode(RecordingHandler):
    """Reports the billing mode that is not the claim's."""

    def actual_billing(self, inv: Any) -> ActualBilling:
        """Return the mode that is not the claim's."""
        other = "personal" if inv.claims.billing_mode == "organization" else "organization"
        return ActualBilling(other)


async def _expired_then_renewed(server: ScenarioServer, consumer: str) -> None:
    release = asyncio.Event()
    handler = RecordingHandler(block_prepare=release)
    counting = _Counting()
    adapter = _adapter(server, handler, consumer, http_transport=counting)
    async with _running(adapter) as (rec, _):
        await _until(server, _ready(rec), "the adapter to connect")
        triggered = await _aw(server.trigger(f"AZ-P6 expired {consumer} @Lotse"))
        await _until(server, lambda: handler.count("prepare") == 1, "prepare")
        first = handler.invocations[0].claims
        expire = (
            server.timings["agent.invocation_token_ttl"]
            + server.sdk_settings.invocation_token_leeway
            + 1.0
        )
        await server.pass_time(expire)
        release.set()
        await _until(server, lambda: handler.count("execute") == 1, "execute after renewal")
        handle = handler.handles[triggered.run_id]
        _check(counting.count("POST", "/invocation-token") == 1, "not exactly one renewal")
        _check(counting.count("POST", "/start") == 1, "not exactly one start")
        _check(handle.token_jti not in (None, first.jti), "the run started with the expired token")
        view = await _aw(server.run_state(triggered.run_id))
        _check(view.starts == 1, f"starts {view.starts}")


async def _replayed_jti(server: ScenarioServer) -> None:
    handler = RecordingHandler()
    adapter = _adapter(server, handler, "azp6-replay")
    client = _client(server)
    try:
        async with _running(adapter) as (rec, _):
            await _until(server, _ready(rec), "the adapter to connect")
            triggered = await _aw(server.trigger("AZ-P6 replay @Lotse"))
            await _until(server, lambda: handler.count("execute") == 1, "the first run")
            first = _envelope_for(rec, triggered.run_id, "agent.invocation")[0]
            _check(first.token is not None, "the delivery had no token")
            if first.token is None:
                raise ScenarioFailed("the delivery had no token")
            replay_id = await _aw(server.deliver_with_token(first.id, first.token))  # type: ignore[attr-defined]
            await _until(
                server,
                lambda: any(e.id == replay_id for e in rec.envelopes()),
                "the replayed delivery",
            )
            replay = next(e for e in rec.envelopes() if e.id == replay_id)
            await server.pass_time(1.0)
            state = await _consumer(client, "azp6-replay")
            _check(state.acked_seq < (replay.seq or 0), "the replayed event was acknowledged")
            _check(handler.count("prepare") == 1, "the replayed token reached prepare")
            _check(handler.count("execute") == 1, "the replayed token reached execute")
    finally:
        await client.aclose()


# ---- AG-09 ------------------------------------------------------------------------------------


async def ag09_double_delivery(server: ScenarioServer) -> None:
    """AG-09: two adapter processes of one agent; exactly one start, the other closes its entry.

    Variant: a second adapter with the same consumer makes the first stop with ``4015``.

    Raises:
        ScenarioFailed: A check failed.
    """
    alpha, beta = RecordingHandler(), RecordingHandler()
    a = _adapter(server, alpha, "ag09-alpha")
    b = _adapter(server, beta, "ag09-beta")
    async with _running(a) as (rec_a, _), _running(b) as (rec_b, _):
        await _until(server, _ready(rec_a), "alpha to connect")
        await _until(server, _ready(rec_b), "beta to connect")
        triggered = await _aw(server.trigger("AG-09 @Lotse"))
        await _until(
            server,
            lambda: (
                bool(_envelope_for(rec_a, triggered.run_id, "agent.invocation"))
                and bool(_envelope_for(rec_b, triggered.run_id, "agent.invocation"))
            ),
            "both deliveries",
        )
        ta = _envelope_for(rec_a, triggered.run_id, "agent.invocation")[0].token
        tb = _envelope_for(rec_b, triggered.run_id, "agent.invocation")[0].token
        _check(ta is not None and tb is not None and ta != tb, "the tokens are not different")

        async def settled() -> bool:
            if alpha.count("execute") + beta.count("execute") != 1:
                return False
            for adapter in (a, b):
                entries = await adapter.inbox.for_run(triggered.run_id)
                if any(e.state is EntryState.ACCEPTED for e in entries):
                    return False
            return True

        await _until(server, settled, "one execute and no pending entry")
        view = await _aw(server.run_state(triggered.run_id))
        _check(view.starts == 1, f"starts {view.starts}")
    # Variant: the same consumer twice.
    first = _adapter(server, RecordingHandler(), "ag09-same")
    second = _adapter(server, RecordingHandler(), "ag09-same")
    async with _running(first) as (rec1, task1):
        await _until(server, _ready(rec1), "the first adapter to connect")
        async with _running(second) as (rec2, _):
            await _until(server, _ready(rec2), "the second adapter to connect")
            await _until(server, task1.done, "the first adapter to stop")
            reason = task1.result()
            # `run()` of the first adapter returned with 4015: it stopped instead of reconnecting.
            _check(reason.code == 4015, f"close code {reason.code}, expected 4015")


# ---- AG-12 ------------------------------------------------------------------------------------


def _op(op: str, path: str, value: Any = None) -> JsonPatchOperation:
    item = JsonPatchOperation(op=JsonPatchOperationOp(op), path=path)
    if op != "remove":
        item.value = value
    return item


async def ag12_patch_rules(server: ScenarioServer) -> None:
    """AG-12: forbidden paths and stale versions over REST and the gateway; the patcher rebases.

    Raises:
        ScenarioFailed: A check failed.
    """
    release = asyncio.Event()
    handler = RecordingHandler(block_execute=release)
    adapter = _adapter(server, handler, "ag12-adapter")
    client = _client(server)
    try:
        async with _running(adapter) as (rec, _):
            await _until(server, _ready(rec), "the adapter to connect")
            triggered = await _aw(server.trigger("AG-12 @Lotse"))
            run_id = triggered.run_id
            await _until(server, lambda: handler.count("execute") == 1, "the run to start")
            handle = handler.handles[run_id]
            try:
                await handle.patch.send_ops([_op("replace", "/state", "completed")])
            except ValueError:
                pass
            else:
                raise ScenarioFailed("send_ops accepted /state")
            version = (await _aw(server.run_state(run_id))).version
            try:
                await client.call(
                    patch_run.asyncio_detailed,
                    run_id=run_id,
                    body=RunPatch(
                        base_version=version, ops=[_op("replace", "/state", "completed")]
                    ),
                )
                raise ScenarioFailed("patchRun accepted /state")
            except ValidationFailed as exc:
                _check(bool(exc.errors), "no field errors")
                _check(exc.errors[0].code == "patch_path_forbidden", f"code {exc.errors[0].code}")
                _check(exc.errors[0].pointer == "/ops/0", f"pointer {exc.errors[0].pointer}")
            try:
                await client.call(
                    patch_run.asyncio_detailed,
                    run_id=run_id,
                    body=RunPatch(base_version=version - 1, ops=[_op("replace", "/model", "m")]),
                )
                raise ScenarioFailed("patchRun accepted a stale base_version")
            except BaseVersionConflict as exc:
                current = exc.run
                server_version = (await _aw(server.run_state(run_id))).version
                if current is None:
                    raise ScenarioFailed("no current document") from None
                _check(current.version == server_version, "current is not the server's version")
            # A concurrent patch moves the version; the patcher rebases.
            stale = handle.patch.base_version
            await client.call(
                patch_run.asyncio_detailed,
                run_id=run_id,
                body=RunPatch(
                    base_version=(await _aw(server.run_state(run_id))).version,
                    ops=[_op("replace", "/status_line", "external change")],
                ),
            )
            _check(handle.patch.base_version == stale, "the patcher saw the change too early")
            handle.patch.set_model("model-rebased")
            await handle.patch.flush()
            run = await client.call_as(Run, get_run.asyncio_detailed, run_id=run_id)
            _check(run.status_line == "external change", "the concurrent change is lost")
            _check(run.model == "model-rebased", "the rebased change is missing")
            await _gateway_cases(server, client, run_id)
            release.set()
    finally:
        await client.aclose()


async def _gateway_cases(server: ScenarioServer, client: Client, run_id: UUID) -> None:
    discovery = await client.discover()
    gateway = Gateway(
        gateway_url(server.base_url, discovery.gateway.url),
        server.credentials,
        consumer="ag12-gateway",
        agent_id=server.agent_id,
        settings=server.sdk_settings,
        clock=server.clock,
    )
    frames = gateway.frames()
    errors: list[GatewayOpError] = []

    async def read() -> None:
        async for frame in frames:
            if frame.op == "error":
                errors.append(frame.model)

    reader = asyncio.ensure_future(read())
    try:
        await _until(server, lambda: gateway.session_id is not None, "the gateway session")
        version = (await _aw(server.run_state(run_id))).version
        await gateway.send_run_update(run_id, version, [_op("replace", "/state", "x")], ref="g1")
        await _until(server, lambda: len(errors) >= 1, "the error of /state")
        _check(errors[0].code == "validation_failed", f"gateway code {errors[0].code}")
        await gateway.send_run_update(
            run_id, version - 1, [_op("replace", "/model", "g")], ref="g2"
        )
        await _until(server, lambda: len(errors) >= 2, "the error of the stale version")
        _check(errors[1].code == "base_version_conflict", f"gateway code {errors[1].code}")
    finally:
        await gateway.close()
        reader.cancel()
        with contextlib.suppress(asyncio.CancelledError, Exception):
            await reader


# ---- AG-28 ------------------------------------------------------------------------------------


async def ag28_slow_start_and_restart(server: ScenarioServer) -> None:
    """AG-28: acknowledgement after acceptance; a redelivery replaces the token; one start.

    Raises:
        ScenarioFailed: A check failed.
    """
    release = asyncio.Event()
    handler = RecordingHandler(block_prepare=release)
    counting = _Counting()
    consumer = "ag28-adapter"
    adapter = _adapter(server, handler, consumer, http_transport=counting)
    client = _client(server)
    try:
        async with _running(adapter) as (rec, _):
            await _until(server, _ready(rec), "the adapter to connect")
            triggered = await _aw(server.trigger("AG-28 @Lotse"))
            run_id = triggered.run_id
            await _until(server, lambda: handler.count("prepare") == 1, "prepare")
            env = _envelope_for(rec, run_id, "agent.invocation")[0]
            seq = env.seq or 0

            async def acked() -> bool:
                return (await _consumer(client, consumer)).acked_seq >= seq

            await _until(server, acked, "the acknowledgement while queued")
            _check((await _aw(server.run_state(run_id))).state == "queued", "the run is not queued")
            await server.pass_time(server.timings["agent.ack_timeout"] + 1.0)
            _check(
                len(_envelope_for(rec, run_id, "agent.invocation")) == 1,
                "the event was delivered again before the restart",
            )
            first_token = env.token
            await _aw(server.restart(consumer, seq - 1))
            await _until(
                server,
                lambda: len(_envelope_for(rec, run_id, "agent.invocation")) >= 2,
                "the redelivery after the restart",
                limit=120.0,
            )

            async def replaced() -> bool:
                entry = await adapter.inbox.get(env.id)
                return entry is not None and entry.deliveries >= 2 and entry.token != first_token

            await _until(server, replaced, "the newer token in the inbox")
            _check(handler.count("prepare") == 1, "the redelivery was prepared again")
            if first_token is None:
                raise ScenarioFailed("the delivered invocation had no token")
            await _expect_token_invalid(
                lambda: client.call(
                    start_run.asyncio_detailed,
                    run_id=run_id,
                    body=RunStart(
                        invocation_token=first_token.reveal(),
                        billing_mode_used=BillingMode(handler.invocations[0].claims.billing_mode),
                        tool_class_used=ToolClass.CHAT,
                    ),
                ),
                "superseded",
                "startRun with the first token",
            )
            await server.pass_time(
                server.timings["agent.invocation_token_ttl"]
                + server.timings["auth.clock_leeway"]
                + 1.0
            )
            renewals_before = counting.count("POST", "/invocation-token")
            release.set()
            await _until(server, lambda: handler.count("execute") == 1, "execute", limit=120.0)
            _check(
                counting.count("POST", "/invocation-token") - renewals_before == 1,
                "not exactly one renewal",
            )
            _check(counting.count("POST", "/start") == 1, "not exactly one start")
            handle = handler.handles[run_id]
            entry = await adapter.inbox.get(env.id)
            _check(
                entry is not None and handle.token_jti == entry.token_jti,
                "the start did not use the renewed token",
            )
            view = await _aw(server.run_state(run_id))
            _check(view.starts == 1, f"starts {view.starts}")
            _check(handler.count("execute") == 1, "execute ran more than once")
    finally:
        await client.aclose()
