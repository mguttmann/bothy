from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.device_registration import DeviceRegistration


T = TypeVar("T", bound="EnrollRedeemRequest")


@_attrs_define
class EnrollRedeemRequest:
    """Redeems an enrollment code with device keys only; the redeeming device becomes `active` (08, 3.2).

    Attributes:
        enrollment_code (str):
        device (DeviceRegistration): Device and device keys sent with every login completion. The DPoP proof of the
            request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    enrollment_code: str
    device: DeviceRegistration

    def to_dict(self) -> dict[str, Any]:
        enrollment_code = self.enrollment_code

        device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "enrollment_code": enrollment_code,
                "device": device,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device_registration import DeviceRegistration

        d = dict(src_dict)
        enrollment_code = d.pop("enrollment_code")

        device = DeviceRegistration.from_dict(d.pop("device"))

        enroll_redeem_request = cls(
            enrollment_code=enrollment_code,
            device=device,
        )

        return enroll_redeem_request
