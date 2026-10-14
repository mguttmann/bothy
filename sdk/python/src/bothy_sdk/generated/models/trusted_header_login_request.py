from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.device_registration import DeviceRegistration


T = TypeVar("T", bound="TrustedHeaderLoginRequest")


@_attrs_define
class TrustedHeaderLoginRequest:
    """
    Attributes:
        device (DeviceRegistration): Device and device keys sent with every login completion. The DPoP proof of the
            request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
        enrollment_code (str | Unset): Optional enrollment code; links an unknown header login to the person of the code
            (08, 3.1).
    """

    device: DeviceRegistration
    enrollment_code: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        device = self.device.to_dict()

        enrollment_code = self.enrollment_code

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "device": device,
            }
        )
        if enrollment_code is not UNSET:
            field_dict["enrollment_code"] = enrollment_code

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device_registration import DeviceRegistration

        d = dict(src_dict)
        device = DeviceRegistration.from_dict(d.pop("device"))

        enrollment_code = d.pop("enrollment_code", UNSET)

        trusted_header_login_request = cls(
            device=device,
            enrollment_code=enrollment_code,
        )

        return trusted_header_login_request
