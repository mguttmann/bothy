from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.device_registration import DeviceRegistration
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="EnrollCompleteRequest")


@_attrs_define
class EnrollCompleteRequest:
    """
    Attributes:
        challenge_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        credential (JsonObject): Free-form JSON object whose schema is named in the field description.
        passkey_name (str):
        device (DeviceRegistration): Device and device keys sent with every login completion. The DPoP proof of the
            request must be signed with the `dpop` key (`public_jwk`); `step_up_key` is the second key with user
            verification (08, 6.1), absent where no secure key store exists.
    """

    challenge_id: UUID
    credential: JsonObject
    passkey_name: str
    device: DeviceRegistration

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        credential = self.credential.to_dict()

        passkey_name = self.passkey_name

        device = self.device.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "challenge_id": challenge_id,
                "credential": credential,
                "passkey_name": passkey_name,
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

        passkey_name = d.pop("passkey_name")

        device = DeviceRegistration.from_dict(d.pop("device"))

        enroll_complete_request = cls(
            challenge_id=challenge_id,
            credential=credential,
            passkey_name=passkey_name,
            device=device,
        )

        return enroll_complete_request
