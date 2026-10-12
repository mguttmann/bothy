from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.device_registration import DeviceRegistration


T = TypeVar("T", bound="OidcCompleteRequest")


@_attrs_define
class OidcCompleteRequest:
    """
    Attributes:
        login_code (str): One-time code the server appended to `app_redirect_uri` after the provider callback.
        client_verifier (str):
        device (DeviceRegistration): Device and device keys sent with every login completion. The DPoP proof of the
            request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    login_code: str
    client_verifier: str
    device: DeviceRegistration

    def to_dict(self) -> dict[str, Any]:
        login_code = self.login_code

        client_verifier = self.client_verifier

        device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "login_code": login_code,
                "client_verifier": client_verifier,
                "device": device,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device_registration import DeviceRegistration

        d = dict(src_dict)
        login_code = d.pop("login_code")

        client_verifier = d.pop("client_verifier")

        device = DeviceRegistration.from_dict(d.pop("device"))

        oidc_complete_request = cls(
            login_code=login_code,
            client_verifier=client_verifier,
            device=device,
        )

        return oidc_complete_request
