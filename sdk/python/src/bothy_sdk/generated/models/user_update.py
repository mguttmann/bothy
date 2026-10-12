from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="UserUpdate")


@_attrs_define
class UserUpdate:
    """
    Attributes:
        display_name (str | Unset):
        email (str | Unset):
        locale (str | Unset):
        time_zone (str | Unset):
    """

    display_name: str | Unset = UNSET
    email: str | Unset = UNSET
    locale: str | Unset = UNSET
    time_zone: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        display_name = self.display_name

        email = self.email

        locale = self.locale

        time_zone = self.time_zone

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if display_name is not UNSET:
            field_dict["display_name"] = display_name
        if email is not UNSET:
            field_dict["email"] = email
        if locale is not UNSET:
            field_dict["locale"] = locale
        if time_zone is not UNSET:
            field_dict["time_zone"] = time_zone

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        display_name = d.pop("display_name", UNSET)

        email = d.pop("email", UNSET)

        locale = d.pop("locale", UNSET)

        time_zone = d.pop("time_zone", UNSET)

        user_update = cls(
            display_name=display_name,
            email=email,
            locale=locale,
            time_zone=time_zone,
        )

        return user_update
