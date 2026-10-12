from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.device_platform import DevicePlatform
from ..types import UNSET, Unset

T = TypeVar("T", bound="ClientVersionRequirement")


@_attrs_define
class ClientVersionRequirement:
    """
    Attributes:
        platform (DevicePlatform):
        min_version (str): Below this version the app does not connect (`client.min_version.<platform>`, 18 section 9).
        recommended_version (str | Unset): Below this version the app shows an update note
            (`client.recommended_version.<platform>`).
    """

    platform: DevicePlatform
    min_version: str
    recommended_version: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        platform = self.platform.value

        min_version = self.min_version

        recommended_version = self.recommended_version

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "platform": platform,
                "min_version": min_version,
            }
        )
        if recommended_version is not UNSET:
            field_dict["recommended_version"] = recommended_version

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        platform = DevicePlatform(d.pop("platform"))

        min_version = d.pop("min_version")

        recommended_version = d.pop("recommended_version", UNSET)

        client_version_requirement = cls(
            platform=platform,
            min_version=min_version,
            recommended_version=recommended_version,
        )

        client_version_requirement.additional_properties = d
        return client_version_requirement

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
