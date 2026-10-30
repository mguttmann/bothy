# SPDX-License-Identifier: Apache-2.0
"""The FakeServer's data and run rules (private).

One world after docs/spec/07-permissions.md "Examples": owner Robin, Alex and Sam who trigger, a
person Writer with ``write`` only, the agent Lotse that operates the channel Allgemein of space
Support, and the agent Texter. The run rules
follow the server code of M2-08 to M2-10 (docs/spec/09-agents.md 4.5, 4.5.1, 5.1 and 5.3,
docs/spec/04-sync-protocol.md 11.2 and 15.2 to 15.5): fresh tokens per delivery, the start
verdicts ``superseded``, ``expired`` and ``invalid``, consumed tokens, ``already_started``,
mismatches that fail the run, renewals with ``renewed_from_jti`` and patches with
``base_version``.
"""

from __future__ import annotations

import copy
import re
from collections.abc import Callable, Mapping
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any
from uuid import UUID

import attrs

from bothy_sdk.generated import models
from bothy_sdk.ids import uuid7
from bothy_sdk.testing import _fake_fixtures as fixtures
from bothy_sdk.testing._fake_base import FakeProblem, field_error, iso
from bothy_sdk.testing._fake_events import LogEntry
from bothy_sdk.testing._fake_extra import (
    ContextFile,
    ContextMessage,
    Extras,
    file_doc,
    file_record,
    submit_transcript,
)
from bothy_sdk.testing._fake_patch import apply_ops, parse_ops, text_error
from bothy_sdk.testing._fake_tokens import TRIGGER_OBJECT, Issuer
from bothy_sdk.testing.doubles import FakeClock

ACTIVE = frozenset({"queued", "running", "waiting_input"})
ENDED = frozenset({"completed", "failed", "cancelled"})
_ACTIVE = frozenset({"queued", "running", "waiting_input"})
TOKEN_EVENTS = frozenset(
    {"agent.invocation", "agent.followup", "interaction.created", "command.invoked"}
)
_KEEP_FAILED = frozenset(
    {
        "billing_mode_mismatch",
        "tool_class_mismatch",
        "invalid_invocation_token",
        "runtime_error",
        "adapter_error",
        "runtime_not_authenticated",
    }
)
_CANCEL_REASONS = frozenset(
    {"user_cancelled", "access_revoked", "agent_disabled", "channel_read_only", "user_disabled"}
)
_TOOL_RANK = {"chat": 0, "scoped": 1}
_CONSUMER = re.compile(r"^[a-z0-9][a-z0-9_-]*$")
_COMMAND_NAME = re.compile(r"^[a-z0-9][a-z0-9_-]*$")
_EXCERPT_MAX = 500
# Members the FakeServer produces itself; a fixture contributes only members beyond these.
_RUN_MEMBERS = tuple(a.name for a in attrs.fields(models.Run) if a.name != "additional_properties")
_CONTEXT_MEMBERS = tuple(
    a.name for a in attrs.fields(models.RunContext) if a.name != "additional_properties"
)
_SUMMARY_MEMBERS = tuple(
    a.name for a in attrs.fields(models.RunSummary) if a.name != "additional_properties"
)


def tool_rank(value: str) -> int:
    """Rank of a tool class (``chat`` < ``scoped`` < everything else)."""
    return _TOOL_RANK.get(value, 2)


def consumer_ok(name: str) -> bool:
    """Whether ``name`` is a valid consumer name."""
    return bool(_CONSUMER.match(name))


@dataclass
class Person:
    """A principal of the world."""

    id: UUID
    kind: str
    handle: str
    display_name: str

    def ref(self) -> dict[str, Any]:
        """The ``PrincipalRef``."""
        return {
            "id": str(self.id),
            "kind": self.kind,
            "handle": self.handle,
            "display_name": self.display_name,
        }


@dataclass
class RunRecord:
    """A run row with the columns the rules need."""

    id: UUID
    thread_id: UUID
    triggered_by: UUID
    triggering_message_id: UUID
    queued_at: datetime
    billing_mode_expected: str
    state: str = "queued"
    state_reason: str | None = None
    version: int = 1
    invocation_jti: str = field(default_factory=lambda: str(uuid7()))
    started_jti: str | None = None
    renewed_from_jti: str | None = None
    billing_mode_used: str | None = None
    summary: str | None = None
    started_at: datetime | None = None
    finished_at: datetime | None = None
    starts: int = 0
    live: dict[str, Any] = field(default_factory=lambda: {"metrics": {}, "steps": {}})
    issued: dict[str, dict[str, Any]] = field(default_factory=dict)
    last_jti: str | None = None
    cancel_requested: bool = False
    channel_id: UUID | None = None
    trigger_kind: str = "message"
    trigger_object: str | None = None
    context: dict[str, Any] | None = None
    costs: list[dict[str, Any]] | None = None


AUTO_SESSION: Any = object()
"""`World.trigger` default: the payload carries the thread's current session, if any."""


class World:
    """Data and rules; the transport layers call into it.

    Args:
        clock: The shared FakeClock.
        timings: The server keys the FakeServer plays.
        billing_mode: Lotse's configured billing mode.
        tool_class: Lotse's configured tool class.
        append: Appends a durable event to the log and wakes the deliverers.
        costs: Accept ``RunFinish.costs`` (the server books them from M2-22 on).
    """

    def __init__(  # noqa: PLR0913 - the world's inputs are explicit
        self,
        clock: FakeClock,
        timings: Mapping[str, float],
        *,
        billing_mode: str,
        tool_class: str,
        append: Callable[[str, dict[str, Any]], LogEntry],
        costs: bool = False,
    ) -> None:
        self.costs_enabled = costs
        self.clock = clock
        self.timings = timings
        self.billing_mode = billing_mode
        self.tool_class = tool_class
        self.append = append
        self.instance_id = uuid7()
        self.sync_epoch = uuid7()
        self.space_id = uuid7()
        self.channel_id = uuid7()
        self.robin = Person(uuid7(), "user", "robin", "Robin")
        self.alex = Person(uuid7(), "user", "alex", "Alex")
        self.lotse = Person(uuid7(), "agent", "lotse", "Lotse")
        self.texter = Person(uuid7(), "agent", "texter", "Texter")
        self.sam = Person(uuid7(), "user", "sam", "Sam")
        self.writer = Person(uuid7(), "user", "writer", "Writer")
        self.system = Person(uuid7(), "system", "system", "System")
        everyone = (self.robin, self.alex, self.lotse, self.texter, self.sam, self.writer)
        self.people = {p.id: p for p in everyone}
        self.by_handle = {p.handle: p for p in (*everyone, self.system)}
        self.permission_version = 1
        self.issuer = Issuer(self.instance_id)
        self.runs: dict[UUID, RunRecord] = {}
        self.threads: dict[UUID, list[UUID]] = {}
        self.messages: dict[UUID, dict[str, Any]] = {}
        self.by_client_msg_id: dict[UUID, UUID] = {}
        self.message_bodies: list[dict[str, Any]] = []
        self.components: dict[UUID, dict[str, Any]] = {}
        self.interactions: dict[UUID, dict[str, Any]] = {}
        self.commands: dict[UUID, dict[str, Any]] = {}
        self.autocomplete: dict[UUID, dict[str, Any]] = {}
        self.autocomplete_answers: dict[UUID, list[dict[str, str]]] = {}
        self.heartbeats: list[dict[str, Any]] = []
        # Channels Lotse serves: Allgemein first; forums with a project binding join (M2-34).
        self.channels: dict[UUID, dict[str, Any]] = {self.channel_id: self._channel("")}
        # Forums another agent operates: Lotse does not serve them and gets no event about them.
        self.unserved: dict[UUID, dict[str, Any]] = {}
        self.thread_meta: dict[UUID, dict[str, Any]] = {}
        # Threads Lotse opened itself (``createThread``, a forum post of a report, 09 14).
        self.created_threads: list[UUID] = []
        self.files: dict[UUID, dict[str, Any]] = {}
        self.context_attachments: dict[UUID, list[UUID]] = {}
        self.sessions: dict[UUID, list[dict[str, Any]]] = {}
        # Trust marks of context messages, titles set, message updates and session reports (M2-33).
        self.extra = Extras(self.lotse.id)
        self.followup_claims: dict[UUID, dict[str, Any]] = {}
        self.hidden_messages: set[UUID] = set()

    # Time ---------------------------------------------------------------------------------

    def now(self) -> datetime:
        """The clock's wall time."""
        return self.clock.now()

    def now_iso(self) -> str:
        """The wall time as the server writes it."""
        return iso(self.now())

    # Documents ----------------------------------------------------------------------------

    def run_doc(self, run: RunRecord) -> dict[str, Any]:
        """The ``Run`` body as ``getRun`` answers it."""
        doc: dict[str, Any] = {
            "id": str(run.id),
            "thread_id": str(run.thread_id),
            "channel_id": str(run.channel_id or self.channel_id),
            "agent_id": str(self.lotse.id),
            "triggered_by": str(run.triggered_by),
            "trigger_kind": run.trigger_kind,
            "triggering_message_id": str(run.triggering_message_id),
            "state": run.state,
            "version": run.version,
            "billing_mode_expected": run.billing_mode_expected,
            "metrics": dict(run.live.get("metrics", {})),
            "cost": run.live.get(
                "cost", {"amount_micros": 0, "currency": "USD", "is_estimate": True}
            ),
            "steps": dict(run.live.get("steps", {})),
            "followups": copy.deepcopy(run.live.get("followups", {})),
            "queued_at": iso(run.queued_at),
        }
        for name in ("model", "status_line", "last_text_excerpt", "plan"):
            if name in run.live:
                doc[name] = run.live[name]
        optional = {
            "state_reason": run.state_reason,
            "billing_mode_used": run.billing_mode_used,
            "summary": run.summary,
            "started_at": iso(run.started_at) if run.started_at else None,
            "finished_at": iso(run.finished_at) if run.finished_at else None,
        }
        doc.update({k: v for k, v in optional.items() if v is not None})
        if run.started_at and run.finished_at:
            doc["duration_ms"] = int((run.finished_at - run.started_at).total_seconds() * 1000)
        return fixtures.build("run", doc, replace=_RUN_MEMBERS)

    def _summary(self, run: RunRecord) -> dict[str, Any]:
        doc = self.run_doc(run)
        out = {n: doc[n] for n in _SUMMARY_MEMBERS if n in doc}
        out["step_count"] = len(run.live.get("steps", {}))
        return out

    def envelope(  # noqa: PLR0913 - the members of an envelope the fake sets
        self,
        kind: str,
        payload: dict[str, Any],
        *,
        resource: dict[str, Any],
        thread_id: UUID | None,
        actor: Person | None = None,
        audience: bool = True,
        drop: tuple[str, ...] = (),
        channel_id: UUID | None = None,
    ) -> dict[str, Any]:
        """An envelope of type ``kind`` built from its fixture (without ``seq``)."""
        if channel_id is None and thread_id is not None:
            channel_id = self.thread_meta.get(thread_id, {}).get("channel_id")
        channel = channel_id or self.channel_id
        scope = {"space_id": str(self.space_id), "channel_id": str(channel)}
        if thread_id is not None:
            scope["thread_id"] = str(thread_id)
        override: dict[str, Any] = {
            "id": str(uuid7()),
            "type": kind,
            "schema_version": 1,
            "occurred_at": self.now_iso(),
            "scope": scope,
            "resource": resource,
            "permission_version": self.permission_version,
            "payload": payload,
            "actor": {"id": str(actor.id), "kind": actor.kind} if actor else {"kind": "system"},
        }
        if audience:
            override["audience"] = "self"
        doc = fixtures.build(
            kind, override, replace=("payload", "scope", "resource", "actor"), drop=drop
        )
        if not audience:
            doc.pop("audience", None)
        return doc

    def _state_changed(self, run: RunRecord) -> None:
        trigger_state = {"running": "working", "completed": "done"}.get(run.state)
        if run.state in ("failed", "cancelled"):
            trigger_state = run.state
        payload: dict[str, Any] = {
            "run": self._summary(run),
            "triggering_message_id": str(run.triggering_message_id),
        }
        if trigger_state:
            payload["trigger_state"] = trigger_state
        envelope = self.envelope(
            "run.state_changed",
            payload,
            resource={"kind": "run", "id": str(run.id), "version": run.version},
            thread_id=run.thread_id,
            audience=False,
        )
        self.append("run.state_changed", envelope)

    def transition(self, run: RunRecord, state: str, reason: str | None = None) -> None:
        """Change the state: version + 1, times, reason and a ``run.state_changed`` event."""
        now = self.now()
        run.state = state
        run.version += 1
        run.state_reason = reason
        if state == "running":
            run.starts += 1
            run.started_at = run.started_at or now
        if state in ENDED:
            run.finished_at = now
        self._state_changed(run)

    # Lookups ------------------------------------------------------------------------------

    def run_or_404(self, run_id: str) -> RunRecord:
        """The run, or ``404 not_found`` (also for a malformed ID)."""
        try:
            run = self.runs.get(UUID(run_id))
        except ValueError:
            run = None
        if run is None:
            raise FakeProblem("not_found")
        return run

    # Trigger and deliveries ---------------------------------------------------------------

    def _message(  # noqa: PLR0913 - the members of a stored message
        self,
        thread_id: UUID,
        author: Person,
        text: str,
        *,
        client_msg_id: UUID | None = None,
        run_id: UUID | None = None,
        kind: str = "text",
    ) -> dict[str, Any]:
        thread = self.threads.setdefault(thread_id, [])
        meta = self.thread_meta.setdefault(thread_id, self._thread(thread_id, self.channel_id))
        message_id = uuid7()
        message: dict[str, Any] = {
            "id": str(message_id),
            "thread_id": str(thread_id),
            "channel_id": str(meta["channel_id"]),
            "thread_seq": len(thread) + 1,
            "author": author.ref(),
            "client_msg_id": str(client_msg_id or uuid7()),
            "origin": "agent" if author.kind == "agent" else "human",
            "kind": kind,
            "content_md": text,
            "content_format": 1,
            "silent": False,
            "attachments": [],
            "mention_ids": [],
            "reactions": [],
            "pinned": False,
            "components": [],
            "version": 1,
            "created_at": self.now_iso(),
        }
        if run_id is not None:
            message["run_id"] = str(run_id)
        thread.append(message_id)
        self.messages[message_id] = message
        meta["last_message_seq"] = message["thread_seq"]
        meta["message_count"] = len(thread)
        meta["last_activity_at"] = message["created_at"]
        return message

    def store_file(self, f: ContextFile) -> None:
        """Store a context file under its own ID (``downloadFile`` and attachments serve it)."""
        self.files[f.file_id] = file_record(
            f.content, name=f.name, mime=f.mime_type, status=f.scan_status, transcript=f.transcript
        )

    def post_context(self, thread_id: UUID, item: ContextMessage) -> UUID:
        """Store a message with its trust mark and files in a thread (for ``getRunContext``)."""
        author = self.by_handle[item.author]
        message = self._message(thread_id, author, item.text)
        message["origin"] = item.origin
        for f in item.files:
            self.store_file(f)
        self.context_attachments[UUID(message["id"])] = [f.file_id for f in item.files]
        self.extra.message_meta[UUID(message["id"])] = {
            "trust": item.trust,
            "role": item.role,
            "already_delivered": item.already_delivered,
        }
        return UUID(message["id"])

    def trigger(  # noqa: PLR0913 - the parts of a trigger a scenario chooses
        self,
        text: str,
        *,
        channel_id: UUID | None = None,
        author: Person | str | None = None,
        trigger_kind: str = "message",
        attachments: tuple[UUID, ...] = (),
        thread_id: UUID | None = None,
        context: Mapping[str, Any] | None = None,
        history: tuple[ContextMessage, ...] = (),
        files: tuple[ContextFile, ...] = (),
        session_mode: str | object | None = AUTO_SESSION,
        command: tuple[dict[str, Any], Mapping[str, str]] | None = None,
    ) -> RunRecord:
        """A message (in a new thread or ``thread_id``), a queued run and its ``agent.invocation``.

        ``trigger_kind = interaction`` names an interaction instead of the message in the
        payload and the token (a run an interaction opens, M2-15). ``history`` are messages
        posted in the thread before the trigger with their trust marks, ``files`` the trigger's
        own attachments. ``session_mode`` left at `AUTO_SESSION` sends the thread's current
        session (``resume`` when started, else ``new``); ``None`` makes a guest run without a
        session (09 3.6); a mode sends that mode with the current session, if any (09 9.1).
        """
        person = self.by_handle[author] if isinstance(author, str) else (author or self.alex)
        channel = channel_id or self.channel_id
        # An unserved channel is a guest run's (09 3.6): Lotse is mentioned, not the operator.
        if channel not in self.channels and channel not in self.unserved:
            raise KeyError("no such channel")
        if thread_id is None:
            thread_id = uuid7()
        if thread_id not in self.thread_meta:
            self.thread_meta[thread_id] = self._thread(thread_id, channel, person)
        channel = self.thread_meta[thread_id]["channel_id"]
        for item in history:
            self.post_context(thread_id, item)
        message = self._message(thread_id, person, text)
        for f in files:
            self.store_file(f)
        self.context_attachments[UUID(message["id"])] = [
            *attachments,
            *(f.file_id for f in files),
        ]
        obj = message["id"] if trigger_kind == "message" else str(uuid7())
        invocation = obj
        if command is not None:
            obj = str(command[0]["id"])  # the token names the command (09 10.4)
        run = RunRecord(
            id=uuid7(),
            thread_id=thread_id,
            triggered_by=person.id,
            triggering_message_id=UUID(message["id"]),
            queued_at=self.now(),
            billing_mode_expected=self.billing_mode,
            channel_id=channel,
            trigger_kind=trigger_kind,
            trigger_object=obj,
        )
        if context is not None:
            run.context = {"context_floor_seq": 0, "thread_context": [], **context}
        self.runs[run.id] = run
        payload: dict[str, Any] = {
            "run_id": str(run.id),
            "thread_id": str(thread_id),
            "channel_id": str(channel),
            "space_id": str(self.space_id),
            "trigger_kind": trigger_kind,
            "triggered_by": person.ref(),
            "billing_mode": self.billing_mode,
            "settings_version": (self.channels.get(channel) or self.unserved[channel])["behavior"][
                "settings_version"
            ],
        }
        payload[
            TRIGGER_OBJECT[trigger_kind] if trigger_kind != "command" else "command_invocation_id"
        ] = invocation
        if command is not None:
            definition, values = command
            payload["command_id"] = definition["id"]
            payload["command_name"] = definition["name"]
            payload["options"] = [{"name": k, "value": v} for k, v in values.items()]
        session = self.current_session(thread_id)
        if session_mode is AUTO_SESSION:
            if session is not None:
                payload["agent_session"] = dict(session)
                payload["session_mode"] = "resume" if session["started"] else "new"
        elif session_mode is not None:
            payload["session_mode"] = session_mode
            if session is not None:
                payload["agent_session"] = dict(session)
        envelope = self.envelope(
            "agent.invocation",
            payload,
            resource={"kind": "run", "id": str(run.id)},
            thread_id=thread_id,
            actor=person,
        )
        self.append("agent.invocation", envelope)
        return run

    def followup(
        self,
        run_id: UUID,
        text: str,
        *,
        author: Person | None = None,
        claims: Mapping[str, Any] | None = None,
    ) -> UUID:
        """A follow-up message in the run's thread, its row ``queued`` and ``agent.followup``.

        ``claims`` replaces claims of the token of every delivery of this event (M2-35), for
        example ``billing_mode`` or ``sub``; the token stays correctly signed.
        """
        run = self.runs[run_id]
        person = author or self.alex
        message = self._message(run.thread_id, person, text)
        rows = run.live.setdefault("followups", {})
        rows[message["id"]] = {"message_id": message["id"], "state": "queued"}
        if claims:
            self.followup_claims[UUID(message["id"])] = dict(claims)
        payload = {
            "run_id": str(run.id),
            "thread_id": str(run.thread_id),
            "message_id": message["id"],
        }
        envelope = self.envelope(
            "agent.followup",
            payload,
            resource={"kind": "run", "id": str(run.id)},
            thread_id=run.thread_id,
            actor=person,
        )
        self.append("agent.followup", envelope)
        return UUID(message["id"])

    def click(
        self, run_id: UUID, custom_id: str, values: Mapping[str, Any], author: str
    ) -> UUID | None:
        """A person operates a component under the single-use rule of 09 11.3.

        The first interaction with a ``single_use`` component disables every single-use
        component of its message in the same step; a later click answers ``409
        component_used`` (``None`` here) and delivers nothing.
        """
        component = next(
            (
                c
                for c in self.components.values()
                if c["custom_id"] == custom_id and c.get("run_id") == str(run_id)
            ),
            None,
        )
        if component is None:
            raise KeyError("no such component")
        if component["state"] != "active":
            return None
        if component["single_use"]:
            for other in self.components.values():
                if other["message_id"] == component["message_id"] and other["single_use"]:
                    other["state"] = "disabled"
                    other["version"] += 1
        return self.interaction(run_id, custom_id, values, author=author)

    def interaction(
        self, run_id: UUID, custom_id: str, values: Mapping[str, Any], *, author: str = "alex"
    ) -> UUID:
        """A person operates a component of the run: ``interaction.created``."""
        person = self.by_handle[author]
        run = self.runs[run_id]
        component = next(
            (
                c
                for c in self.components.values()
                if c["custom_id"] == custom_id and c.get("run_id") == str(run_id)
            ),
            None,
        )
        if component is None:
            message = self._message(
                run.thread_id, self.lotse, "Approve?", run_id=run_id, kind="card"
            )
            component = self._component(
                message, {"custom_id": custom_id, "type": "button", "definition": {}}, 0
            )
        interaction = {
            "id": str(uuid7()),
            "component_id": component["id"],
            "message_id": component["message_id"],
            "principal_id": str(person.id),
            "client_interaction_id": str(uuid7()),
            "custom_id": custom_id,
            "values": dict(values),
            "state": "dispatched",
            "thread_id": str(run.thread_id),
            "run_id": str(run.id),
            "created_at": self.now_iso(),
        }
        self.interactions[UUID(interaction["id"])] = interaction
        envelope = self.envelope(
            "interaction.created",
            {"interaction": interaction},
            resource={"kind": "interaction", "id": interaction["id"]},
            thread_id=run.thread_id,
            actor=person,
        )
        self.append("interaction.created", envelope)
        return UUID(interaction["id"])

    def command_invoked(
        self,
        command: dict[str, Any],
        run_id: UUID | None,
        thread_id: UUID | None,
        options: Mapping[str, str] | None = None,
    ) -> UUID:
        """Alex runs a command of Lotse: ``command.invoked``."""
        if run_id is not None:
            thread_id = self.runs[run_id].thread_id
        invocation_id = uuid7()
        channel = self.channel_id
        if thread_id is not None and thread_id in self.thread_meta:
            channel = self.thread_meta[thread_id]["channel_id"]
        payload: dict[str, Any] = {
            "invocation_id": str(invocation_id),
            "command_id": command["id"],
            "command_name": command["name"],
            "control": command["control"],
            "channel_id": str(channel),
            "invoked_by": self.alex.ref(),
            "options": [{"name": k, "value": v} for k, v in (options or {}).items()],
        }
        if thread_id is not None:
            payload["thread_id"] = str(thread_id)
        if run_id is not None:
            payload["run_id"] = str(run_id)
        envelope = self.envelope(
            "command.invoked",
            payload,
            resource={"kind": "command", "id": command["id"]},
            thread_id=thread_id,
            actor=self.alex,
        )
        self.append("command.invoked", envelope)
        return invocation_id

    def _claims(self, *, sub: UUID, kind: str, obj: str, thread_id: UUID | None) -> dict[str, Any]:
        iat = int(self.now().timestamp())
        channel = self.channel_id
        if thread_id is not None and thread_id in self.thread_meta:
            channel = self.thread_meta[thread_id]["channel_id"]
        claims: dict[str, Any] = {
            "aud": str(self.lotse.id),
            "sub": str(sub),
            "iat": iat,
            "exp": iat + int(self.timings["agent.invocation_token_ttl"]),
            "trigger_kind": kind,
            TRIGGER_OBJECT[kind]: obj,
            "thread_id": str(thread_id) if thread_id else str(self.channel_id),
            "channel_id": str(channel),
            "space_id": str(self.space_id),
            "level": "trigger",
            "billing_mode": self.billing_mode,
            "tool_class": self.tool_class,
            "permission_version": self.permission_version,
        }
        return claims

    def invocation_claims(self, run: RunRecord) -> dict[str, Any]:
        """The claims an ``agent.invocation`` of the run would carry now (not recorded).

        Args:
            run: The run.

        Returns:
            Full claims with ``iss`` and a fresh ``jti``.
        """
        claims = self._claims(
            sub=run.triggered_by,
            kind=run.trigger_kind,
            obj=run.trigger_object or str(run.triggering_message_id),
            thread_id=run.thread_id,
        )
        claims["run_id"] = str(run.id)
        return {"iss": str(self.instance_id), "jti": str(uuid7()), **claims}

    def deliver(self, entry: LogEntry) -> dict[str, Any] | None:
        """The delivery hook: decide, issue a fresh token and record it as the server does.

        Returns:
            The envelope with its delivery fields, or ``None`` when the event is skipped.
        """
        envelope = dict(entry.envelope)
        if entry.type not in TOKEN_EVENTS:
            return envelope
        payload = envelope["payload"]
        run_id = payload.get("run_id") or payload.get("interaction", {}).get("run_id")
        run = self.runs.get(UUID(run_id)) if run_id else None
        if run is not None and run.state in ENDED:
            return None
        record = run is not None and (
            (entry.type == "agent.invocation" and run.state == "queued")
            or (
                entry.type in ("agent.followup", "interaction.created")
                and run.state == "waiting_input"
            )
        )
        if entry.type == "agent.invocation":
            kind = payload["trigger_kind"]
            obj = run.trigger_object if run and run.trigger_object else payload.get("message_id")
            sub = run.triggered_by if run else self.alex.id
        elif entry.type == "agent.followup":
            author = self.messages.get(UUID(payload["message_id"]), {}).get("author", {})
            kind, obj = "followup", payload["message_id"]
            sub = UUID(author["id"]) if "id" in author else self.alex.id
        elif entry.type == "interaction.created":
            interaction = payload["interaction"]
            kind, obj, sub = "interaction", interaction["id"], UUID(interaction["principal_id"])
        else:
            kind, obj, sub = "command", payload["command_id"], UUID(payload["invoked_by"]["id"])
        thread = (
            UUID(payload["thread_id"])
            if "thread_id" in payload
            else (run.thread_id if run else None)
        )
        claims = self._claims(sub=sub, kind=kind, obj=obj, thread_id=thread)
        if run is not None:
            claims["run_id"] = str(run.id)
        if entry.type == "agent.followup":
            claims.update(self.followup_claims.get(UUID(payload["message_id"]), {}))
        token, full = self.issuer.issue(claims)
        if run is not None:
            run.issued[full["jti"]] = full
            run.last_jti = full["jti"]
            changed = run.billing_mode_expected != self.billing_mode
            run.billing_mode_expected = self.billing_mode
            if record:
                run.invocation_jti = full["jti"]
                run.renewed_from_jti = None
            if record or changed:
                run.version += 1
        envelope["invocation_token"] = token
        return envelope

    def autocomplete_event(
        self, command: dict[str, Any], value: str, expires_in: float
    ) -> dict[str, Any]:
        """An ephemeral ``command.autocomplete_requested`` with its token (no ``seq``)."""
        request_id = uuid7()
        expires = self.now().timestamp() + expires_in
        payload = {
            "request_id": str(request_id),
            "command_id": command["id"],
            "channel_id": str(self.channel_id),
            "user_id": str(self.alex.id),
            "option": "target",
            "value": value,
            "options": [],
            "expires_at": iso(datetime.fromtimestamp(expires, tz=self.now().tzinfo)),
        }
        envelope = self.envelope(
            "command.autocomplete_requested",
            payload,
            resource={"kind": "command", "id": command["id"]},
            thread_id=None,
            actor=self.alex,
            drop=("seq", "permission_version"),
        )
        claims = self._claims(
            sub=self.alex.id, kind="autocomplete", obj=command["id"], thread_id=None
        )
        envelope["invocation_token"], _ = self.issuer.issue(claims)
        self.autocomplete[request_id] = {"expires": expires, "payload": payload}
        return envelope

    # Invocation-Token header (SR9) --------------------------------------------------------

    def bind(self, values: list[str], operation_id: str) -> None:
        """Check the ``Invocation-Token`` header of a request.

        Raises:
            FakeProblem: ``403 invocation_token_invalid`` with ``invalid`` or ``run_inactive``.
        """
        if not values:
            return
        invalid = FakeProblem("invocation_token_invalid", reason="invalid")
        if len(values) != 1 or not values[0]:
            raise invalid
        checked = self.issuer.check(
            values[0],
            aud=self.lotse.id,
            now=self.now(),
            leeway=self.timings["auth.clock_leeway"],
            ignore_expiry=True,
        )
        claims = checked.claims
        if checked.verdict != "valid" or claims is None or "run_id" not in claims:
            raise invalid
        run = self.runs.get(UUID(claims["run_id"]))
        if run is None:
            raise invalid
        # 07 SR9: only message and command tokens name the run's triggering person; an
        # interaction or follow-up token names the person who acted.
        triggering = claims.get("trigger_kind") in ("message", "command")
        if triggering and claims["sub"] != str(run.triggered_by):
            raise invalid
        if claims["thread_id"] != str(run.thread_id):
            raise invalid
        if run.state in ENDED and operation_id not in (
            "startRun",
            "resumeRun",
            "renewInvocationToken",
            # A repeated final state answers 200 (M2-09 note 6), also for a cancelled queued run.
            "finishRun",
        ):
            raise FakeProblem("invocation_token_invalid", reason="run_inactive")

    # Run operations -----------------------------------------------------------------------

    def _check_token(self, run: RunRecord, token: object) -> dict[str, Any]:
        if not isinstance(token, str) or not token:
            raise FakeProblem("invocation_token_invalid", reason="invalid")
        checked = self.issuer.check(
            token,
            aud=self.lotse.id,
            now=self.now(),
            leeway=self.timings["auth.clock_leeway"],
            run_id=run.id,
            ignore_expiry=True,
        )
        if checked.verdict != "valid" or checked.claims is None:
            raise FakeProblem("invocation_token_invalid", reason="invalid")
        return checked.claims

    def start(self, run_id: str, body: Any, *, resume: bool) -> tuple[dict[str, Any], bool]:
        """``startRun`` or ``resumeRun``.

        Returns:
            The Run body and whether the answer is an idempotent replay.
        """
        _closed(
            body,
            required=("invocation_token", "billing_mode_used", "tool_class_used"),
            optional=("external_session_ref", "model", "billing_evidence"),
            enums={
                "billing_mode_used": ("personal", "organization"),
                "tool_class_used": ("chat", "scoped", "host"),
            },
        )
        run = self.run_or_404(run_id)
        claims = self._check_token(run, body["invocation_token"])
        jti = claims["jti"]
        if run.started_jti is not None and jti == run.started_jti:
            if run.state == "running":
                return self.run_doc(run), True
            if run.state == "waiting_input":
                raise FakeProblem("invocation_token_invalid", reason="superseded")
        if run.state not in ("queued", "waiting_input"):
            raise FakeProblem("already_started", detail="the run has already started or ended")
        if jti != run.invocation_jti:
            raise FakeProblem("invocation_token_invalid", reason="superseded")
        if self.now().timestamp() > claims["exp"] + self.timings["auth.clock_leeway"]:
            raise FakeProblem("invocation_token_invalid", reason="expired")
        if (run.state == "waiting_input") != resume:
            raise FakeProblem("invalid_state_transition")
        evidence = body.get("billing_evidence") or {}
        mismatch = None
        if (
            evidence.get("key_type") == "unknown"
            or body["billing_mode_used"] != run.billing_mode_expected
        ):
            mismatch = "billing_mode_mismatch"
        elif tool_rank(body["tool_class_used"]) > tool_rank(claims["tool_class"]):
            mismatch = "tool_class_mismatch"
        if mismatch:
            self.transition(run, "failed", mismatch)
            raise FakeProblem(mismatch, reason=mismatch)
        run.started_jti = jti
        run.renewed_from_jti = None
        run.billing_mode_used = body["billing_mode_used"]
        ref = body.get("external_session_ref")
        if isinstance(ref, str) and ref and self.current_session(run.thread_id) is None:
            # M2-12 note 10: a start that names a reference creates the session row (version 1).
            self.session_for(run.thread_id, ref, started=False, model=body.get("model"))
        if body.get("model"):
            run.live["model"] = body["model"]
        self.transition(run, "running")
        return self.run_doc(run), False

    def renew(self, run_id: str, body: Any) -> dict[str, Any]:
        """``renewInvocationToken``: a new token for a queued or waiting run."""
        _closed(body, required=("invocation_token",))
        run = self.run_or_404(run_id)
        claims = self._check_token(run, body["invocation_token"])
        jti = claims["jti"]
        if run.state not in ("queued", "waiting_input"):
            raise FakeProblem("already_started", detail="the run has already started or ended")
        known = jti == run.invocation_jti or (
            run.renewed_from_jti is not None and jti == run.renewed_from_jti
        )
        if not known or run.started_jti == jti:
            raise FakeProblem("invocation_token_invalid", reason="superseded")
        if claims["trigger_kind"] not in ("message", "followup"):
            raise FakeProblem("internal_error")
        obj = TRIGGER_OBJECT[claims["trigger_kind"]]
        fresh = self._claims(
            sub=UUID(claims["sub"]),
            kind=claims["trigger_kind"],
            obj=claims[obj],
            thread_id=UUID(claims["thread_id"]),
        )
        fresh["run_id"] = str(run.id)
        token, full = self.issuer.issue(fresh)
        if jti == run.invocation_jti:
            run.renewed_from_jti = jti
        run.invocation_jti = full["jti"]
        run.billing_mode_expected = self.billing_mode
        run.issued[full["jti"]] = full
        run.last_jti = full["jti"]
        run.version += 1
        expires = datetime.fromtimestamp(full["exp"], tz=self.now().tzinfo)
        return {"invocation_token": token, "expires_at": iso(expires)}

    def await_input(self, run_id: str, body: Any) -> dict[str, Any]:
        """``awaitRunInput``: ``running`` to ``waiting_input``."""
        _closed(body, required=("status_line",))
        if text_error(body["status_line"]):
            raise field_error("/status_line", "invalid")
        run = self.run_or_404(run_id)
        if run.state == "waiting_input":
            return self.run_doc(run)
        if run.state != "running":
            raise FakeProblem("invalid_state_transition")
        if body["status_line"]:
            run.live["status_line"] = body["status_line"]
        self.transition(run, "waiting_input")
        return self.run_doc(run)

    def finish(self, run_id: str, body: Any) -> dict[str, Any]:
        """``finishRun`` with the server's reason rules."""
        _closed(
            body,
            required=("state",),
            optional=("reason", "summary", "metrics", "costs"),
            enums={"state": ("completed", "failed", "cancelled")},
        )
        if "costs" in body and not self.costs_enabled:
            raise field_error("/costs", "not_enabled")
        state, reason = body["state"], body.get("reason") or None
        if state in ("failed", "cancelled") and not reason:
            raise field_error("/reason", "required")
        if state == "cancelled" and reason not in _CANCEL_REASONS:
            raise field_error("/reason", "invalid_enum")
        if "summary" in body and text_error(body["summary"]):
            raise field_error("/summary", "invalid")
        run = self.run_or_404(run_id)
        if run.state == state:
            return self.run_doc(run)
        if run.state in ENDED or (state == "completed" and run.state != "running"):
            raise FakeProblem("invalid_state_transition")
        if state == "completed":
            reason = None
        elif state == "failed" and reason not in _KEEP_FAILED:
            self._reason_step(run, str(reason))
            reason = "adapter_error"
        if body.get("summary"):
            run.summary = body["summary"]
        if "metrics" in body:
            run.live["metrics"] = dict(body["metrics"])
        if "costs" in body:
            run.costs = list(body["costs"])
        self.transition(run, state, reason)
        return self.run_doc(run)

    def _reason_step(self, run: RunRecord, original: str) -> None:
        steps = run.live.setdefault("steps", {})
        step_id = str(uuid7())
        now = self.now_iso()
        steps[step_id] = {
            "id": step_id,
            "ordinal": max((s["ordinal"] for s in steps.values()), default=0) + 1,
            "kind": "status",
            "name": "adapter_reason",
            "title": "",
            "status": "failed",
            "detail": {"original_reason": original} if not text_error(original) else {},
            "started_at": now,
            "finished_at": now,
        }

    def patch(self, run_id: str, base_version: Any, ops: Any) -> int:
        """``patchRun`` and the gateway op ``run.update``.

        Returns:
            The new version.
        """
        run = self.run_or_404(run_id)
        parsed = parse_ops(ops)
        if run.state not in ACTIVE:
            raise FakeProblem("conflict", reason="run_not_active", detail="the run is not active")
        if base_version != run.version:
            raise FakeProblem(
                "base_version_conflict",
                detail="base_version is not the current version of the run",
                current=self.run_doc(run),
            )
        run.live = apply_ops(run.live, parsed, now_iso=self.now_iso, excerpt_max=_EXCERPT_MAX)
        run.version += 1
        return run.version

    def cancel(self, run: RunRecord, reason: str) -> bool:
        """The server side of ``cancelRun``.

        Returns:
            Whether the run still waits for the adapter's ``cancelled`` (grace timer).

        Raises:
            ValueError: The run has completed or failed.
        """
        if run.state == "cancelled":
            return False
        if run.state in ("completed", "failed"):
            raise ValueError("the run has ended")
        payload: dict[str, Any] = {
            "run_id": str(run.id),
            "thread_id": str(run.thread_id),
            "reason": reason,
        }
        if reason == "user_cancelled":
            payload["requested_by"] = str(self.alex.id)
        envelope = self.envelope(
            "run.cancel_requested",
            payload,
            resource={"kind": "run", "id": str(run.id)},
            thread_id=run.thread_id,
            actor=self.alex,
        )
        self.append("run.cancel_requested", envelope)
        if run.state == "queued":
            self.transition(run, "cancelled", reason)
            return False
        run.cancel_requested = True
        return True

    # Context, channels, heartbeat ---------------------------------------------------------

    def context(self, run_id: str) -> dict[str, Any]:
        """``getRunContext``: the trigger and the other messages of the thread as marked items.

        ``thread_context`` holds every other message of the thread, also the follow-ups after the
        trigger, so an adapter finds a follow-up's content by its ``message_id``.
        """
        run = self.run_or_404(run_id)
        items = [self._context_item(self.messages[m], run) for m in self.threads[run.thread_id]]
        if run.context is not None:
            doc = {**run.context, "run_id": str(run.id)}
            if "trigger_message" not in doc:
                trigger_id = str(run.triggering_message_id)
                doc["trigger_message"] = next(i for i in items if i["message_id"] == trigger_id)
            return doc
        trigger = next(i for i in items if i["message_id"] == str(run.triggering_message_id))
        doc = {
            "run_id": str(run.id),
            "trigger_message": trigger,
            "thread_context": [i for i in items if i is not trigger],
            "channel_context": [],
            "context_floor_seq": 0,
        }
        return fixtures.build("run_context", doc, replace=_CONTEXT_MEMBERS)

    def _context_item(self, message: dict[str, Any], run: RunRecord) -> dict[str, Any]:
        own = message["author"]["id"] == str(self.lotse.id)
        trigger = message["id"] == str(run.triggering_message_id)
        meta = self.extra.message_meta.get(UUID(message["id"]), {})
        item = {
            "message_id": message["id"],
            "thread_seq": message["thread_seq"],
            "created_at": message["created_at"],
            "author": message["author"],
            "role": meta.get("role") or ("assistant" if own else "user"),
            "origin": message["origin"],
            "kind": message["kind"],
            "channel_id": message["channel_id"],
            "content_md": message["content_md"],
            "attachments": [
                self.attachment(f) for f in self.context_attachments.get(UUID(message["id"]), [])
            ],
            "trust": meta.get("trust")
            or ("trigger" if trigger else ("self" if own else "authorized")),
        }
        if trigger:
            item["trust"] = "trigger"
        if meta.get("already_delivered"):
            item["already_delivered"] = True
        return item

    @property
    def settings_version(self) -> int:
        """``settings_version`` of Allgemein."""
        version: int = self.channels[self.channel_id]["behavior"]["settings_version"]
        return version

    @staticmethod
    def _channel(prompt: str, binding: dict[str, Any] | None = None) -> dict[str, Any]:
        behavior: dict[str, Any] = {
            "require_mention": True,
            "thread_require_mention": False,
            "free_response": False,
            "auto_thread": True,
            "allow_mentioned_agents": True,
            "history_backfill_limit": 50,
            "prompt": prompt,
            "skills": [],
            "settings_version": 1,
        }
        if binding is not None:
            behavior["project_binding"] = binding
        return {"behavior": behavior, "kind": "text"}

    def channel_config(self, channel_id: UUID | None = None) -> dict[str, Any]:
        """The ``AgentChannelConfig`` of a served channel (Allgemein when omitted)."""
        key = channel_id or self.channel_id
        return {
            "channel_id": str(key),
            "space_id": str(self.space_id),
            "behavior": dict(self.channels[key]["behavior"]),
        }

    def channel_or_404(self, channel_id: str) -> UUID:
        """A served channel's ID, or ``404 not_found``."""
        try:
            key = UUID(channel_id)
        except ValueError:
            raise FakeProblem("not_found") from None
        if key not in self.channels:
            raise FakeProblem("not_found")
        return key

    def add_forum(
        self,
        workspace_ref: str | None,
        *,
        model: str | None = None,
        allowed_tools: list[str] | None = None,
        prompt: str = "",
        served: bool = True,
    ) -> UUID:
        """A forum Lotse serves, with a project binding unless ``workspace_ref`` is ``None``."""
        channel_id = uuid7()
        binding = None
        if workspace_ref is not None:
            binding = self._binding(
                workspace_ref, model, allowed_tools, version=1, provision=None, url=None
            )
        channel = self._channel(prompt, binding)
        channel["kind"] = "forum"
        (self.channels if served else self.unserved)[channel_id] = channel
        return channel_id

    @staticmethod
    def _binding(  # noqa: PLR0913 - the members of a binding
        workspace_ref: str,
        model: str | None,
        allowed_tools: list[str] | None,
        *,
        version: int,
        provision: str | None,
        url: str | None,
    ) -> dict[str, Any]:
        config: dict[str, Any] = {}
        if model is not None:
            config["model"] = model
        if allowed_tools is not None:
            config["allowed_tools"] = list(allowed_tools)
        binding: dict[str, Any] = {
            "workspace_ref": workspace_ref,
            "runner_config": config,
            "ready": False,
            "version": version,
        }
        if provision is not None:
            binding["provision"] = provision
        if url is not None:
            binding["repository_url"] = url
        return binding

    def update_forum(self, channel_id: UUID, **changes: Any) -> None:
        """Change a forum and append ``agent.config_changed`` when Lotse serves it.

        Changes: ``workspace_ref``, ``model``, ``allowed_tools``, ``provision``,
        ``repository_url``, ``prompt``; ``settings_version`` grows by one. A forum of another
        operator changes without an event to Lotse, as on the server (the event goes to the
        effective operator only).
        """
        served = channel_id in self.channels
        behavior = (self.channels if served else self.unserved)[channel_id]["behavior"]
        if "prompt" in changes:
            behavior["prompt"] = changes.pop("prompt")
        old = behavior.get("project_binding")
        if changes:
            config = dict(old["runner_config"]) if old else {}
            ref = changes.get("workspace_ref", old["workspace_ref"] if old else "")
            behavior["project_binding"] = self._binding(
                ref,
                changes.get("model", config.get("model")),
                changes.get("allowed_tools", config.get("allowed_tools")),
                version=(old["version"] + 1) if old else 1,
                provision=changes.get("provision", old.get("provision") if old else None),
                url=changes.get("repository_url", old.get("repository_url") if old else None),
            )
        behavior["settings_version"] += 1
        if served:
            self._config_changed(channel_id, behavior["settings_version"])

    def unbind_forum(self, channel_id: UUID) -> None:
        """Delete the project binding of a served forum and append ``agent.config_changed``."""
        behavior = self.channels[channel_id]["behavior"]
        behavior.pop("project_binding", None)
        behavior["settings_version"] += 1
        self._config_changed(channel_id, behavior["settings_version"])

    def set_operator(self, channel_id: UUID, *, served: bool, announce: bool = True) -> None:
        """Make Lotse the operator of a forum (``served``) or another agent; ``access.changed``.

        As on the server an operator change writes no ``agent.config_changed``, only the
        personal ``access.changed`` with the channel under ``added`` or ``removed`` (04 8.2),
        and none at all without ``announce`` (no change of Lotse's visible targets).
        """
        source, target = (
            (self.unserved, self.channels) if served else (self.channels, self.unserved)
        )
        target[channel_id] = source.pop(channel_id)
        if not announce:
            return
        self.permission_version += 1
        entry = {"kind": "channel", "id": str(channel_id)}
        payload: dict[str, Any] = {
            "permission_version": self.permission_version,
            "added": [{**entry, "level": "trigger"}] if served else [],
            "removed": [] if served else [entry],
            "changed": [],
        }
        envelope = self.envelope(
            "access.changed",
            payload,
            resource={"kind": "principal", "id": str(self.lotse.id)},
            thread_id=None,
            actor=self.robin,
        )
        envelope["scope"] = {}
        self.append("access.changed", envelope)

    def remove_channel(self, channel_id: UUID) -> None:
        """Lotse stops serving the channel: ``agent.config_changed``, then ``404``."""
        version = self.channels.pop(channel_id)["behavior"]["settings_version"] + 1
        self._config_changed(channel_id, version)

    def _config_changed(self, channel_id: UUID, version: int) -> None:
        envelope = self.envelope(
            "agent.config_changed",
            {"channel_id": str(channel_id), "settings_version": version},
            resource={"kind": "channel", "id": str(channel_id)},
            thread_id=None,
            channel_id=channel_id,
            actor=self.robin,
        )
        self.append("agent.config_changed", envelope)

    # Threads, files, sessions (M2-34) ---------------------------------------------------------

    def _thread(
        self, thread_id: UUID, channel_id: UUID, author: Person | None = None
    ) -> dict[str, Any]:
        return {
            "id": str(thread_id),
            "channel_id": channel_id,
            "kind": "thread",
            "title": "",
            "title_source": "auto",
            "state": "open",
            "last_message_seq": 0,
            "last_activity_at": self.now_iso(),
            "message_count": 0,
            "created_by": str((author or self.alex).id),
            "created_at": self.now_iso(),
            "version": 1,
            "tag_ids": [],
            "operator_agent_id": str(self.lotse.id),
        }

    def thread_doc(self, thread_id: str) -> dict[str, Any]:
        """``getThread``: the ``Thread``, or ``404``."""
        meta = self._thread_or_404(thread_id)
        return {**meta, "channel_id": str(meta["channel_id"])}

    def channel_doc(self, channel_id: str) -> dict[str, Any]:
        """``getChannel`` of a served channel (a ``ChannelView`` with Lotse's level ``write``).

        A text channel names its main thread; a channel of `add_forum` is a forum.
        """
        key = self.channel_or_404(channel_id)
        channel = self.channels[key]
        doc: dict[str, Any] = {
            "id": str(key),
            "kind": channel["kind"],
            "name": "allgemein" if key == self.channel_id else "forum",
            "topic": "",
            "sort_key": "a",
            "operator": {"mode": "agent", "agent_id": str(self.lotse.id)},
            "read_only": False,
            "project_bound": "project_binding" in channel["behavior"],
            "version": 1,
            "settings_version": channel["behavior"]["settings_version"],
            "created_at": self.now_iso(),
            "space_id": str(self.space_id),
        }
        if channel["kind"] == "text":
            doc["main_thread_id"] = str(self.main_thread(key))
        return {"channel": doc, "level": "write", "capabilities": []}

    def main_thread(self, channel_id: UUID) -> UUID:
        """The main thread of a text channel (created on first use)."""
        channel = self.channels[channel_id]
        if "main_thread_id" not in channel:
            thread_id = uuid7()
            meta = self._thread(thread_id, channel_id)
            meta["kind"] = "main"
            self.thread_meta[thread_id] = meta
            self.threads[thread_id] = []
            channel["main_thread_id"] = thread_id
        main: UUID = channel["main_thread_id"]
        return main

    def create_thread(self, channel_id: str, body: Any) -> dict[str, Any]:
        """``createThread`` by Lotse: a forum post with its first message (``ThreadCreated``)."""
        key = self.channel_or_404(channel_id)
        _closed(body, optional=("title", "parent_message_id", "tag_ids", "first_message"))
        if self.channels[key]["kind"] != "forum":
            raise field_error("/parent_message_id", "required")
        first = body.get("first_message")
        if not isinstance(first, dict):
            raise field_error("/first_message", "required")
        thread_id = uuid7()
        meta = self._thread(thread_id, key, self.lotse)
        meta["title"] = str(body.get("title", ""))
        self.thread_meta[thread_id] = meta
        self.threads[thread_id] = []
        message, _ = self.create_message(str(thread_id), first)
        self.created_threads.append(thread_id)
        return {"thread": self.thread_doc(str(thread_id)), "first_message": message}

    def _thread_or_404(self, thread_id: str) -> dict[str, Any]:
        try:
            meta = self.thread_meta.get(UUID(thread_id))
        except ValueError:
            meta = None
        if meta is None:
            raise FakeProblem("not_found")
        return meta

    def set_title(self, thread_id: str, body: Any, if_match: str | None) -> dict[str, Any]:
        """``setThreadTitle`` with ``If-Match``; a manual title is never replaced."""
        meta = self._thread_or_404(thread_id)
        _closed(body, required=("title",))
        _check_if_match(if_match, meta["version"])
        if meta["title_source"] == "manual":
            raise FakeProblem("title_manual")
        if not isinstance(body["title"], str) or text_error(body["title"]):
            raise field_error("/title", "invalid")
        meta["title"] = body["title"]
        meta["version"] += 1
        self.extra.titles_set.append({"thread_id": meta["id"], "title": meta["title"]})
        return self.thread_doc(thread_id)

    def add_file(
        self, content: bytes, *, name: str, mime: str, status: str, transcript: str | None = None
    ) -> UUID:
        """A stored file with its scan status (``clean``, ``skipped``, ``pending``, ``blocked``)."""
        file_id = uuid7()
        self.files[file_id] = file_record(
            content, name=name, mime=mime, status=status, transcript=transcript
        )
        return file_id

    def attachment(self, file_id: UUID) -> dict[str, Any]:
        """A ``RunContextAttachment`` of a stored file."""
        f = self.files[file_id]
        return {
            "file_id": str(file_id),
            "original_name": f["name"],
            "mime_type": f["mime"],
            "size_bytes": len(f["content"]),
            "scan_status": f["status"],
            **({"transcript": f["transcript"]} if f["transcript"] is not None else {}),
        }

    def file_or_404(self, file_id: str) -> tuple[UUID, dict[str, Any]]:
        """The record of a stored file, or ``404`` for an unknown or malformed ID."""
        try:
            key = UUID(file_id)
        except ValueError:
            raise FakeProblem("not_found") from None
        f = self.files.get(key)
        if f is None:
            raise FakeProblem("not_found")
        return key, f

    def file(self, file_id: str) -> dict[str, Any]:
        """``getFile``: the ``File`` of a stored file (uploaded by Alex)."""
        key, f = self.file_or_404(file_id)
        return file_doc(key, f, uploader=self.alex.id, at=self.now_iso())

    def submit_transcript(self, file_id: str, body: Any, if_match: str | None) -> dict[str, Any]:
        """``submitTranscript``: Lotse stores the transcript under the derived ``If-Match``."""
        key, f = self.file_or_404(file_id)
        return submit_transcript(key, f, body, if_match)

    def download(self, file_id: str) -> tuple[bytes, str]:
        """``downloadFile``: the bytes of a clean or skipped file."""
        _, f = self.file_or_404(file_id)
        if f["status"] == "blocked":
            raise FakeProblem("file_blocked")
        if f["status"] not in ("clean", "skipped"):
            raise FakeProblem("file_not_ready")
        return f["content"], f["mime"]

    def current_session(self, thread_id: UUID) -> dict[str, Any] | None:
        """The current agent session of a thread, if any."""
        return next((r for r in self.sessions.get(thread_id, []) if r["is_current"]), None)

    def session_for(
        self, thread_id: UUID, ref: str, *, started: bool, model: str | None = None
    ) -> dict[str, Any]:
        """Make ``ref`` the current session of the thread (version 1)."""
        for old in self.sessions.get(thread_id, []):
            old["is_current"] = False
        row: dict[str, Any] = {
            "id": str(uuid7()),
            "thread_id": str(thread_id),
            "agent_id": str(self.lotse.id),
            "external_session_ref": ref,
            "is_current": True,
            "started": started,
            "cost": {"amount_micros": 0, "currency": "USD", "is_estimate": True},
            "created_at": self.now_iso(),
            "version": 1,
        }
        if model:
            row["model"] = model
        self.sessions.setdefault(thread_id, []).append(row)
        return row

    def message_or_404(self, message_id: str) -> dict[str, Any]:
        """``getMessage``: the stored message, or ``404 not_found`` (also when hidden)."""
        try:
            key = UUID(message_id)
        except ValueError:
            raise FakeProblem("not_found") from None
        message = self.messages.get(key)
        if message is None or key in self.hidden_messages:
            raise FakeProblem("not_found")
        # The components as they are now: a click or updateComponent changes state and version.
        components = [
            {k: v for k, v in self.components[UUID(c["id"])].items() if k != "message_id"}
            for c in message["components"]
        ]
        return {**message, "components": components}

    def report_session(self, thread_id: str, body: Any, if_match: str | None) -> dict[str, Any]:
        """``reportAgentSession``: compare-and-swap over the current row's version (09 9.1, 9.5).

        Only during an active run of the thread. A different reference replaces the stored one
        (9.5 point 3); ``bind_failed`` without a current row answers ``session_not_bound``.
        """
        meta = self._thread_or_404(thread_id)
        _closed(
            body, required=("external_session_ref",), optional=("model", "started", "bind_failed")
        )
        if not isinstance(body["external_session_ref"], str) or not body["external_session_ref"]:
            raise field_error("/external_session_ref", "required")
        if "bind_failed" in body and body["bind_failed"] != "session_missing":
            raise field_error("/bind_failed", "invalid_enum")
        key = UUID(meta["id"])
        active = [r for r in self.runs.values() if r.thread_id == key and r.state in _ACTIVE]
        if not active:
            raise FakeProblem("conflict", reason="run_not_active")
        row = self.current_session(key)
        created = row is None
        if row is None:
            if if_match is not None:
                raise FakeProblem("version_conflict")
            if "bind_failed" in body:
                raise FakeProblem("conflict", reason="session_not_bound")
            row = self.session_for(key, body["external_session_ref"], started=False)
        elif if_match is None or if_match.strip() not in (
            f'"{row["version"]}"',
            f'W/"{row["version"]}"',
        ):
            raise FakeProblem("version_conflict", current=dict(row))
        if body.get("bind_failed") is not None and not created:
            # M2-12 note 12: the bound row ends with rebind and a new current row takes over.
            if not row["started"]:
                raise FakeProblem("conflict", reason="session_not_bound")
            row["is_current"] = False
            row["end_reason"] = "rebind"
            new = self.session_for(
                key,
                body["external_session_ref"],
                started=body.get("started") is True,
                model=body.get("model"),
            )
            self.extra.reports.setdefault(key, []).append(dict(body))
            return dict(new)
        row["external_session_ref"] = body["external_session_ref"]
        if body.get("started") is True:
            row["started"] = True
        if body.get("model"):
            row["model"] = body["model"]
        if not created:
            row["version"] += 1
        self.extra.reports.setdefault(key, []).append(dict(body))
        return dict(row)

    def heartbeat(self, body: Any) -> dict[str, Any]:
        """``sendHeartbeat``: store the report and answer the presence."""
        if not isinstance(body, dict) or body.get("state") not in ("online", "degraded"):
            raise field_error(
                "/state",
                "required" if not isinstance(body, dict) or "state" not in body else "invalid_enum",
            )
        if "rate_limit" in body:
            _check_rate_limit(body["rate_limit"])
        self.heartbeats.append(dict(body))
        presence: dict[str, Any] = {
            "state": body["state"],
            "last_heartbeat_at": self.now_iso(),
            "billing_mode_matches": body.get("reported_billing_mode", self.billing_mode)
            == self.billing_mode,
        }
        for name in ("reported_billing_mode", "runtime_version", "reported_tool_class"):
            if name in body:
                presence[name] = body[name]
        return {
            "presence": presence,
            "interval_ms": int(self.timings["sync.heartbeat_interval"] * 1000),
        }

    # Messages, components, interactions, commands -----------------------------------------

    def create_message(self, thread_id: str, body: Any) -> tuple[dict[str, Any], bool]:
        """``createMessage``, idempotent by ``client_msg_id``.

        Returns:
            The Message and whether it was created now.
        """
        if not isinstance(body, dict) or "client_msg_id" not in body:
            raise field_error("/client_msg_id", "required")
        try:
            client_msg_id = UUID(str(body["client_msg_id"]))
            thread = UUID(thread_id)
        except ValueError:
            raise field_error("/client_msg_id", "invalid_format") from None
        if thread not in self.threads:
            raise FakeProblem("not_found")
        existing = self.by_client_msg_id.get(client_msg_id)
        if existing is not None:
            return self.messages[existing], False
        run_id = UUID(body["run_id"]) if body.get("run_id") else None
        message = self._message(
            thread,
            self.lotse,
            str(body.get("content_md", "")),
            client_msg_id=client_msg_id,
            run_id=run_id,
            kind=str(body.get("kind", "text")),
        )
        for position, spec in enumerate(body.get("components", [])):
            self._component(message, spec, position)
        self.by_client_msg_id[client_msg_id] = UUID(message["id"])
        self.message_bodies.append(dict(body))
        return message, True

    def _component(
        self, message: dict[str, Any], spec: Mapping[str, Any], position: int
    ) -> dict[str, Any]:
        component: dict[str, Any] = {
            "id": str(uuid7()),
            "custom_id": spec["custom_id"],
            "type": spec["type"],
            "definition": dict(spec.get("definition", {})),
            "position": position,
            "single_use": bool(spec.get("single_use", False)),
            "restricted_to": spec.get("restricted_to", "any"),
            "state": "active",
            "version": 1,
            "message_id": message["id"],
        }
        for name in ("required_capability", "expires_at"):
            if name in spec:
                component[name] = spec[name]
        if "run_id" in message:
            component["run_id"] = message["run_id"]
        self.components[UUID(component["id"])] = component
        message["components"].append({k: v for k, v in component.items() if k != "message_id"})
        return component

    def update_component(
        self, component_id: str, body: Any, if_match: str | None
    ) -> dict[str, Any]:
        """``updateComponent`` with ``If-Match``."""
        component = self._get(self.components, component_id)
        _check_if_match(if_match, component["version"])
        _closed(body, optional=("state", "definition"), enums={"state": ("active", "disabled")})
        component.update(body)
        component["version"] += 1
        return {k: v for k, v in component.items() if k != "message_id"}

    def respond(self, interaction_id: str, body: Any) -> dict[str, Any]:
        """``respondToInteraction``."""
        interaction = self._get(self.interactions, interaction_id)
        _closed(
            body,
            required=("state",),
            optional=("response",),
            enums={"state": ("acknowledged", "failed", "rejected")},
        )
        interaction["state"] = body["state"]
        if "response" in body:
            interaction["response"] = body["response"]
        return interaction

    def register_command(self, body: Any) -> dict[str, Any]:
        """``registerCommand`` for Lotse."""
        _closed(
            body,
            required=("name", "description"),
            optional=("scope_kind", "scope_id", "options", "required_level", "control"),
        )
        if not _COMMAND_NAME.match(str(body["name"])):
            raise field_error("/name", "pattern_mismatch")
        scope_kind = body.get("scope_kind", "instance")
        scope_id = body.get("scope_id")
        for command in self.commands.values():
            if (command["name"], command["scope_kind"], command.get("scope_id")) == (
                body["name"],
                scope_kind,
                scope_id,
            ):
                raise FakeProblem("conflict", reason="command_exists")
        command = {
            "id": str(uuid7()),
            "owner_principal_id": str(self.lotse.id),
            "name": body["name"],
            "description": body["description"],
            "scope_kind": scope_kind,
            "options": list(body.get("options", [])),
            "required_level": body.get("required_level", "trigger"),
            "control": bool(body.get("control", False)),
            "version": 1,
        }
        if scope_id is not None:
            command["scope_id"] = scope_id
        self.commands[UUID(command["id"])] = command
        return command

    def update_command(self, command_id: str, body: Any, if_match: str | None) -> dict[str, Any]:
        """``updateCommand`` with ``If-Match``."""
        command = self._get(self.commands, command_id)
        _check_if_match(if_match, command["version"])
        _closed(
            body,
            required=("name", "description"),
            optional=("scope_kind", "scope_id", "options", "required_level", "control"),
        )
        command.update(body)
        command["version"] += 1
        return command

    def channel_commands(self, channel_id: str) -> dict[str, Any]:
        """``listChannelCommands``: commands visible in the channel."""
        visible = [
            c
            for c in self.commands.values()
            if c["scope_kind"] == "instance"
            or (c["scope_kind"] == "channel" and c.get("scope_id") == channel_id)
            or (c["scope_kind"] == "space" and c.get("scope_id") == str(self.space_id))
        ]
        if channel_id != str(self.channel_id):
            self.channel_or_404(channel_id)
        return {"items": visible}

    @staticmethod
    def _get(table: Mapping[UUID, dict[str, Any]], key: str) -> dict[str, Any]:
        try:
            found = table.get(UUID(key))
        except ValueError:
            found = None
        if found is None:
            raise FakeProblem("not_found")
        return found


def _check_if_match(if_match: str | None, version: int) -> None:
    if if_match is None:
        raise FakeProblem("precondition_required")
    if if_match.strip() not in (f'"{version}"', f'W/"{version}"'):
        raise FakeProblem("version_conflict")


def _closed(
    body: Any,
    *,
    required: tuple[str, ...] = (),
    optional: tuple[str, ...] = (),
    enums: Mapping[str, tuple[str, ...]] | None = None,
) -> None:
    """The schema stage for a closed request body (``400 validation_failed``)."""
    if not isinstance(body, dict):
        raise field_error("", "required")
    errors = [
        {"pointer": f"/{n}", "code": "unknown_member"}
        for n in sorted(set(body) - set(required) - set(optional))
    ]
    errors += [{"pointer": f"/{n}", "code": "required"} for n in required if n not in body]
    for name, values in (enums or {}).items():
        if name in body and body[name] not in values:
            errors.append({"pointer": f"/{name}", "code": "invalid_enum"})
    if errors:
        raise FakeProblem("validation_failed", errors=errors)


_RATE_LIMIT_STATES = ("ok", "near_limit", "limited", "unknown")


def _check_rate_limit(value: Any) -> None:
    """Check ``rate_limit`` of a heartbeat as the server does (M6-14).

    ``state`` from the closed list, an optional string ``resets_at`` and no other member.
    """
    if not isinstance(value, dict):
        raise field_error("/rate_limit", "invalid_type")
    for key in value:
        if key not in ("state", "resets_at"):
            raise field_error(f"/rate_limit/{key}", "unknown_member")
    if "state" not in value:
        raise field_error("/rate_limit/state", "required")
    if value["state"] not in _RATE_LIMIT_STATES:
        raise field_error("/rate_limit/state", "invalid_enum")
    if "resets_at" in value and not isinstance(value["resets_at"], str):
        raise field_error("/rate_limit/resets_at", "invalid_type")
