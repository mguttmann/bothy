# SPDX-License-Identifier: Apache-2.0
"""The FakeServer's data and run rules (private).

One world after docs/spec/07-permissions.md "Examples": owner Robin, Alex who triggers, the agent
Lotse that operates the channel Allgemein of space Support, and the agent Texter. The run rules
follow the server code of M2-08 to M2-10 (docs/spec/09-agents.md 4.5, 4.5.1, 5.1 and 5.3,
docs/spec/04-sync-protocol.md 11.2 and 15.2 to 15.5): fresh tokens per delivery, the start
verdicts ``superseded``, ``expired`` and ``invalid``, consumed tokens, ``already_started``,
mismatches that fail the run, renewals with ``renewed_from_jti`` and patches with
``base_version``.
"""

from __future__ import annotations

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
from bothy_sdk.testing._fake_patch import apply_ops, parse_ops, text_error
from bothy_sdk.testing._fake_tokens import TRIGGER_OBJECT, Issuer
from bothy_sdk.testing.doubles import FakeClock

ACTIVE = frozenset({"queued", "running", "waiting_input"})
ENDED = frozenset({"completed", "failed", "cancelled"})
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


class World:
    """Data and rules; the transport layers call into it.

    Args:
        clock: The shared FakeClock.
        timings: The server keys the FakeServer plays.
        billing_mode: Lotse's configured billing mode.
        tool_class: Lotse's configured tool class.
        append: Appends a durable event to the log and wakes the deliverers.
    """

    def __init__(
        self,
        clock: FakeClock,
        timings: Mapping[str, float],
        *,
        billing_mode: str,
        tool_class: str,
        append: Callable[[str, dict[str, Any]], LogEntry],
    ) -> None:
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
        self.people = {p.id: p for p in (self.robin, self.alex, self.lotse, self.texter)}
        self.permission_version = 1
        self.settings_version = 1
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
            "channel_id": str(self.channel_id),
            "agent_id": str(self.lotse.id),
            "triggered_by": str(run.triggered_by),
            "trigger_kind": "message",
            "triggering_message_id": str(run.triggering_message_id),
            "state": run.state,
            "version": run.version,
            "billing_mode_expected": run.billing_mode_expected,
            "metrics": dict(run.live.get("metrics", {})),
            "cost": run.live.get(
                "cost", {"amount_micros": 0, "currency": "USD", "is_estimate": True}
            ),
            "steps": dict(run.live.get("steps", {})),
            "followups": {},
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
    ) -> dict[str, Any]:
        """An envelope of type ``kind`` built from its fixture (without ``seq``)."""
        scope = {"space_id": str(self.space_id), "channel_id": str(self.channel_id)}
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
        message_id = uuid7()
        message: dict[str, Any] = {
            "id": str(message_id),
            "thread_id": str(thread_id),
            "channel_id": str(self.channel_id),
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
        return message

    def trigger(self, text: str) -> RunRecord:
        """A new thread with Alex's message, a queued run and its ``agent.invocation``."""
        thread_id = uuid7()
        message = self._message(thread_id, self.alex, text)
        run = RunRecord(
            id=uuid7(),
            thread_id=thread_id,
            triggered_by=self.alex.id,
            triggering_message_id=UUID(message["id"]),
            queued_at=self.now(),
            billing_mode_expected=self.billing_mode,
        )
        self.runs[run.id] = run
        payload = {
            "run_id": str(run.id),
            "thread_id": str(thread_id),
            "channel_id": str(self.channel_id),
            "space_id": str(self.space_id),
            "trigger_kind": "message",
            "message_id": message["id"],
            "triggered_by": self.alex.ref(),
            "billing_mode": self.billing_mode,
            "settings_version": self.settings_version,
        }
        envelope = self.envelope(
            "agent.invocation",
            payload,
            resource={"kind": "run", "id": str(run.id)},
            thread_id=thread_id,
            actor=self.alex,
        )
        self.append("agent.invocation", envelope)
        return run

    def followup(self, run_id: UUID, text: str) -> UUID:
        """Alex's follow-up message in the run's thread and its ``agent.followup``."""
        run = self.runs[run_id]
        message = self._message(run.thread_id, self.alex, text)
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
            actor=self.alex,
        )
        self.append("agent.followup", envelope)
        return UUID(message["id"])

    def interaction(self, run_id: UUID, custom_id: str, values: Mapping[str, Any]) -> UUID:
        """Alex operates a component of the run: ``interaction.created``."""
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
            "principal_id": str(self.alex.id),
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
            actor=self.alex,
        )
        self.append("interaction.created", envelope)
        return UUID(interaction["id"])

    def command_invoked(
        self, command: dict[str, Any], run_id: UUID | None, thread_id: UUID | None
    ) -> UUID:
        """Alex runs a command of Lotse: ``command.invoked``."""
        if run_id is not None:
            thread_id = self.runs[run_id].thread_id
        invocation_id = uuid7()
        payload: dict[str, Any] = {
            "invocation_id": str(invocation_id),
            "command_id": command["id"],
            "command_name": command["name"],
            "control": command["control"],
            "channel_id": str(self.channel_id),
            "invoked_by": self.alex.ref(),
            "options": [],
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
        claims: dict[str, Any] = {
            "aud": str(self.lotse.id),
            "sub": str(sub),
            "iat": iat,
            "exp": iat + int(self.timings["agent.invocation_token_ttl"]),
            "trigger_kind": kind,
            TRIGGER_OBJECT[kind]: obj,
            "thread_id": str(thread_id) if thread_id else str(self.channel_id),
            "channel_id": str(self.channel_id),
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
            kind="message",
            obj=str(run.triggering_message_id),
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
            kind, obj, sub = (
                "message",
                payload["message_id"],
                run.triggered_by if run else self.alex.id,
            )
        elif entry.type == "agent.followup":
            kind, obj, sub = "followup", payload["message_id"], self.alex.id
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
        if run is None or claims["sub"] != str(run.triggered_by):
            raise invalid
        if claims["thread_id"] != str(run.thread_id):
            raise invalid
        if run.state in ENDED and operation_id not in (
            "startRun",
            "resumeRun",
            "renewInvocationToken",
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
        if "external_session_ref" in body:
            raise field_error("/external_session_ref", "not_enabled")
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
        if "costs" in body:
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
        """``getRunContext``: the trigger and the thread as marked items."""
        run = self.run_or_404(run_id)
        items = [self._context_item(self.messages[m], run) for m in self.threads[run.thread_id]]
        trigger = next(i for i in items if i["message_id"] == str(run.triggering_message_id))
        doc = {
            "run_id": str(run.id),
            "trigger_message": trigger,
            "thread_context": items,
            "channel_context": [],
            "context_floor_seq": 0,
        }
        return fixtures.build("run_context", doc, replace=_CONTEXT_MEMBERS)

    def _context_item(self, message: dict[str, Any], run: RunRecord) -> dict[str, Any]:
        own = message["author"]["id"] == str(self.lotse.id)
        trigger = message["id"] == str(run.triggering_message_id)
        return {
            "message_id": message["id"],
            "thread_seq": message["thread_seq"],
            "created_at": message["created_at"],
            "author": message["author"],
            "role": "assistant" if own else "user",
            "origin": message["origin"],
            "kind": message["kind"],
            "content_md": message["content_md"],
            "attachments": [],
            "trust": "trigger" if trigger else ("self" if own else "authorized"),
        }

    def channel_config(self) -> dict[str, Any]:
        """The ``AgentChannelConfig`` of Allgemein."""
        return {
            "channel_id": str(self.channel_id),
            "space_id": str(self.space_id),
            "behavior": {
                "require_mention": True,
                "thread_require_mention": False,
                "free_response": False,
                "auto_thread": True,
                "allow_mentioned_agents": True,
                "history_backfill_limit": 50,
                "prompt": "",
                "skills": [],
                "settings_version": self.settings_version,
            },
        }

    def heartbeat(self, body: Any) -> dict[str, Any]:
        """``sendHeartbeat``: store the report and answer the presence."""
        if not isinstance(body, dict) or body.get("state") not in ("online", "degraded"):
            raise field_error(
                "/state",
                "required" if not isinstance(body, dict) or "state" not in body else "invalid_enum",
            )
        if "rate_limit" in body:
            raise field_error("/rate_limit", "not_enabled")
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
            raise FakeProblem("not_found")
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
