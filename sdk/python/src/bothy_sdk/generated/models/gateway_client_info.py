from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.device_platform import DevicePlatform

T = TypeVar("T", bound="GatewayClientInfo")


@_attrs_define
class GatewayClientInfo:
    """
    Attributes:
        platform (DevicePlatform):
        version (str):
    """

    platform: DevicePlatform
    version: str

    def to_dict(self) -> dict[str, Any]:
        platform = self.platform.value

        version = self.version

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "platform": platform,
                "version": version,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        platform = DevicePlatform(d.pop("platform"))

        version = d.pop("version")

        gateway_client_info = cls(
            platform=platform,
            version=version,
        )

        return gateway_client_info
