from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.device_registration import DeviceRegistration


T = TypeVar("T", bound="PasswordLoginRequest")


@_attrs_define
class PasswordLoginRequest:
    """
    Attributes:
        login_name (str):
        password (str):
        totp_code (str):
        device (DeviceRegistration): Device and device keys sent with every login completion. The DPoP proof of the
            request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    login_name: str
    password: str
    totp_code: str
    device: DeviceRegistration

    def to_dict(self) -> dict[str, Any]:
        login_name = self.login_name

        password = self.password

        totp_code = self.totp_code

        device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "login_name": login_name,
                "password": password,
                "totp_code": totp_code,
                "device": device,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device_registration import DeviceRegistration

        d = dict(src_dict)
        login_name = d.pop("login_name")

        password = d.pop("password")

        totp_code = d.pop("totp_code")

        device = DeviceRegistration.from_dict(d.pop("device"))

        password_login_request = cls(
            login_name=login_name,
            password=password,
            totp_code=totp_code,
            device=device,
        )

        return password_login_request
