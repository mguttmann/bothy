from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="GroupMemberExtend")


@_attrs_define
class GroupMemberExtend:
    """The later expiry of a time-limited membership in effect (`extendGroupMember`).

    Attributes:
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    expires_at: datetime.datetime

    def to_dict(self) -> dict[str, Any]:
        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        group_member_extend = cls(
            expires_at=expires_at,
        )

        return group_member_extend
