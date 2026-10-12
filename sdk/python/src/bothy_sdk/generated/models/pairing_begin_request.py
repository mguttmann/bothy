from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.device_registration import DeviceRegistration


T = TypeVar("T", bound="PairingBeginRequest")


@_attrs_define
class PairingBeginRequest:
    """
    Attributes:
        device (DeviceRegistration): Device and device keys sent with every login completion. The DPoP proof of the
            request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    device: DeviceRegistration

    def to_dict(self) -> dict[str, Any]:
        device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "device": device,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device_registration import DeviceRegistration

        d = dict(src_dict)
        device = DeviceRegistration.from_dict(d.pop("device"))

        pairing_begin_request = cls(
            device=device,
        )

        return pairing_begin_request
