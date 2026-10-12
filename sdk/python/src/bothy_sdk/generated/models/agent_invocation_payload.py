from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_invocation_payload_trigger_kind import (
    AgentInvocationPayloadTriggerKind,
)
from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_session import AgentSession
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
        agent_session (AgentSession | Unset):
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
    agent_session: AgentSession | Unset = UNSET
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

        agent_session: dict[str, Any] | Unset = UNSET
        if not isinstance(self.agent_session, Unset):
            agent_session = self.agent_session.to_dict()

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
        if agent_session is not UNSET:
            field_dict["agent_session"] = agent_session
        if settings_version is not UNSET:
            field_dict["settings_version"] = settings_version

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_session import AgentSession
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

        _agent_session = d.pop("agent_session", UNSET)
        agent_session: AgentSession | Unset
        if isinstance(_agent_session, Unset):
            agent_session = UNSET
        else:
            agent_session = AgentSession.from_dict(_agent_session)

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
            agent_session=agent_session,
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
