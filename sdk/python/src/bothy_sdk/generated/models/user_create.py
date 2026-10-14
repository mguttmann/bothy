from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="UserCreate")


@_attrs_define
class UserCreate:
    """
    Attributes:
        handle (str):
        display_name (str):
        email (str | Unset):
        locale (str | Unset):
        time_zone (str | Unset):
        local_account (bool | Unset): Create an enrollment code for the local identity provider.
    """

    handle: str
    display_name: str
    email: str | Unset = UNSET
    locale: str | Unset = UNSET
    time_zone: str | Unset = UNSET
    local_account: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        handle = self.handle

        display_name = self.display_name

        email = self.email

        locale = self.locale

        time_zone = self.time_zone

        local_account = self.local_account

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "handle": handle,
                "display_name": display_name,
            }
        )
        if email is not UNSET:
            field_dict["email"] = email
        if locale is not UNSET:
            field_dict["locale"] = locale
        if time_zone is not UNSET:
            field_dict["time_zone"] = time_zone
        if local_account is not UNSET:
            field_dict["local_account"] = local_account

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        handle = d.pop("handle")

        display_name = d.pop("display_name")

        email = d.pop("email", UNSET)

        locale = d.pop("locale", UNSET)

        time_zone = d.pop("time_zone", UNSET)

        local_account = d.pop("local_account", UNSET)

        user_create = cls(
            handle=handle,
            display_name=display_name,
            email=email,
            locale=locale,
            time_zone=time_zone,
            local_account=local_account,
        )

        return user_create
