from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.login_client import LoginClient
from ..types import UNSET, Unset

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
        client (LoginClient | Unset): `console`: no `device` or DPoP; cookie, no token (08, 7.5).
        device (DeviceRegistration | Unset): Device and device keys sent with every login completion. The DPoP proof of
            the request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    login_name: str
    password: str
    totp_code: str
    client: LoginClient | Unset = UNSET
    device: DeviceRegistration | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        login_name = self.login_name

        password = self.password

        totp_code = self.totp_code

        client: str | Unset = UNSET
        if not isinstance(self.client, Unset):
            client = self.client.value

        device: dict[str, Any] | Unset = UNSET
        if not isinstance(self.device, Unset):
            device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "login_name": login_name,
                "password": password,
                "totp_code": totp_code,
            }
        )
        if client is not UNSET:
            field_dict["client"] = client
        if device is not UNSET:
            field_dict["device"] = device

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device_registration import DeviceRegistration

        d = dict(src_dict)
        login_name = d.pop("login_name")

        password = d.pop("password")

        totp_code = d.pop("totp_code")

        _client = d.pop("client", UNSET)
        client: LoginClient | Unset
        if isinstance(_client, Unset):
            client = UNSET
        else:
            client = LoginClient(_client)

        _device = d.pop("device", UNSET)
        device: DeviceRegistration | Unset
        if isinstance(_device, Unset):
            device = UNSET
        else:
            device = DeviceRegistration.from_dict(_device)

        password_login_request = cls(
            login_name=login_name,
            password=password,
            totp_code=totp_code,
            client=client,
            device=device,
        )

        return password_login_request
