from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="HostUpdate")


@_attrs_define
class HostUpdate:
    """Members that are absent stay unchanged. To remove the assignee send `clear_assigned_user` (no JSON null, ADR 0053).

    Attributes:
        reason (str):
        name (str | Unset):
        assigned_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        clear_assigned_user (bool | Unset):
    """

    reason: str
    name: str | Unset = UNSET
    assigned_user_id: UUID | Unset = UNSET
    clear_assigned_user: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        reason = self.reason

        name = self.name

        assigned_user_id: str | Unset = UNSET
        if not isinstance(self.assigned_user_id, Unset):
            assigned_user_id = str(self.assigned_user_id)

        clear_assigned_user = self.clear_assigned_user

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "reason": reason,
            }
        )
        if name is not UNSET:
            field_dict["name"] = name
        if assigned_user_id is not UNSET:
            field_dict["assigned_user_id"] = assigned_user_id
        if clear_assigned_user is not UNSET:
            field_dict["clear_assigned_user"] = clear_assigned_user

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        reason = d.pop("reason")

        name = d.pop("name", UNSET)

        _assigned_user_id = d.pop("assigned_user_id", UNSET)
        assigned_user_id: UUID | Unset
        if isinstance(_assigned_user_id, Unset):
            assigned_user_id = UNSET
        else:
            assigned_user_id = UUID(_assigned_user_id)

        clear_assigned_user = d.pop("clear_assigned_user", UNSET)

        host_update = cls(
            reason=reason,
            name=name,
            assigned_user_id=assigned_user_id,
            clear_assigned_user=clear_assigned_user,
        )

        return host_update
