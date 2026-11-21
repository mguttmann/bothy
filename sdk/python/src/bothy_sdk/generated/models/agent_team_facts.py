from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_team_facts_source import AgentTeamFactsSource
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_team_member import AgentTeamMember


T = TypeVar("T", bound="AgentTeamFacts")


@_attrs_define
class AgentTeamFacts:
    """The team facts of a `team` run (09 3.9.9). The owner's authority never comes from here; it comes only from the input
    message (origin `human` and an owner as author).

        Attributes:
            channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            member_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            handle (str):
            source (AgentTeamFactsSource):
            addressed (bool): The input mentions the receiving member's handle.
            members (list[AgentTeamMember]):
            source_handle (str | Unset): The handle of the source member; set exactly for `source = member`.
    """

    channel_id: UUID
    member_id: UUID
    handle: str
    source: AgentTeamFactsSource
    addressed: bool
    members: list[AgentTeamMember]
    source_handle: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        member_id = str(self.member_id)

        handle = self.handle

        source = self.source.value

        addressed = self.addressed

        members = []
        for members_item_data in self.members:
            members_item = members_item_data.to_dict()
            members.append(members_item)

        source_handle = self.source_handle

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "channel_id": channel_id,
                "member_id": member_id,
                "handle": handle,
                "source": source,
                "addressed": addressed,
                "members": members,
            }
        )
        if source_handle is not UNSET:
            field_dict["source_handle"] = source_handle

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_team_member import AgentTeamMember

        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        member_id = UUID(d.pop("member_id"))

        handle = d.pop("handle")

        source = AgentTeamFactsSource(d.pop("source"))

        addressed = d.pop("addressed")

        members = []
        _members = d.pop("members")
        for members_item_data in _members:
            members_item = AgentTeamMember.from_dict(members_item_data)

            members.append(members_item)

        source_handle = d.pop("source_handle", UNSET)

        agent_team_facts = cls(
            channel_id=channel_id,
            member_id=member_id,
            handle=handle,
            source=source,
            addressed=addressed,
            members=members,
            source_handle=source_handle,
        )

        agent_team_facts.additional_properties = d
        return agent_team_facts

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
