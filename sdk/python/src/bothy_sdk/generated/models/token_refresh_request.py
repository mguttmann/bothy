from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="TokenRefreshRequest")


@_attrs_define
class TokenRefreshRequest:
    """
    Attributes:
        refresh_token (str):
    """

    refresh_token: str

    def to_dict(self) -> dict[str, Any]:
        refresh_token = self.refresh_token

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "refresh_token": refresh_token,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        refresh_token = d.pop("refresh_token")

        token_refresh_request = cls(
            refresh_token=refresh_token,
        )

        return token_refresh_request
