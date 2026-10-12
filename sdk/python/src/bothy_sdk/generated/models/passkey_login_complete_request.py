from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

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
        device (DeviceRegistration): Device and device keys sent with every login completion. The DPoP proof of the
            request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    challenge_id: UUID
    credential: JsonObject
    device: DeviceRegistration

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        credential = self.credential.to_dict()

        device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "challenge_id": challenge_id,
                "credential": credential,
                "device": device,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device_registration import DeviceRegistration
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        challenge_id = UUID(d.pop("challenge_id"))

        credential = JsonObject.from_dict(d.pop("credential"))

        device = DeviceRegistration.from_dict(d.pop("device"))

        passkey_login_complete_request = cls(
            challenge_id=challenge_id,
            credential=credential,
            device=device,
        )

        return passkey_login_complete_request
