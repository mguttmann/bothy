from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.user_status import UserStatus
from ..types import UNSET, Unset

T = TypeVar("T", bound="User")


@_attrs_define
class User:
    """Directory entry of a person.

    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        handle (str):
        display_name (str):
        status (UserStatus):
        is_owner (bool):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        avatar_file_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        locale (str | Unset):
        time_zone (str | Unset):
    """

    id: UUID
    handle: str
    display_name: str
    status: UserStatus
    is_owner: bool
    created_at: datetime.datetime
    avatar_file_id: UUID | Unset = UNSET
    locale: str | Unset = UNSET
    time_zone: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        handle = self.handle

        display_name = self.display_name

        status = self.status.value

        is_owner = self.is_owner

        created_at = self.created_at.isoformat()

        avatar_file_id: str | Unset = UNSET
        if not isinstance(self.avatar_file_id, Unset):
            avatar_file_id = str(self.avatar_file_id)

        locale = self.locale

        time_zone = self.time_zone

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "handle": handle,
                "display_name": display_name,
                "status": status,
                "is_owner": is_owner,
                "created_at": created_at,
            }
        )
        if avatar_file_id is not UNSET:
            field_dict["avatar_file_id"] = avatar_file_id
        if locale is not UNSET:
            field_dict["locale"] = locale
        if time_zone is not UNSET:
            field_dict["time_zone"] = time_zone

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        handle = d.pop("handle")

        display_name = d.pop("display_name")

        status = UserStatus(d.pop("status"))

        is_owner = d.pop("is_owner")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _avatar_file_id = d.pop("avatar_file_id", UNSET)
        avatar_file_id: UUID | Unset
        if isinstance(_avatar_file_id, Unset):
            avatar_file_id = UNSET
        else:
            avatar_file_id = UUID(_avatar_file_id)

        locale = d.pop("locale", UNSET)

        time_zone = d.pop("time_zone", UNSET)

        user = cls(
            id=id,
            handle=handle,
            display_name=display_name,
            status=status,
            is_owner=is_owner,
            created_at=created_at,
            avatar_file_id=avatar_file_id,
            locale=locale,
            time_zone=time_zone,
        )

        user.additional_properties = d
        return user

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
