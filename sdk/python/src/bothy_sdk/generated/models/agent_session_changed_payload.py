from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.agent_session import AgentSession


T = TypeVar("T", bound="AgentSessionChangedPayload")


@_attrs_define
class AgentSessionChangedPayload:
    """
    Attributes:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_session (AgentSession):
    """

    thread_id: UUID
    agent_session: AgentSession
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread_id = str(self.thread_id)

        agent_session = self.agent_session.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "thread_id": thread_id,
                "agent_session": agent_session,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_session import AgentSession

        d = dict(src_dict)
        thread_id = UUID(d.pop("thread_id"))

        agent_session = AgentSession.from_dict(d.pop("agent_session"))

        agent_session_changed_payload = cls(
            thread_id=thread_id,
            agent_session=agent_session,
        )

        agent_session_changed_payload.additional_properties = d
        return agent_session_changed_payload

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
