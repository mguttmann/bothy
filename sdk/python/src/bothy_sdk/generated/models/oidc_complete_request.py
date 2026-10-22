from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.login_client import LoginClient
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.device_registration import DeviceRegistration


T = TypeVar("T", bound="OidcCompleteRequest")


@_attrs_define
class OidcCompleteRequest:
    """
    Attributes:
        login_code (str): One-time code the server appended to `app_redirect_uri` after the provider callback.
        client_verifier (str):
        client (LoginClient | Unset): `console`: no `device` or DPoP; cookie, no token (08, 7.5).
        device (DeviceRegistration | Unset): Device and device keys sent with every login completion. The DPoP proof of
            the request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    login_code: str
    client_verifier: str
    client: LoginClient | Unset = UNSET
    device: DeviceRegistration | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        login_code = self.login_code

        client_verifier = self.client_verifier

        client: str | Unset = UNSET
        if not isinstance(self.client, Unset):
            client = self.client.value

        device: dict[str, Any] | Unset = UNSET
        if not isinstance(self.device, Unset):
            device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "login_code": login_code,
                "client_verifier": client_verifier,
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
        login_code = d.pop("login_code")

        client_verifier = d.pop("client_verifier")

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

        oidc_complete_request = cls(
            login_code=login_code,
            client_verifier=client_verifier,
            client=client,
            device=device,
        )

        return oidc_complete_request
