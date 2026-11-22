from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="AgentTeamMember")


@_attrs_define
class AgentTeamMember:
    """
    Attributes:
        member_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        handle (str):
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        home_channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    member_id: UUID
    handle: str
    agent_id: UUID
    home_channel_id: UUID
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        member_id = str(self.member_id)

        handle = self.handle

        agent_id = str(self.agent_id)

        home_channel_id = str(self.home_channel_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "member_id": member_id,
                "handle": handle,
                "agent_id": agent_id,
                "home_channel_id": home_channel_id,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        member_id = UUID(d.pop("member_id"))

        handle = d.pop("handle")

        agent_id = UUID(d.pop("agent_id"))

        home_channel_id = UUID(d.pop("home_channel_id"))

        agent_team_member = cls(
            member_id=member_id,
            handle=handle,
            agent_id=agent_id,
            home_channel_id=home_channel_id,
        )

        agent_team_member.additional_properties = d
        return agent_team_member

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
