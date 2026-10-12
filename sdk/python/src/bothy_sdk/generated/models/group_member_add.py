from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="GroupMemberAdd")


@_attrs_define
class GroupMemberAdd:
    """
    Attributes:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    user_id: UUID
    expires_at: datetime.datetime | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        user_id = str(self.user_id)

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "user_id": user_id,
            }
        )
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        user_id = UUID(d.pop("user_id"))

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        group_member_add = cls(
            user_id=user_id,
            expires_at=expires_at,
        )

        return group_member_add
