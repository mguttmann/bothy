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


T = TypeVar("T", bound="Owner")


@_attrs_define
class Owner:
    """
    Attributes:
        user (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        granted_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        reason (str):
        granted_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    user: PrincipalRef
    granted_at: datetime.datetime
    reason: str
    granted_by: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        user = self.user.to_dict()

        granted_at = self.granted_at.isoformat()

        reason = self.reason

        granted_by: str | Unset = UNSET
        if not isinstance(self.granted_by, Unset):
            granted_by = str(self.granted_by)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "user": user,
                "granted_at": granted_at,
                "reason": reason,
            }
        )
        if granted_by is not UNSET:
            field_dict["granted_by"] = granted_by

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        user = PrincipalRef.from_dict(d.pop("user"))

        granted_at = datetime.datetime.fromisoformat(d.pop("granted_at"))

        reason = d.pop("reason")

        _granted_by = d.pop("granted_by", UNSET)
        granted_by: UUID | Unset
        if isinstance(_granted_by, Unset):
            granted_by = UNSET
        else:
            granted_by = UUID(_granted_by)

        owner = cls(
            user=user,
            granted_at=granted_at,
            reason=reason,
            granted_by=granted_by,
        )

        owner.additional_properties = d
        return owner

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
