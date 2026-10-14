from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="PasskeyLoginBeginRequest")


@_attrs_define
class PasskeyLoginBeginRequest:
    """
    Attributes:
        login_name (str | Unset): Optional; without it the server offers discoverable credentials.
    """

    login_name: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        login_name = self.login_name

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if login_name is not UNSET:
            field_dict["login_name"] = login_name

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        login_name = d.pop("login_name", UNSET)

        passkey_login_begin_request = cls(
            login_name=login_name,
        )

        return passkey_login_begin_request
