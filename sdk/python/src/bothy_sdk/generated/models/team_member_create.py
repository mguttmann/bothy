from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="TeamMemberCreate")


@_attrs_define
class TeamMemberCreate:
    """
    Attributes:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        home_channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        handle (str): Unique in the team channel, at most `team.handle_max_chars` characters.
    """

    agent_id: UUID
    home_channel_id: UUID
    handle: str

    def to_dict(self) -> dict[str, Any]:
        agent_id = str(self.agent_id)

        home_channel_id = str(self.home_channel_id)

        handle = self.handle

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "agent_id": agent_id,
                "home_channel_id": home_channel_id,
                "handle": handle,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        agent_id = UUID(d.pop("agent_id"))

        home_channel_id = UUID(d.pop("home_channel_id"))

        handle = d.pop("handle")

        team_member_create = cls(
            agent_id=agent_id,
            home_channel_id=home_channel_id,
            handle=handle,
        )

        return team_member_create
