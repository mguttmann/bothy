from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.principal_ref import PrincipalRef


T = TypeVar("T", bound="GroupMember")


@_attrs_define
class GroupMember:
    """
    Attributes:
        user (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        added_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        added_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    user: PrincipalRef
    added_at: datetime.datetime
    added_by: UUID | Unset = UNSET
    expires_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        user = self.user.to_dict()

        added_at = self.added_at.isoformat()

        added_by: str | Unset = UNSET
        if not isinstance(self.added_by, Unset):
            added_by = str(self.added_by)

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "user": user,
                "added_at": added_at,
            }
        )
        if added_by is not UNSET:
            field_dict["added_by"] = added_by
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        user = PrincipalRef.from_dict(d.pop("user"))

        added_at = datetime.datetime.fromisoformat(d.pop("added_at"))

        _added_by = d.pop("added_by", UNSET)
        added_by: UUID | Unset
        if isinstance(_added_by, Unset):
            added_by = UNSET
        else:
            added_by = UUID(_added_by)

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        group_member = cls(
            user=user,
            added_at=added_at,
            added_by=added_by,
            expires_at=expires_at,
        )

        group_member.additional_properties = d
        return group_member

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
