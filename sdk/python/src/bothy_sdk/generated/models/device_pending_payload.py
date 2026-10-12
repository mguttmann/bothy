from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.device_platform import DevicePlatform

T = TypeVar("T", bound="DevicePendingPayload")


@_attrs_define
class DevicePendingPayload:
    """A new device of the human waits for approval (R-AZ-31). Carries only IDs and the platform.

    Attributes:
        device_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        platform (DevicePlatform):
    """

    device_id: UUID
    platform: DevicePlatform
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        device_id = str(self.device_id)

        platform = self.platform.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "device_id": device_id,
                "platform": platform,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        device_id = UUID(d.pop("device_id"))

        platform = DevicePlatform(d.pop("platform"))

        device_pending_payload = cls(
            device_id=device_id,
            platform=platform,
        )

        device_pending_payload.additional_properties = d
        return device_pending_payload

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
