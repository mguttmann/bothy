from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.resolve_i1 import ResolveI1


T = TypeVar("T", bound="GroupMemberAdd")


@_attrs_define
class GroupMemberAdd:
    """
    Attributes:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        resolve_i1 (ResolveI1 | Unset): Guided flow (05 rule 1.6, 10 section 11): switch these agents to `organization`
            first, in the same transaction as the change.
    """

    user_id: UUID
    expires_at: datetime.datetime | Unset = UNSET
    resolve_i1: ResolveI1 | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        user_id = str(self.user_id)

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        resolve_i1: dict[str, Any] | Unset = UNSET
        if not isinstance(self.resolve_i1, Unset):
            resolve_i1 = self.resolve_i1.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "user_id": user_id,
            }
        )
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at
        if resolve_i1 is not UNSET:
            field_dict["resolve_i1"] = resolve_i1

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.resolve_i1 import ResolveI1

        d = dict(src_dict)
        user_id = UUID(d.pop("user_id"))

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        _resolve_i1 = d.pop("resolve_i1", UNSET)
        resolve_i1: ResolveI1 | Unset
        if isinstance(_resolve_i1, Unset):
            resolve_i1 = UNSET
        else:
            resolve_i1 = ResolveI1.from_dict(_resolve_i1)

        group_member_add = cls(
            user_id=user_id,
            expires_at=expires_at,
            resolve_i1=resolve_i1,
        )

        return group_member_add
