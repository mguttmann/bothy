from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.login_client import LoginClient
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.device_registration import DeviceRegistration
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="PasskeyLoginCompleteRequest")


@_attrs_define
class PasskeyLoginCompleteRequest:
    """
    Attributes:
        challenge_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        credential (JsonObject): Free-form JSON object whose schema is named in the field description.
        client (LoginClient | Unset): `console`: no `device` or DPoP; cookie, no token (08, 7.5).
        device (DeviceRegistration | Unset): Device and device keys sent with every login completion. The DPoP proof of
            the request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    challenge_id: UUID
    credential: JsonObject
    client: LoginClient | Unset = UNSET
    device: DeviceRegistration | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        credential = self.credential.to_dict()

        client: str | Unset = UNSET
        if not isinstance(self.client, Unset):
            client = self.client.value

        device: dict[str, Any] | Unset = UNSET
        if not isinstance(self.device, Unset):
            device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "challenge_id": challenge_id,
                "credential": credential,
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
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        challenge_id = UUID(d.pop("challenge_id"))

        credential = JsonObject.from_dict(d.pop("credential"))

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

        passkey_login_complete_request = cls(
            challenge_id=challenge_id,
            credential=credential,
            client=client,
            device=device,
        )

        return passkey_login_complete_request
