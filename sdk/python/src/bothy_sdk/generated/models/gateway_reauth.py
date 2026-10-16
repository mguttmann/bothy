from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="GatewayReauth")


@_attrs_define
class GatewayReauth:
    """
    Attributes:
        access_token (str):
        dpop (str):
    """

    access_token: str
    dpop: str

    def to_dict(self) -> dict[str, Any]:
        access_token = self.access_token

        dpop = self.dpop

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "access_token": access_token,
                "dpop": dpop,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        access_token = d.pop("access_token")

        dpop = d.pop("dpop")

        gateway_reauth = cls(
            access_token=access_token,
            dpop=dpop,
        )

        return gateway_reauth
