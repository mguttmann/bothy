from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="AgentHostingChangedPayload")


@_attrs_define
class AgentHostingChangedPayload:
    """Payload of `agent.hosting_changed` (26 9); only the id of the built-in agent, the runner reads the rest with
    `listHostedAgents`.

        Attributes:
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    agent_id: UUID
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        agent_id = str(self.agent_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "agent_id": agent_id,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        agent_id = UUID(d.pop("agent_id"))

        agent_hosting_changed_payload = cls(
            agent_id=agent_id,
        )

        agent_hosting_changed_payload.additional_properties = d
        return agent_hosting_changed_payload

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
