from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_invocation_payload_session_mode import (
    AgentInvocationPayloadSessionMode,
)
from ..models.agent_invocation_payload_trigger_kind import (
    AgentInvocationPayloadTriggerKind,
)
from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_session import AgentSession
    from ..models.command_option_value import CommandOptionValue
    from ..models.principal_ref import PrincipalRef


T = TypeVar("T", bound="AgentInvocationPayload")


@_attrs_define
class AgentInvocationPayload:
    """Sent to the agent; the envelope carries a fresh `invocation_token` per delivery.

    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        trigger_kind (AgentInvocationPayloadTriggerKind):
        triggered_by (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        interaction_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        command_invocation_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        command_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        command_name (str | Unset): Name of the triggering command; set exactly for `trigger_kind = command`.
        options (list[CommandOptionValue] | Unset): Parsed option values in the order of the command definition; set
            exactly for `trigger_kind = command`.
        agent_session (AgentSession | Unset):
        session_mode (AgentInvocationPayloadSessionMode | Unset): `session.mode` of 09, 9.1: `new` creates the external
            session (with the reference of `agent_session` when present), `resume` continues it, `bind` takes over a session
            that no run used yet (rebinding, import; 9.5). Present exactly for a run of the thread's operator; a guest run
            has neither `session_mode` nor `agent_session` (09, 3.6).
        settings_version (int | Unset): Channel settings version; reload `agent.config.read` when newer than cached.
    """

    run_id: UUID
    thread_id: UUID
    channel_id: UUID
    trigger_kind: AgentInvocationPayloadTriggerKind
    triggered_by: PrincipalRef
    billing_mode: BillingMode
    space_id: UUID | Unset = UNSET
    message_id: UUID | Unset = UNSET
    interaction_id: UUID | Unset = UNSET
    command_invocation_id: UUID | Unset = UNSET
    command_id: UUID | Unset = UNSET
    command_name: str | Unset = UNSET
    options: list[CommandOptionValue] | Unset = UNSET
    agent_session: AgentSession | Unset = UNSET
    session_mode: AgentInvocationPayloadSessionMode | Unset = UNSET
    settings_version: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        trigger_kind = self.trigger_kind.value

        triggered_by = self.triggered_by.to_dict()

        billing_mode = self.billing_mode.value

        space_id: str | Unset = UNSET
        if not isinstance(self.space_id, Unset):
            space_id = str(self.space_id)

        message_id: str | Unset = UNSET
        if not isinstance(self.message_id, Unset):
            message_id = str(self.message_id)

        interaction_id: str | Unset = UNSET
        if not isinstance(self.interaction_id, Unset):
            interaction_id = str(self.interaction_id)

        command_invocation_id: str | Unset = UNSET
        if not isinstance(self.command_invocation_id, Unset):
            command_invocation_id = str(self.command_invocation_id)

        command_id: str | Unset = UNSET
        if not isinstance(self.command_id, Unset):
            command_id = str(self.command_id)

        command_name = self.command_name

        options: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.options, Unset):
            options = []
            for options_item_data in self.options:
                options_item = options_item_data.to_dict()
                options.append(options_item)

        agent_session: dict[str, Any] | Unset = UNSET
        if not isinstance(self.agent_session, Unset):
            agent_session = self.agent_session.to_dict()

        session_mode: str | Unset = UNSET
        if not isinstance(self.session_mode, Unset):
            session_mode = self.session_mode.value

        settings_version = self.settings_version

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "thread_id": thread_id,
                "channel_id": channel_id,
                "trigger_kind": trigger_kind,
                "triggered_by": triggered_by,
                "billing_mode": billing_mode,
            }
        )
        if space_id is not UNSET:
            field_dict["space_id"] = space_id
        if message_id is not UNSET:
            field_dict["message_id"] = message_id
        if interaction_id is not UNSET:
            field_dict["interaction_id"] = interaction_id
        if command_invocation_id is not UNSET:
            field_dict["command_invocation_id"] = command_invocation_id
        if command_id is not UNSET:
            field_dict["command_id"] = command_id
        if command_name is not UNSET:
            field_dict["command_name"] = command_name
        if options is not UNSET:
            field_dict["options"] = options
        if agent_session is not UNSET:
            field_dict["agent_session"] = agent_session
        if session_mode is not UNSET:
            field_dict["session_mode"] = session_mode
        if settings_version is not UNSET:
            field_dict["settings_version"] = settings_version

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_session import AgentSession
        from ..models.command_option_value import CommandOptionValue
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        trigger_kind = AgentInvocationPayloadTriggerKind(d.pop("trigger_kind"))

        triggered_by = PrincipalRef.from_dict(d.pop("triggered_by"))

        billing_mode = BillingMode(d.pop("billing_mode"))

        _space_id = d.pop("space_id", UNSET)
        space_id: UUID | Unset
        if isinstance(_space_id, Unset):
            space_id = UNSET
        else:
            space_id = UUID(_space_id)

        _message_id = d.pop("message_id", UNSET)
        message_id: UUID | Unset
        if isinstance(_message_id, Unset):
            message_id = UNSET
        else:
            message_id = UUID(_message_id)

        _interaction_id = d.pop("interaction_id", UNSET)
        interaction_id: UUID | Unset
        if isinstance(_interaction_id, Unset):
            interaction_id = UNSET
        else:
            interaction_id = UUID(_interaction_id)

        _command_invocation_id = d.pop("command_invocation_id", UNSET)
        command_invocation_id: UUID | Unset
        if isinstance(_command_invocation_id, Unset):
            command_invocation_id = UNSET
        else:
            command_invocation_id = UUID(_command_invocation_id)

        _command_id = d.pop("command_id", UNSET)
        command_id: UUID | Unset
        if isinstance(_command_id, Unset):
            command_id = UNSET
        else:
            command_id = UUID(_command_id)

        command_name = d.pop("command_name", UNSET)

        _options = d.pop("options", UNSET)
        options: list[CommandOptionValue] | Unset = UNSET
        if _options is not UNSET:
            options = []
            for options_item_data in _options:
                options_item = CommandOptionValue.from_dict(options_item_data)

                options.append(options_item)

        _agent_session = d.pop("agent_session", UNSET)
        agent_session: AgentSession | Unset
        if isinstance(_agent_session, Unset):
            agent_session = UNSET
        else:
            agent_session = AgentSession.from_dict(_agent_session)

        _session_mode = d.pop("session_mode", UNSET)
        session_mode: AgentInvocationPayloadSessionMode | Unset
        if isinstance(_session_mode, Unset):
            session_mode = UNSET
        else:
            session_mode = AgentInvocationPayloadSessionMode(_session_mode)

        settings_version = d.pop("settings_version", UNSET)

        agent_invocation_payload = cls(
            run_id=run_id,
            thread_id=thread_id,
            channel_id=channel_id,
            trigger_kind=trigger_kind,
            triggered_by=triggered_by,
            billing_mode=billing_mode,
            space_id=space_id,
            message_id=message_id,
            interaction_id=interaction_id,
            command_invocation_id=command_invocation_id,
            command_id=command_id,
            command_name=command_name,
            options=options,
            agent_session=agent_session,
            session_mode=session_mode,
            settings_version=settings_version,
        )

        agent_invocation_payload.additional_properties = d
        return agent_invocation_payload

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
