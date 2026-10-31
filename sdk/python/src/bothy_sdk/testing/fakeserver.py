# SPDX-License-Identifier: Apache-2.0
"""In-process FakeServer for adapter tests (docs/spec/09-agents.md 16.1, extra ``testing``).

`FakeServer` serves the REST operations an agent adapter uses and the gateway on one aiohttp
listener on ``127.0.0.1`` with port 0. It plays the server rules of docs/spec/09-agents.md 4.3
to 4.10, 5.1 and 5.3 and docs/spec/04-sync-protocol.md 6, 11.2 and 15.1 to 15.5 as the server
code builds them: DPoP with nonces, consumers with cursors, a fresh invocation token per
delivery, redelivery after ``agent.ack_timeout``, the start, renew and finish verdicts and the
patch rules. Every time decision goes through the shared `FakeClock`; tests move time with
`FakeServer.pass_time`. Faults (`bothy_sdk.testing.faults`) make one operation fail on purpose.

Recorded requests never hold a token: the values of ``Authorization``, ``DPoP`` and
``Invocation-Token`` and every token member of a body are replaced by their `fingerprint`.

Examples:
    >>> import asyncio
    >>> async def demo() -> str:
    ...     async with FakeServer() as fake:
    ...         run = await fake.trigger()
    ...         return fake.run_state(run.run_id).state
    >>> asyncio.run(demo())
    'queued'
"""

from __future__ import annotations

import asyncio
import json
import re
import secrets
from collections import defaultdict, deque
from collections.abc import Awaitable, Mapping, Sequence
from dataclasses import dataclass
from types import TracebackType
from typing import Any
from uuid import UUID

from aiohttp import web
from cryptography.hazmat.primitives.asymmetric import ec

from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.generated.models import CommandChoice
from bothy_sdk.ids import uuid7
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing._fake_base import FakeProblem, field_error, timings_with_defaults
from bothy_sdk.testing._fake_dpop import Authenticator
from bothy_sdk.testing._fake_events import LogEntry, Window
from bothy_sdk.testing._fake_extra import (
    ContextFile,
    ContextMessage,
    byte_range,
    transcript_etag,
)
from bothy_sdk.testing._fake_gateway import Gateway
from bothy_sdk.testing._fake_hub import Hub
from bothy_sdk.testing._fake_tokens import FORGE_VARIANTS
from bothy_sdk.testing._fake_uploads import FakeUploads
from bothy_sdk.testing._fake_world import AUTO_SESSION, World, consumer_ok
from bothy_sdk.testing.doubles import FakeClock, fingerprint
from bothy_sdk.testing.faults import Fault, FaultKind

__all__ = [
    "ContextFile",
    "ContextMessage",
    "FakeServer",
    "RecordedRequest",
    "RunView",
    "Triggered",
]

_API = "/api/v1"
_GATEWAY_PATH = f"{_API}/gateway"
_TOKEN_MEMBERS = frozenset({"invocation_token", "api_token", "access_token", "dpop"})
_SECRET_HEADERS = frozenset({"authorization", "dpop", "invocation-token"})
_GATEWAY_OPS = frozenset(
    {"ack", "run.update", "heartbeat", "identify", "resume", "command.autocomplete_result"}
)
_PUBLIC = frozenset({"getDiscovery", "getJwks"})
# Consumer operations judge the agent by its own grants: the run binding is ignored there.
_DELIVERY = frozenset({"pollAgentEvents", "ackAgentEvents", "getConsumerState"})
_THREAD_OPS = frozenset({"updateMessage"})
_IDEMPOTENT_KEY_OPS = frozenset(
    {
        "updateComponent",
        "respondToInteraction",
        "registerCommand",
        "updateCommand",
        "removeCommand",
        "reportAgentSession",
        "createUpload",
        "completeUpload",
        "createThread",
    }
)
_ROUTES: tuple[tuple[str, str, str], ...] = (
    ("GET", "/instance/discovery", "getDiscovery"),
    ("GET", "/instance/jwks", "getJwks"),
    ("GET", "/agents/me/consumers/{consumer}/events", "pollAgentEvents"),
    ("POST", "/agents/me/consumers/{consumer}/ack", "ackAgentEvents"),
    ("GET", "/agents/me/consumers/{consumer}", "getConsumerState"),
    ("POST", "/agents/me/heartbeat", "sendHeartbeat"),
    ("GET", "/agents/me/triggerers", "getAgentTriggerers"),
    ("GET", "/agents/me/channels", "listServedChannels"),
    ("GET", "/agents/me/channels/{channel_id}/config", "getServedChannelConfig"),
    ("GET", "/runs/{run_id}", "getRun"),
    ("PATCH", "/runs/{run_id}", "patchRun"),
    ("GET", "/runs/{run_id}/context", "getRunContext"),
    ("POST", "/runs/{run_id}/start", "startRun"),
    ("POST", "/runs/{run_id}/resume", "resumeRun"),
    ("POST", "/runs/{run_id}/invocation-token", "renewInvocationToken"),
    ("POST", "/runs/{run_id}/await-input", "awaitRunInput"),
    ("POST", "/runs/{run_id}/finish", "finishRun"),
    ("POST", "/threads/{thread_id}/messages", "createMessage"),
    ("PATCH", "/components/{component_id}", "updateComponent"),
    ("POST", "/interactions/{interaction_id}/respond", "respondToInteraction"),
    ("POST", "/commands", "registerCommand"),
    ("PUT", "/commands/{command_id}", "updateCommand"),
    ("DELETE", "/commands/{command_id}", "removeCommand"),
    ("GET", "/channels/{channel_id}/commands", "listChannelCommands"),
    ("POST", "/command-autocompletions/{request_id}/answer", "answerAutocomplete"),
    ("GET", "/me", "getMe"),
    ("GET", "/messages/{message_id}", "getMessage"),
    ("GET", "/files/{file_id}/content", "downloadFile"),
    ("GET", "/files/{file_id}", "getFile"),
    ("PUT", "/files/{file_id}/transcript", "submitTranscript"),
    ("GET", "/threads/{thread_id}", "getThread"),
    ("GET", "/channels/{channel_id}", "getChannel"),
    ("POST", "/channels/{channel_id}/threads", "createThread"),
    ("PUT", "/threads/{thread_id}/title", "setThreadTitle"),
    ("POST", "/threads/{thread_id}/agent-sessions/report", "reportAgentSession"),
    ("PATCH", "/messages/{message_id}", "updateMessage"),
    ("GET", "/instance", "getInstance"),
    ("POST", "/channels/{channel_id}/uploads", "createUpload"),
    ("PUT", "/uploads/{file_id}/parts/{part_number}", "uploadPart"),
    ("POST", "/uploads/{file_id}/complete", "completeUpload"),
    ("GET", "/uploads/{file_id}", "getUpload"),
    ("DELETE", "/uploads/{file_id}", "abortUpload"),
)
_BINARY = frozenset({"uploadPart"})
_COMPILED = tuple(
    (method, re.compile("^" + re.sub(r"\{(\w+)\}", r"(?P<\1>[^/]+)", path) + "$"), op)
    for method, path, op in _ROUTES
)

_Answer = tuple[int, Any, dict[str, str]]


@dataclass(frozen=True)
class Triggered:
    """What `FakeServer.trigger` created.

    Attributes:
        run_id: The queued run.
        thread_id: The new thread.
        message_id: Alex's triggering message.
    """

    run_id: UUID
    thread_id: UUID
    message_id: UUID


@dataclass(frozen=True)
class RunView:
    """The server's view of a run for assertions.

    Attributes:
        state: The run state.
        state_reason: The reason, if any.
        version: The run version.
        starts: How often the run went to ``running`` (start and resume).
    """

    state: str
    state_reason: str | None
    version: int
    starts: int


@dataclass(frozen=True)
class RecordedRequest:
    """One REST request as the FakeServer received it, with tokens replaced by fingerprints.

    Attributes:
        method: The HTTP method.
        path: The path without query.
        operation_id: The ``operationId`` of the route, or ``""`` for an unknown route.
        headers: Lower-case header names; ``authorization``, ``dpop`` and ``invocation-token``
            hold `fingerprint` of their value.
        json: The JSON body with every token member fingerprinted, or ``None``.
        content: The raw body of a binary operation (``uploadPart``), else ``None``.
    """

    method: str
    path: str
    operation_id: str
    headers: Mapping[str, str]
    json: Any
    content: bytes | None = None


def _scrub(value: Any) -> Any:
    if isinstance(value, dict):
        return {
            k: fingerprint(v) if k in _TOKEN_MEMBERS and isinstance(v, str) and v else _scrub(v)
            for k, v in value.items()
        }
    if isinstance(value, list):
        return [_scrub(v) for v in value]
    return value


class FakeServer:
    """The in-process server; use it as an async context manager.

    Args:
        clock: The clock shared with the SDK under test; a new `FakeClock` when omitted.
        timings: Server keys to override (``agent.invocation_token_ttl``, ``auth.clock_leeway``,
            ``agent.ack_timeout``, ``agent.ack_persist_interval``, ``sync.heartbeat_interval``,
            ``sync.resume_window``, ``agent.max_inflight_events``,
            ``command.autocomplete_timeout``, ``agent.jwks_max_age``, ``agent.cancel_grace``,
            ``auth.dpop_nonce_ttl``, ``auth.dpop_proof_max_age``), in seconds or counts.
        dpop: Bind Lotse's API token to a DPoP key (``credentials.dpop``); else a bearer token.
        billing_mode: Lotse's configured billing mode (``personal`` or ``organization``).
        tool_class: Lotse's configured tool class (``chat``, ``scoped`` or ``host``).
        costs: Accept ``RunFinish.costs``; off by default, like the server before M2-22.

    Raises:
        ValueError: An unknown timing key.

    Attributes:
        origin: ``http://127.0.0.1:<port>`` after the start.
        base_url: ``origin + "/api/v1"``.
        gateway_url: The WebSocket URL of the gateway.
        gateway_htu: The ``htu`` a gateway proof must name (the HTTPS form the server builds).
        instance_id: The instance (``iss`` of every token).
        agent_id: Lotse, the agent under test.
        other_agent_id: Texter, a second agent.
        credentials: Lotse's API token and, with ``dpop``, its key.
        clock: The shared clock.
        timings: The server keys in effect.
        sdk_settings: Settings an adapter under test should use (small reconnect and retry
            waits, a leeway not above ``auth.clock_leeway``).
        requests: Every REST request received, in order.
        frames: Every gateway frame received from a client, tokens fingerprinted.
        heartbeats: Every heartbeat body received (REST and gateway member ``agent``).
        uploads: Upload sessions, received bytes and ``InstanceLimits`` (`FakeUploads`).
    """

    def __init__(  # noqa: PLR0913 - every option of the fake is a keyword
        self,
        *,
        clock: FakeClock | None = None,
        timings: Mapping[str, float] | None = None,
        dpop: bool = True,
        billing_mode: str = "organization",
        tool_class: str = "scoped",
        costs: bool = False,
    ) -> None:
        """Create the server; `start` (or ``async with``) opens the listener."""
        self.clock = clock or FakeClock()
        self.timings: Mapping[str, float] = timings_with_defaults(timings)
        self.requests: list[RecordedRequest] = []
        self.frames: list[dict[str, Any]] = []
        self._faults: dict[str, deque[Fault]] = defaultdict(deque)
        self._deprecated: dict[str, str] = {}
        self._idempotency: dict[tuple[str, str], tuple[str, _Answer | None]] = {}
        self._fixed_tokens: dict[int, str] = {}
        self._forged: dict[UUID, str | None] = {}
        self._tasks: set[asyncio.Task[None]] = set()
        self.hub = Hub(self.clock, self.timings, self._deliver)
        self.world = World(
            self.clock,
            self.timings,
            billing_mode=billing_mode,
            tool_class=tool_class,
            append=self.hub.append,
            costs=costs,
        )
        self.uploads = FakeUploads(self.world.now, self.world.lotse.id)
        key = FileDpopKey(ec.generate_private_key(ec.SECP256R1())) if dpop else None
        self.credentials = Credentials(
            api_token=Secret("bothy_" + secrets.token_urlsafe(32)), dpop=key
        )
        self._auth = Authenticator(
            self.credentials.api_token.reveal(), key.jkt if key else None, self.clock, self.timings
        )
        self.gateway = Gateway(
            self.world,
            self.hub,
            self._auth,
            lambda: self.gateway_htu,
            self._take_gateway_fault,
            lambda frame: self.frames.append(_scrub(frame)),
        )
        leeway = min(10.0, float(self.timings["auth.clock_leeway"]))
        self.sdk_settings = SdkSettings(
            retry_backoff_base=0.05,
            retry_backoff_cap=0.5,
            reconnect_backoff_base=0.05,
            reconnect_backoff_cap=0.5,
            invocation_token_leeway=leeway,
        )
        self.origin = ""
        self._runner: web.AppRunner | None = None

    # Identity -----------------------------------------------------------------------------

    @property
    def instance_id(self) -> UUID:
        """The instance ID."""
        return self.world.instance_id

    @property
    def agent_id(self) -> UUID:
        """Lotse's principal ID."""
        return self.world.lotse.id

    @property
    def other_agent_id(self) -> UUID:
        """Texter's principal ID."""
        return self.world.texter.id

    @property
    def base_url(self) -> str:
        """The REST base URL."""
        return self.origin + _API

    @property
    def gateway_url(self) -> str:
        """The gateway's WebSocket URL."""
        return "ws" + self.origin.removeprefix("http") + _GATEWAY_PATH

    @property
    def gateway_htu(self) -> str:
        """The ``htu`` of a gateway proof: always HTTPS with the server's host and port."""
        return "https" + self.origin.removeprefix("http") + _GATEWAY_PATH

    @property
    def messages(self) -> list[dict[str, Any]]:
        """Stored ``createMessage`` bodies (with ``client_msg_id``), in order."""
        return self.world.message_bodies

    @property
    def commands(self) -> dict[UUID, dict[str, Any]]:
        """Registered commands by ID."""
        return self.world.commands

    @property
    def heartbeats(self) -> list[dict[str, Any]]:
        """Heartbeat bodies received, in order."""
        return self.world.heartbeats

    # Life cycle ---------------------------------------------------------------------------

    async def start(self) -> None:
        """Open the listener on ``127.0.0.1`` with a free port."""
        app = web.Application(client_max_size=4 << 20)
        app.router.add_get(_GATEWAY_PATH, self.gateway.handle)
        app.router.add_route("*", "/{tail:.*}", self._http)
        self._runner = web.AppRunner(app, access_log=None, handle_signals=False)
        await self._runner.setup()
        site = web.TCPSite(self._runner, "127.0.0.1", 0)
        await site.start()
        host, port = self._runner.addresses[0][:2]
        self.origin = f"http://{host}:{port}"

    async def stop(self) -> None:
        """Close every connection and the listener."""
        for task in list(self._tasks):
            task.cancel()
        await self.gateway.close_all(1001, "going_away")
        if self._runner is not None:
            await self._runner.cleanup()
            self._runner = None

    async def __aenter__(self) -> FakeServer:
        """Start the server."""
        await self.start()
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        """Stop the server."""
        await self.stop()

    # Scenario helpers ---------------------------------------------------------------------

    async def trigger(  # noqa: PLR0913 - the parts of a trigger a scenario chooses
        self,
        text: str = "Hello @Lotse",
        *,
        channel_id: UUID | None = None,
        author: str = "alex",
        trigger_kind: str = "message",
        attachments: tuple[UUID, ...] | list[UUID] = (),
        thread_id: UUID | None = None,
        context: Mapping[str, Any] | None = None,
        history: tuple[ContextMessage, ...] = (),
        files: tuple[ContextFile, ...] = (),
        session_mode: str | object | None = AUTO_SESSION,
        forged: str | None = None,
        command: str | None = None,
        options: Mapping[str, str] | None = None,
    ) -> Triggered:
        """A person posts ``text``: a queued run and its ``agent.invocation``.

        The payload carries the thread's current agent session, if any (M2-12).

        Args:
            text: The message content.
            channel_id: A served channel (`add_forum`); Allgemein when omitted.
            author: The handle of the triggering person (``alex``, ``sam`` or ``robin``).
            trigger_kind: ``message`` or ``interaction`` (the token names that object).
            attachments: Files of `add_file` attached to the message.
            thread_id: An existing thread to post in; a new thread when omitted.
            context: A ``RunContext`` to serve for the run (``thread_context``,
                ``channel_context``, ``context_floor_seq``, ...); its ``trigger_message`` is the
                triggering message unless given. When omitted the thread is the context.
            history: Messages posted in the thread before the trigger, with their trust marks.
            files: Attachments of the triggering message with their content.
            session_mode: Left out, the payload carries the thread's current session (``resume``
                when started, else ``new``); ``None`` makes a guest run without a session (09
                3.6); ``new``, ``resume`` or ``bind`` send that mode for an operator run (09 9.1).
            forged: Every delivery of the run's ``agent.invocation`` carries a token of this
                kind instead of a fresh one, and nothing is recorded on the run (AG-08 and
                AZ-P6 against an adapter in another process): ``missing`` (no token), a forge
                variant of `forge_token`, or ``expired`` (well formed, ``exp`` in the past).
            command: The name of a registered command of Lotse: a model command run
                (``trigger_kind = command``) whose payload names it with ``options``.
            options: The option values of ``command`` by name.

        Returns:
            The run, thread and message.

        Raises:
            ValueError: An unknown ``forged`` kind.
        """
        if forged is not None and forged not in ("missing", "expired", *FORGE_VARIANTS):
            raise ValueError(f"unknown token kind: {forged}")
        definition = None
        if command is not None:
            found = [c for c in self.world.commands.values() if c["name"] == command]
            if not found:
                raise ValueError(f"no such command: {command}")
            definition = (found[0], dict(options or {}))
            trigger_kind = "command"
        run = self.world.trigger(
            text,
            channel_id=channel_id,
            author=author,
            trigger_kind=trigger_kind,
            command=definition,
            attachments=tuple(attachments),
            thread_id=thread_id,
            context=context,
            history=history,
            files=files,
            session_mode=session_mode,
        )
        if forged is not None:
            # Set before the first delivery: the deliverers run only after the next await.
            self._forged[run.id] = self._forged_token(run.id, forged)
        await _settle()
        return Triggered(run.id, run.thread_id, run.triggering_message_id)

    def add_forum(  # noqa: PLR0913 - the parts of a forum a scenario chooses
        self,
        name: str,
        workspace_ref: str | None,
        *,
        model: str | None = None,
        allowed_tools: list[str] | None = None,
        prompt: str = "",
        served: bool = True,
    ) -> UUID:
        """A channel Lotse serves, with a project binding unless ``workspace_ref`` is ``None``.

        Args:
            name: A label for the test (not stored).
            workspace_ref: The binding's ``workspace_ref``; ``None`` for a channel without one.
            model: ``runner_config.model``.
            allowed_tools: ``runner_config.allowed_tools``.
            prompt: ``behavior.prompt``.
            served: ``False`` makes another agent its operator: Lotse does not serve it until
                `set_operator`.

        Returns:
            The channel ID.
        """
        del name
        return self.world.add_forum(
            workspace_ref, model=model, allowed_tools=allowed_tools, prompt=prompt, served=served
        )

    def update_forum(self, channel_id: UUID, **changes: Any) -> None:
        """Change a forum and deliver ``agent.config_changed`` (``settings_version`` + 1).

        Args:
            channel_id: The forum.
            **changes: ``workspace_ref``, ``model``, ``allowed_tools``, ``provision``,
                ``repository_url`` (M4-43) or ``prompt``.
        """
        self.world.update_forum(channel_id, **changes)

    def unbind_forum(self, channel_id: UUID) -> None:
        """Delete the project binding of a served forum (``agent.config_changed``).

        Args:
            channel_id: The forum.
        """
        self.world.unbind_forum(channel_id)

    def set_operator(self, channel_id: UUID, *, served: bool = True, announce: bool = True) -> None:
        """Make Lotse the operator of a forum, or (``served=False``) another agent.

        Delivers only ``access.changed`` with the channel under ``added`` or ``removed``, as the
        server does for an operator change (04 8.2); no ``agent.config_changed``.

        Args:
            channel_id: A forum of `add_forum`.
            served: Whether Lotse serves it afterwards.
            announce: ``False`` writes no ``access.changed`` (the server writes none when the
                visible targets of the agent do not change).
        """
        self.world.set_operator(channel_id, served=served, announce=announce)

    def remove_channel(self, channel_id: UUID) -> None:
        """Lotse stops serving the channel (``agent.config_changed``, then ``404``).

        Args:
            channel_id: The channel.
        """
        self.world.remove_channel(channel_id)

    def add_file(
        self,
        content: bytes,
        *,
        name: str,
        mime: str,
        status: str = "clean",
        transcript: str | None = None,
    ) -> UUID:
        """Store a file for attachments and ``downloadFile``.

        Args:
            content: The bytes.
            name: ``original_name``.
            mime: ``mime_type``.
            status: ``scan_status``: ``clean``, ``skipped``, ``pending`` or ``blocked``.
            transcript: The transcript of a voice attachment, if any.

        Returns:
            The file ID.
        """
        return self.world.add_file(
            content, name=name, mime=mime, status=status, transcript=transcript
        )

    def set_file_status(self, file_id: UUID, status: str) -> None:
        """End or change the check of a stored file, as the scan pass of the server does.

        Args:
            file_id: A file of `add_file`.
            status: ``clean``, ``skipped``, ``pending`` or ``blocked``.
        """
        self.world.files[file_id]["status"] = status

    def file_etag(self, file_id: UUID) -> str:
        """The current transcript ``ETag`` of a stored file, as ``getFile`` sends it.

        Args:
            file_id: A file of `add_file` or of a trigger's ``files``.

        Returns:
            The quoted value of the derived transcript ``ETag`` (03 R-DM-48).
        """
        return transcript_etag(self.world.files[file_id])

    def set_context(self, run_id: UUID, doc: Mapping[str, Any]) -> None:
        """Serve ``doc`` as the run's ``getRunContext`` (its ``run_id`` is set to the run).

        Args:
            run_id: The run.
            doc: A ``RunContext`` document.
        """
        self.world.runs[run_id].context = dict(doc)

    @property
    def created_threads(self) -> list[UUID]:
        """Threads Lotse opened itself with ``createThread`` (forum posts of reports)."""
        return list(self.world.created_threads)

    def main_thread(self, channel_id: UUID | None = None) -> UUID:
        """The main thread of a text channel (Allgemein when omitted).

        Args:
            channel_id: The channel.

        Returns:
            The thread ID.
        """
        return self.world.main_thread(channel_id or self.world.channel_id)

    def thread(self, thread_id: UUID) -> dict[str, Any]:
        """The ``getThread`` body of a thread.

        Args:
            thread_id: The thread.

        Returns:
            The Thread document.
        """
        return self.world.thread_doc(str(thread_id))

    def set_title_source(self, thread_id: UUID, source: str) -> None:
        """Set ``title_source`` (``manual`` makes ``setThreadTitle`` answer ``title_manual``).

        Args:
            thread_id: The thread.
            source: ``auto`` or ``manual``.
        """
        self.world.thread_meta[thread_id]["title_source"] = source

    def sessions(self, thread_id: UUID) -> list[dict[str, Any]]:
        """The agent session rows of a thread.

        Args:
            thread_id: The thread.

        Returns:
            The rows, oldest first.
        """
        return [dict(r) for r in self.world.sessions.get(thread_id, [])]

    def _forged_token(self, run_id: UUID, kind: str) -> str | None:
        if kind == "missing":
            return None
        if kind != "expired":
            return self.forge_token(run_id, kind).reveal()
        claims = self.world.invocation_claims(self.world.runs[run_id])
        back = int(self.timings["agent.invocation_token_ttl"] + self.timings["auth.clock_leeway"])
        claims["iat"] -= 2 * back
        claims["exp"] -= 2 * back
        claims = {k: v for k, v in claims.items() if k not in ("iss", "jti")}
        token, _ = self.world.issuer.issue(claims)
        return token

    def set_channel(self, prompt: str = "", skills: tuple[str, ...] = ()) -> None:
        """Change the channel prompt and skills of Allgemein (``settings_version`` + 1).

        Args:
            prompt: The channel prompt.
            skills: The skill names, in order.
        """
        behavior = self.world.channels[self.world.channel_id]["behavior"]
        behavior["prompt"] = prompt
        behavior["skills"] = list(skills)
        behavior["settings_version"] += 1

    def enable_costs(self) -> None:
        """Accept ``costs`` in ``finishRun`` (the server refuses them until M2-22)."""
        self.world.costs_enabled = True

    def start_session(self, thread_id: UUID, ref: str, *, started: bool = True) -> None:
        """Give a thread a current session row with ``ref`` (a reset, bind or import did).

        Args:
            thread_id: The thread.
            ref: The external session reference.
            started: Whether a runtime already confirmed it.
        """
        self.world.session_for(thread_id, ref, started=started)

    def session_reports(self, thread_id: UUID) -> list[dict[str, Any]]:
        """The ``reportAgentSession`` bodies stored for a thread, in order.

        Args:
            thread_id: The thread.

        Returns:
            The bodies (empty without a report).
        """
        return list(self.world.extra.reports.get(thread_id, []))

    def set_manual_title(self, thread_id: UUID, title: str) -> None:
        """A person sets the thread's title: ``title_source`` becomes ``manual`` (09 12).

        Args:
            thread_id: The thread.
            title: The title.
        """
        meta = self.world.thread_meta[thread_id]
        meta["title"], meta["title_source"] = title, "manual"
        meta["version"] += 1

    @property
    def titles(self) -> list[dict[str, Any]]:
        """The titles the adapter set with ``setThreadTitle``, in order."""
        return list(self.world.extra.titles_set)

    @property
    def message_updates(self) -> list[dict[str, Any]]:
        """The ``updateMessage`` calls stored, in order (``message_id`` and ``body``)."""
        return list(self.world.extra.message_updates)

    def message(self, message_id: UUID) -> dict[str, Any]:
        """A stored message.

        Args:
            message_id: The message.

        Returns:
            The Message body.
        """
        return self.world.messages[message_id]

    def component(self, component_id: UUID) -> dict[str, Any]:
        """A stored component.

        Args:
            component_id: The component.

        Returns:
            The component with its ``message_id``.
        """
        return self.world.components[component_id]

    @property
    def sam_id(self) -> UUID:
        """Sam's principal ID."""
        return self.world.sam.id

    @property
    def robin_id(self) -> UUID:
        """Robin's principal ID."""
        return self.world.robin.id

    async def deliver_followup(
        self,
        run_id: UUID,
        text: str = "One more thing",
        *,
        author: str = "alex",
        claims: Mapping[str, Any] | None = None,
        files: Sequence[UUID] = (),
    ) -> UUID:
        """A person posts a follow-up to the run: its row ``queued`` and ``agent.followup``.

        Args:
            run_id: The run.
            text: The message content.
            author: ``alex``, ``robin`` or ``sam``; ``sub`` of the token.
            claims: Claims that replace those of every token of this event (still signed with
                the server key), for example ``{"billing_mode": "personal"}``,
                ``{"tool_class": "chat"}`` or ``{"sub": "<another principal>"}``.
            files: Files of `add_file` attached to the message, in their order; ``getMessage``
                shows each with its state at the time of the read, as the server does (the
                follow-up is delivered at once, also while a file is still checked, 12 10).

        Returns:
            The follow-up message ID.
        """
        person = {"alex": self.world.alex, "robin": self.world.robin, "sam": self.world.sam}[author]
        message_id = self.world.followup(run_id, text, author=person, claims=claims, files=files)
        await _settle()
        return message_id

    def edit_message(
        self, message_id: UUID, *, content_md: str | None = None, deleted: bool = False
    ) -> None:
        """Change a stored message as ``getMessage`` will answer it.

        Args:
            message_id: The message.
            content_md: A new content, if given.
            deleted: Set ``deleted_at`` (the content stays as given).
        """
        message = self.world.messages[message_id]
        if content_md is not None:
            message["content_md"] = content_md
        if deleted:
            message["deleted_at"] = self.world.now_iso()

    def hide_message(self, message_id: UUID) -> None:
        """Let ``getMessage`` answer ``404 not_found`` for a message.

        Args:
            message_id: The message.
        """
        self.world.hidden_messages.add(message_id)

    def set_run_state(self, run_id: UUID, state: str, reason: str | None = None) -> None:
        """End or move a run as the server would (for example ``failed`` with ``stale``).

        Args:
            run_id: The run.
            state: The new state.
            reason: The ``state_reason``.
        """
        self.world.transition(self.world.runs[run_id], state, reason)

    def followup_state(self, run_id: UUID, message_id: UUID) -> str | None:
        """The state of a follow-up row of a run, or ``None`` without such a row.

        Args:
            run_id: The run.
            message_id: The follow-up message.

        Returns:
            ``queued``, ``delivered``, ``absorbed`` ...
        """
        row = self.world.runs[run_id].live.get("followups", {}).get(str(message_id))
        return None if row is None else str(row["state"])

    async def deliver_interaction(
        self,
        run_id: UUID,
        custom_id: str = "approval.once",
        values: Mapping[str, Any] | None = None,
    ) -> UUID:
        """Alex operates a component of the run: ``interaction.created``.

        Args:
            run_id: The run.
            custom_id: The component's custom ID (created on a card of the run if missing).
            values: The submitted values.

        Returns:
            The interaction ID.
        """
        interaction_id = self.world.interaction(run_id, custom_id, values or {})
        await _settle()
        return interaction_id

    async def click(
        self,
        run_id: UUID,
        custom_id: str,
        values: Mapping[str, Any] | None = None,
        *,
        author: str = "alex",
    ) -> UUID | None:
        """A person clicks a component of the run's card, with the single-use rule (09 11.3).

        Args:
            run_id: The run.
            custom_id: The component's custom ID (it must exist on a card of the run).
            values: The submitted values.
            author: The clicking person's handle.

        Returns:
            The interaction ID, or ``None`` when the server answered ``409 component_used``.

        Raises:
            KeyError: No component of the run has this custom ID.
        """
        interaction_id = self.world.click(run_id, custom_id, values or {}, author)
        await _settle()
        return interaction_id

    async def register_command(self, name: str) -> UUID:
        """Register a command of Lotse in the channel (as ``registerCommand`` would).

        Args:
            name: The command name.

        Returns:
            The command ID.
        """
        command = self.world.register_command(
            {
                "name": name,
                "description": f"The {name} command",
                "scope_kind": "channel",
                "scope_id": str(self.world.channel_id),
            }
        )
        return UUID(command["id"])

    async def invoke_command(
        self,
        name: str,
        *,
        run_id: UUID | None = None,
        thread_id: UUID | None = None,
        options: Mapping[str, str] | None = None,
    ) -> UUID:
        """Alex runs Lotse's command ``name`` (registered first if missing): ``command.invoked``.

        Args:
            name: The command name.
            run_id: The run of the thread, if any (its thread is used).
            thread_id: The thread without a run, if any.
            options: The option values by name.

        Returns:
            The invocation ID of the payload.
        """
        command = next((c for c in self.world.commands.values() if c["name"] == name), None)
        if command is None:
            command = self.world.commands[await self.register_command(name)]
        invocation_id = self.world.command_invoked(command, run_id, thread_id, options)
        await _settle()
        return invocation_id

    async def request_autocomplete(
        self, command_id: UUID, value: str = "", *, expires_in: float | None = None
    ) -> UUID:
        """Send ``command.autocomplete_requested`` to a live gateway connection of Lotse.

        Args:
            command_id: A registered command.
            value: What Alex has typed so far.
            expires_in: Seconds until ``expires_at``; ``command.autocomplete_timeout`` when
                omitted.

        Returns:
            The request ID.

        Raises:
            KeyError: The command is not registered.
            RuntimeError: No gateway connection of Lotse is live.
        """
        command = self.world.commands[command_id]
        timeout = self.timings["command.autocomplete_timeout"] if expires_in is None else expires_in
        envelope = self.world.autocomplete_event(command, value, timeout)
        if not await self.hub.ephemeral(envelope):
            raise RuntimeError("no live gateway connection")
        return UUID(envelope["payload"]["request_id"])

    def autocomplete_answer(self, request_id: UUID) -> list[CommandChoice] | None:
        """What the adapter answered before ``expires_at``, if anything.

        Args:
            request_id: The request.

        Returns:
            The choices, or ``None`` when no answer arrived in time.
        """
        choices = self.world.autocomplete_answers.get(request_id)
        return None if choices is None else [CommandChoice.from_dict(c) for c in choices]

    async def cancel(self, run_id: UUID, reason: str = "user_cancelled") -> None:
        """A person cancels the run: ``run.cancel_requested``.

        A queued run ends ``cancelled`` at once; an active run waits ``agent.cancel_grace`` for
        the adapter's ``cancelled`` and then ends with ``cancel_timeout``.

        Args:
            run_id: The run.
            reason: The payload's reason.

        Raises:
            ValueError: The run has completed or failed.
        """
        run = self.world.runs[run_id]
        if self.world.cancel(run, reason):
            self._spawn(self._cancel_timeout(run_id))
        await _settle()

    async def _cancel_timeout(self, run_id: UUID) -> None:
        await self.clock.sleep(self.timings["agent.cancel_grace"])
        run = self.world.runs[run_id]
        if run.state in ("running", "waiting_input"):
            self.world.transition(run, "cancelled", "cancel_timeout")

    async def redeliver(self, consumer: str | None = None) -> None:
        """Deliver every unacknowledged event again now, with fresh tokens and ``redelivered``.

        Args:
            consumer: Only this consumer; every consumer when omitted.
        """
        windows: list[Window] = [w for name, w in self.hub.rest.items() if consumer in (None, name)]
        conns = [c for name, c in self.hub.connections.items() if consumer in (None, name)]
        for window in [*windows, *(c.window for c in conns)]:
            window.redeliver_up_to = max(window.redeliver_up_to, window.highest_sent)
            window.inflight.clear()
            window.read = window.acked
        for conn in conns:
            conn.wake.set()
        await _settle()

    async def deliver_with_token(self, envelope_id: UUID, token: Secret) -> UUID:
        """Deliver an existing event's payload again under a new event ID with ``token``.

        The replay case of AZ-P6: the same token on a second event.

        Args:
            envelope_id: The ID of an event in the log.
            token: The token the new delivery carries (not recorded on the run).

        Returns:
            The new event ID.

        Raises:
            KeyError: No event with that ID.
        """
        entry = next((e for e in self.hub.log if e.envelope["id"] == str(envelope_id)), None)
        if entry is None:
            raise KeyError("no such event")
        envelope = {k: v for k, v in entry.envelope.items() if k != "seq"}
        envelope["id"] = str(uuid7())
        new = self.hub.append(entry.type, envelope)
        self._fixed_tokens[new.seq] = token.reveal()
        await _settle()
        return UUID(envelope["id"])

    def forge_token(self, run_id: UUID, variant: str) -> Secret:
        """A forged token for the run, built from its newest delivered token.

        A run without a delivery yet gets the claims its ``agent.invocation`` would carry; nothing
        is recorded on the run, so it stays as it is (AG-08 sends these to a queued run).

        Args:
            run_id: A run that has had at least one delivery.
            variant: ``bad_signature``, ``alg_hs256``, ``typ_access`` or ``aud_other_agent``.

        Returns:
            The forged token.

        Raises:
            ValueError: An unknown variant.
        """
        if variant not in FORGE_VARIANTS:
            raise ValueError(f"unknown forge variant: {variant}")
        run = self.world.runs[run_id]
        claims = run.issued.get(run.last_jti or "") or self.world.invocation_claims(run)
        return self.world.issuer.forge(claims, variant, other_agent_id=self.other_agent_id)

    def run_state(self, run_id: UUID) -> RunView:
        """The run's state, reason, version and start count.

        Args:
            run_id: The run.

        Returns:
            The view.
        """
        run = self.world.runs[run_id]
        return RunView(run.state, run.state_reason, run.version, run.starts)

    def run(self, run_id: UUID) -> dict[str, Any]:
        """The ``getRun`` body of the run.

        Args:
            run_id: The run.

        Returns:
            The Run document.
        """
        return self.world.run_doc(self.world.runs[run_id])

    def consumer_state(self, consumer: str) -> tuple[int, int]:
        """The consumer's ``(acked_seq, head_seq)``.

        Args:
            consumer: The consumer name.

        Returns:
            The stored acknowledgement and the head.

        Raises:
            KeyError: The consumer never connected.
        """
        cursor = self.hub.cursors.get(consumer)
        if cursor is None:
            raise KeyError(consumer)
        return cursor.acked, self.hub.head

    async def restart(self, consumer: str | None = None, acked_seq: int | None = None) -> None:
        """Restart the server process and keep its data.

        Every gateway connection closes with ``4013`` (as the server's shutdown does), sessions
        and delivery windows in memory are forgotten, and one consumer's cursor may be set.

        Args:
            consumer: The consumer whose cursor is set.
            acked_seq: The cursor value (for example the ``seq`` before an event, AG-28).
        """
        self.gateway.draining = True
        try:
            await self.gateway.close_all(4013, "server_restart")
            self.gateway.forget_sessions()
            self.hub.restart(consumer, acked_seq)
        finally:
            self.gateway.draining = False
        await _settle()

    async def pass_time(self, seconds: float) -> None:
        """Move the shared clock forward (`FakeClock.advance`).

        Args:
            seconds: How far.
        """
        await self.clock.advance(seconds)
        await _settle()

    def inject(self, target: str, fault: Fault, times: int = 1) -> None:
        """Let the next ``times`` calls of ``target`` fail with ``fault``.

        Args:
            target: An ``operationId`` or a gateway op (``ack``, ``run.update``,
                ``heartbeat``, ...); on a gateway op every fault drops the connection.
            fault: The fault.
            times: How many calls it hits.
        """
        self._faults[target].extend([fault] * times)

    async def close_connections(self, code: int) -> None:
        """Close every gateway connection with ``code``.

        Args:
            code: A close code of docs/api/asyncapi.yaml.
        """
        await self.gateway.close_all(code, "closed")
        await _settle()

    def deprecate(self, operation_id: str, sunset: str = "Wed, 30 Jun 2027 00:00:00 GMT") -> None:
        """Mark an operation deprecated: its answers carry ``Deprecation: true`` and ``Sunset``.

        Args:
            operation_id: The operation.
            sunset: The HTTP date of the ``Sunset`` header.
        """
        self._deprecated[operation_id] = sunset

    def rotate_nonce(self) -> None:
        """Start a new DPoP nonce window now; the previous nonce stays valid one window."""
        self._auth.nonces.rotate()

    # Internals ----------------------------------------------------------------------------

    def _spawn(self, coro: Awaitable[None]) -> None:
        task: asyncio.Task[None] = asyncio.ensure_future(coro)
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)

    def _deliver(self, entry: LogEntry) -> dict[str, Any] | None:
        fixed = self._fixed_tokens.get(entry.seq)
        if fixed is not None:
            return {**entry.envelope, "invocation_token": fixed}
        run_id = entry.envelope.get("payload", {}).get("run_id")
        if entry.type == "agent.invocation" and run_id and UUID(run_id) in self._forged:
            forged = self._forged[UUID(run_id)]
            envelope = {k: v for k, v in entry.envelope.items() if k != "invocation_token"}
            if forged is not None:
                envelope["invocation_token"] = forged
            return envelope
        return self.world.deliver(entry)

    def _take_gateway_fault(self, op: str) -> bool:
        queue = self._faults.get(op)
        if op in _GATEWAY_OPS and queue:
            queue.popleft()
            return True
        return False

    def _take_fault(self, operation_id: str) -> Fault | None:
        queue = self._faults.get(operation_id)
        return queue.popleft() if queue else None

    async def _http(self, request: web.Request) -> web.StreamResponse:  # noqa: PLR0912
        path = request.rel_url.raw_path
        method = request.method
        operation_id, params = "", {}
        if path.startswith(_API):
            for route_method, pattern, op in _COMPILED:
                match = pattern.match(path[len(_API) :])
                if match and route_method == method:
                    operation_id, params = op, match.groupdict()
                    break
        raw = await request.read()
        body: Any = None
        malformed = False
        if raw and operation_id not in _BINARY:
            try:
                body = json.loads(raw)
            except ValueError:
                malformed = True
        headers = {
            k.lower(): fingerprint(v) if k.lower() in _SECRET_HEADERS else v
            for k, v in request.headers.items()
        }
        content = raw if operation_id in _BINARY else None
        self.requests.append(
            RecordedRequest(method, path, operation_id, headers, _scrub(body), content)
        )
        try:
            if not operation_id:
                raise FakeProblem("not_found")
            if malformed:
                raise FakeProblem("malformed_request")
            self._authenticate(operation_id, request)
            # Faults hit an authenticated call, so a nonce challenge does not use one up.
            fault = self._take_fault(operation_id)
            if fault is not None and fault.kind is FaultKind.DISCONNECT:
                if request.transport is not None:
                    request.transport.close()
                raise asyncio.CancelledError
            if fault is not None and fault.kind is FaultKind.PROBLEM:
                raise FakeProblem(fault.code, reason=fault.reason)
            if fault is not None and fault.kind is FaultKind.PROXY_ERROR:
                return web.Response(
                    status=fault.status, text="proxy error", content_type="text/plain"
                )
            if fault is not None and fault.kind is FaultKind.RATE_LIMITED:
                raise FakeProblem("rate_limited", headers={"Retry-After": str(fault.retry_after)})
            if fault is not None and fault.kind is FaultKind.HOLD and fault.gate is not None:
                await fault.gate.hold()
            if operation_id == "downloadFile":
                return self._download(params["file_id"], request)
            if fault is not None and fault.kind is FaultKind.STORED_LATE:
                await self.clock.sleep(fault.delay)
            if operation_id in _BINARY:
                body = raw
            status, out, extra = await self._serve(operation_id, params, request, body)
        except FakeProblem as problem:
            return self._problem(problem, path, operation_id)
        if fault is not None and fault.kind is FaultKind.STORED_THEN_502:
            return self._problem(FakeProblem("bad_gateway", status=502), path, operation_id)
        return self._respond(status, out, extra, operation_id)

    def _download(self, file_id: str, request: web.Request) -> web.Response:
        content, mime = self.world.download(file_id)
        status, content, headers = byte_range(content, mime, request.headers.get("Range"))
        out = dict(headers)
        out["Correlation-Id"] = str(uuid7())
        if self.credentials.dpop is not None:
            out["DPoP-Nonce"] = self._auth.nonces.current()
        return web.Response(status=status, body=content, headers=out)

    def _respond(
        self, status: int, body: Any, headers: Mapping[str, str], operation_id: str
    ) -> web.Response:
        out = dict(headers)
        out["Correlation-Id"] = str(uuid7())
        if self.credentials.dpop is not None:
            out["DPoP-Nonce"] = self._auth.nonces.current()
        if operation_id in self._deprecated:
            out["Deprecation"] = "true"
            out["Sunset"] = self._deprecated[operation_id]
        if body is None:
            return web.Response(status=status, headers=out)
        if isinstance(body, bytes):
            return web.Response(status=status, body=body, headers=out)
        content_type = out.pop("Content-Type", "application/json")
        return web.Response(
            status=status, text=json.dumps(body), content_type=content_type, headers=out
        )

    def _problem(self, problem: FakeProblem, path: str, operation_id: str) -> web.Response:
        headers = {"Content-Type": "application/problem+json", **problem.headers}
        return self._respond(problem.status, problem.body(path), headers, operation_id)

    def _authenticate(self, operation_id: str, request: web.Request) -> None:
        binding = request.headers.getall("Invocation-Token", [])
        if operation_id not in _PUBLIC:
            authorization = request.headers.getall("Authorization", [])
            if binding and not authorization:
                raise FakeProblem(
                    "unauthenticated", headers={"WWW-Authenticate": 'DPoP algs="ES256 EdDSA"'}
                )
            self._auth.authenticate(
                method=request.method,
                htu=self.origin + request.rel_url.raw_path,
                authorization=authorization,
                proofs=request.headers.getall("DPoP", []),
            )
            if operation_id not in _DELIVERY:
                self.world.bind(binding, operation_id)

    async def _serve(
        self, operation_id: str, params: dict[str, str], request: web.Request, body: Any
    ) -> _Answer:
        key = request.headers.get("Idempotency-Key")
        if key and operation_id in _IDEMPOTENT_KEY_OPS:
            return await self._idempotent(key, operation_id, params, request, body)
        return await self._operation(operation_id, params, request, body)

    async def _idempotent(
        self, key: str, operation_id: str, params: dict[str, str], request: web.Request, body: Any
    ) -> _Answer:
        signature = json.dumps([request.method, request.rel_url.raw_path, body], sort_keys=True)
        stored = self._idempotency.get((operation_id, key))
        if stored is not None:
            if stored[0] != signature:
                raise FakeProblem("idempotency_key_reuse")
            if stored[1] is None:
                raise FakeProblem("request_in_progress")
            status, out, headers = stored[1]
            return status, out, {**headers, "Idempotent-Replay": "true"}
        self._idempotency[(operation_id, key)] = (signature, None)
        try:
            answer = await self._operation(operation_id, params, request, body)
        except FakeProblem:
            del self._idempotency[(operation_id, key)]
            raise
        self._idempotency[(operation_id, key)] = (signature, answer)
        return answer

    async def _operation(  # noqa: PLR0911, PLR0912, PLR0915 - one branch per operation
        self, op: str, p: dict[str, str], request: web.Request, body: Any
    ) -> _Answer:
        world = self.world
        if op == "getDiscovery":
            from bothy_sdk.testing import _fake_fixtures  # noqa: PLC0415 - keeps imports flat

            doc = _fake_fixtures.build(
                "discovery",
                {
                    "instance_id": str(world.instance_id),
                    "gateway": {"url": _GATEWAY_PATH, "protocol_versions": [1]},
                    "jwks_uri": f"{_API}/instance/jwks",
                },
            )
            return 200, doc, {}
        if op == "getJwks":
            max_age = int(self.timings["agent.jwks_max_age"])
            return (
                200,
                world.issuer.jwks(),
                {
                    "Content-Type": "application/jwk-set+json",
                    "Cache-Control": f"public, max-age={max_age}",
                },
            )
        if op in ("pollAgentEvents", "ackAgentEvents", "getConsumerState"):
            consumer = p["consumer"]
            if not consumer_ok(consumer):
                raise field_error("#path/consumer", "pattern_mismatch")
            if op == "getConsumerState":
                return 200, self.hub.state(consumer), {}
            if op == "ackAgentEvents":
                if not isinstance(body, dict) or set(body) - {"seq"} or "seq" not in body:
                    raise field_error("/seq", "required")
                seq = body["seq"]
                if not isinstance(seq, int) or isinstance(seq, bool) or seq < 0:
                    raise field_error("/seq", "too_small")
                return 200, self.hub.ack(consumer, seq), {}
            limit = request.query.get("limit")
            if limit is not None and (not limit.isdigit() or int(limit) < 1):
                raise field_error("#query/limit", "too_small")
            page = await self.hub.poll(consumer, int(limit) if limit else None)
            page["sync_epoch"] = str(world.sync_epoch)
            return 200, page, {}
        if op == "sendHeartbeat":
            return 200, world.heartbeat(body), {}
        if op == "getAgentTriggerers":
            return (
                200,
                {
                    "principal_ids": [str(world.alex.id), str(world.robin.id), str(world.sam.id)],
                    "permission_version": world.permission_version,
                },
                {},
            )
        if op == "listServedChannels":
            return 200, {"items": [world.channel_config(c) for c in world.channels]}, {}
        if op == "getServedChannelConfig":
            channel = world.channel_or_404(p["channel_id"])
            config = world.channel_config(channel)
            return 200, config, {"ETag": f'"{config["behavior"]["settings_version"]}"'}
        if op == "getMe":
            me = {"principal": world.lotse.ref(), "is_owner": False}
            return 200, {**me, "permission_version": world.permission_version}, {}
        if op == "downloadFile":
            content, mime = world.download(p["file_id"])
            return 200, content, {"Content-Type": mime}
        if op == "getMessage":
            message = world.message_or_404(p["message_id"])
            return 200, message, {"ETag": f'"{message["version"]}"'}
        if op == "getFile":
            doc = world.file(p["file_id"])
            return 200, doc, {"ETag": self.file_etag(UUID(p["file_id"]))}
        if op == "submitTranscript":
            view = world.submit_transcript(p["file_id"], body, request.headers.get("If-Match"))
            return 200, view, {"ETag": self.file_etag(UUID(p["file_id"]))}
        if op == "getThread":
            doc = world.thread_doc(p["thread_id"])
            return 200, doc, {"ETag": f'"{doc["version"]}"'}
        if op == "getChannel":
            return 200, world.channel_doc(p["channel_id"]), {"ETag": '"1"'}
        if op == "createThread":
            doc = world.create_thread(p["channel_id"], body)
            return 201, doc, {"ETag": f'"{doc["thread"]["version"]}"'}
        if op == "setThreadTitle":
            doc = world.set_title(p["thread_id"], body, request.headers.get("If-Match"))
            return 200, doc, {"ETag": f'"{doc["version"]}"'}
        if op == "reportAgentSession":
            row = world.report_session(p["thread_id"], body, request.headers.get("If-Match"))
            return 200, row, {"ETag": f'"{row["version"]}"'}
        if op == "getRun":
            run = world.run_or_404(p["run_id"])
            return 200, world.run_doc(run), {"ETag": f'"{run.version}"'}
        if op == "getRunContext":
            return 200, world.context(p["run_id"]), {}
        if op == "patchRun":
            if not isinstance(body, dict):
                raise field_error("", "required")
            if set(body) - {"base_version", "ops"}:
                raise field_error(
                    f"/{sorted(set(body) - {'base_version', 'ops'})[0]}", "unknown_member"
                )
            version = body.get("base_version")
            if not isinstance(version, int) or isinstance(version, bool):
                raise field_error(
                    "/base_version", "required" if version is None else "invalid_type"
                )
            if not body.get("ops"):
                raise field_error("/ops", "too_short")
            version = world.patch(p["run_id"], body.get("base_version"), body["ops"])
            return 200, {"version": version}, {}
        if op in ("startRun", "resumeRun"):
            doc, replay = world.start(p["run_id"], body, resume=op == "resumeRun")
            await _settle()
            return 200, doc, {"Idempotent-Replay": "true"} if replay else {}
        if op == "renewInvocationToken":
            return 200, world.renew(p["run_id"], body), {}
        if op == "awaitRunInput":
            return 200, world.await_input(p["run_id"], body), {}
        if op == "finishRun":
            return 200, world.finish(p["run_id"], body), {}
        if op == "createMessage":
            return self._create_message(p["thread_id"], body)
        if op in _UPLOAD_OPS:
            return self._upload_operation(op, p, request, body)
        if op == "updateComponent":
            component = world.update_component(
                p["component_id"], body, request.headers.get("If-Match")
            )
            return 200, component, {"ETag": f'"{component["version"]}"'}
        if op == "respondToInteraction":
            return 200, world.respond(p["interaction_id"], body), {}
        if op == "registerCommand":
            command = world.register_command(body)
            return 201, command, {"ETag": f'"{command["version"]}"'}
        if op == "updateCommand":
            command = world.update_command(p["command_id"], body, request.headers.get("If-Match"))
            return 200, command, {"ETag": f'"{command["version"]}"'}
        if op == "removeCommand":
            world.commands.pop(UUID(world._get(world.commands, p["command_id"])["id"]))
            return 204, None, {}
        if op == "listChannelCommands":
            return 200, world.channel_commands(p["channel_id"]), {}
        if op in _THREAD_OPS:
            return self._thread_operation(op, p, request, body)
        return self._answer_autocomplete(p["request_id"], body)

    def _create_message(self, thread_id: str, body: Any) -> _Answer:
        """``createMessage``; ``attachment_file_ids`` attach completed own uploads (M3-07).

        A voice message with an upload answers ``409 file_not_ready``: uploads are never scanned.
        """
        world = self.world
        uploads = []
        content = str(body.get("content_md", "")) if isinstance(body, dict) else ""
        # The server refuses a content above message.max_content_bytes (M4-47, 06).
        if len(content.encode("utf-8")) > self.uploads.limits["message_max_content_bytes"]:
            raise field_error("/content_md", "too_long")
        ids = body.get("attachment_file_ids") if isinstance(body, dict) else None
        known = isinstance(body, dict) and _known_message(world, body.get("client_msg_id"))
        if ids is not None and not known:
            meta = world.thread_meta.get(UUID(thread_id)) if _is_uuid(thread_id) else None
            if meta is None:
                raise FakeProblem("not_found")
            uploads = self.uploads.check_attachable(ids, UUID(str(meta["channel_id"])))
            if uploads and body.get("kind") == "voice":
                # The FakeServer never scans an upload: it is not final, and a voice message
                # needs a final file (12 8.1).
                raise FakeProblem("file_not_ready")
        message, created = world.create_message(thread_id, body)
        if created and uploads:
            self.uploads.attach(uploads, message)
        return (201, message, {}) if created else (200, message, {"Idempotent-Replay": "true"})

    def _upload_operation(
        self, op: str, p: dict[str, str], request: web.Request, body: Any
    ) -> _Answer:
        uploads = self.uploads
        if op == "getInstance":
            return 200, uploads.instance(self.world.instance_id), {}
        if op == "createUpload":
            return 201, uploads.create(p["channel_id"], body), {}
        if op == "uploadPart":
            digest = request.headers.get("Content-Digest")
            return 200, uploads.put_part(p["file_id"], p["part_number"], body, digest), {}
        if op == "completeUpload":
            return 200, uploads.complete(p["file_id"], body), {}
        if op == "getUpload":
            return 200, uploads.state(p["file_id"]), {}
        uploads.abort(p["file_id"])
        return 204, None, {}

    def _thread_operation(
        self, op: str, p: dict[str, str], request: web.Request, body: Any
    ) -> _Answer:
        del op  # updateMessage is the only one
        message = self.world._get(self.world.messages, p["message_id"])
        updated = self.world.extra.update_message(message, body, request.headers.get("If-Match"))
        return 200, updated, {"ETag": f'"{updated["version"]}"'}

    def _answer_autocomplete(self, request_id: str, body: Any) -> _Answer:
        try:
            key = UUID(request_id)
        except ValueError:
            raise FakeProblem("not_found") from None
        pending = self.world.autocomplete.get(key)
        if pending is None:
            raise FakeProblem("not_found")
        if self.world.now().timestamp() > pending["expires"]:
            raise FakeProblem("gone")
        if not isinstance(body, dict) or not isinstance(body.get("choices"), list):
            raise field_error("/choices", "required")
        self.world.autocomplete_answers.setdefault(key, list(body["choices"]))
        return 204, None, {}


_UPLOAD_OPS = frozenset(
    {"getInstance", "createUpload", "uploadPart", "completeUpload", "getUpload", "abortUpload"}
)


def _is_uuid(value: str) -> bool:
    try:
        UUID(value)
    except ValueError:
        return False
    return True


def _known_message(world: World, client_msg_id: Any) -> bool:
    """Whether a ``client_msg_id`` was stored before (its repeat answers the original)."""
    if not isinstance(client_msg_id, str) or not _is_uuid(client_msg_id):
        return False
    return UUID(client_msg_id) in world.by_client_msg_id


async def _settle() -> None:
    # Let the delivery runners and woken tasks run before a helper returns.
    for _ in range(20):
        await asyncio.sleep(0)
