from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="HostSessionUser")


@_attrs_define
class HostSessionUser:
    """A valid assignment of a person to their own session user on the host (ruling T12, 24 section 2.2).

    Attributes:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        os_user (str):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        owner_account (bool | Unset): Present and `true` for an assignment of the owner account (ADR 0073): sessions run
            in the owner's own account with administrator rights; clients warn before the step-up.
    """

    user_id: UUID
    os_user: str
    created_at: datetime.datetime
    owner_account: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        user_id = str(self.user_id)

        os_user = self.os_user

        created_at = self.created_at.isoformat()

        owner_account = self.owner_account

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "user_id": user_id,
                "os_user": os_user,
                "created_at": created_at,
            }
        )
        if owner_account is not UNSET:
            field_dict["owner_account"] = owner_account

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        user_id = UUID(d.pop("user_id"))

        os_user = d.pop("os_user")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        owner_account = d.pop("owner_account", UNSET)

        host_session_user = cls(
            user_id=user_id,
            os_user=os_user,
            created_at=created_at,
            owner_account=owner_account,
        )

        host_session_user.additional_properties = d
        return host_session_user

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
