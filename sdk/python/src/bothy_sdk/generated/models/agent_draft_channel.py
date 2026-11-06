from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="AgentDraftChannel")


@_attrs_define
class AgentDraftChannel:
    """Where people talk to the new agent; the creation flow offers to create this channel (26 8.2).

    Attributes:
        space_name (str):
        channel_name (str):
    """

    space_name: str
    channel_name: str

    def to_dict(self) -> dict[str, Any]:
        space_name = self.space_name

        channel_name = self.channel_name

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "space_name": space_name,
                "channel_name": channel_name,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        space_name = d.pop("space_name")

        channel_name = d.pop("channel_name")

        agent_draft_channel = cls(
            space_name=space_name,
            channel_name=channel_name,
        )

        return agent_draft_channel
