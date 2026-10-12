from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="HostCreate")


@_attrs_define
class HostCreate:
    """
    Attributes:
        name (str):
        reason (str):
        assigned_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    name: str
    reason: str
    assigned_user_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        reason = self.reason

        assigned_user_id: str | Unset = UNSET
        if not isinstance(self.assigned_user_id, Unset):
            assigned_user_id = str(self.assigned_user_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "reason": reason,
            }
        )
        if assigned_user_id is not UNSET:
            field_dict["assigned_user_id"] = assigned_user_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        reason = d.pop("reason")

        _assigned_user_id = d.pop("assigned_user_id", UNSET)
        assigned_user_id: UUID | Unset
        if isinstance(_assigned_user_id, Unset):
            assigned_user_id = UNSET
        else:
            assigned_user_id = UUID(_assigned_user_id)

        host_create = cls(
            name=name,
            reason=reason,
            assigned_user_id=assigned_user_id,
        )

        return host_create
