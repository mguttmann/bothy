from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.team_pause_reason import TeamPauseReason
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.team_member import TeamMember


T = TypeVar("T", bound="Team")


@_attrs_define
class Team:
    """The team of a team channel (09 3.9.1); `pause_reason` is absent while member inputs flow.

    Attributes:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        sponsor_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        member_turns (int): Member inputs since an owner's last message (09 3.9.6).
        turn_budget (int): The value of `team.turn_budget` now.
        members (list[TeamMember]):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        version (int):
        pause_reason (TeamPauseReason | Unset): Why member inputs are paused (09 3.9.6, 3.9.7); an owner message lifts
            every pause.
    """

    channel_id: UUID
    sponsor_id: UUID
    member_turns: int
    turn_budget: int
    members: list[TeamMember]
    created_at: datetime.datetime
    version: int
    pause_reason: TeamPauseReason | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        sponsor_id = str(self.sponsor_id)

        member_turns = self.member_turns

        turn_budget = self.turn_budget

        members = []
        for members_item_data in self.members:
            members_item = members_item_data.to_dict()
            members.append(members_item)

        created_at = self.created_at.isoformat()

        version = self.version

        pause_reason: str | Unset = UNSET
        if not isinstance(self.pause_reason, Unset):
            pause_reason = self.pause_reason.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "channel_id": channel_id,
                "sponsor_id": sponsor_id,
                "member_turns": member_turns,
                "turn_budget": turn_budget,
                "members": members,
                "created_at": created_at,
                "version": version,
            }
        )
        if pause_reason is not UNSET:
            field_dict["pause_reason"] = pause_reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.team_member import TeamMember

        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        sponsor_id = UUID(d.pop("sponsor_id"))

        member_turns = d.pop("member_turns")

        turn_budget = d.pop("turn_budget")

        members = []
        _members = d.pop("members")
        for members_item_data in _members:
            members_item = TeamMember.from_dict(members_item_data)

            members.append(members_item)

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        version = d.pop("version")

        _pause_reason = d.pop("pause_reason", UNSET)
        pause_reason: TeamPauseReason | Unset
        if isinstance(_pause_reason, Unset):
            pause_reason = UNSET
        else:
            pause_reason = TeamPauseReason(_pause_reason)

        team = cls(
            channel_id=channel_id,
            sponsor_id=sponsor_id,
            member_turns=member_turns,
            turn_budget=turn_budget,
            members=members,
            created_at=created_at,
            version=version,
            pause_reason=pause_reason,
        )

        team.additional_properties = d
        return team

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
