from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.device_platform import DevicePlatform
from ..models.device_registration_alg import DeviceRegistrationAlg
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.device_key_registration import DeviceKeyRegistration
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="DeviceRegistration")


@_attrs_define
class DeviceRegistration:
    """Device and device keys sent with every login completion. The DPoP proof of the request must be signed with the
    `dpop` key (`public_jwk`); `step_up_key` is the second key with user verification (08, 6.1), absent where no secure
    key store exists.

        Attributes:
            device_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            name (str):
            platform (DevicePlatform):
            public_jwk (JsonObject): Free-form JSON object whose schema is named in the field description.
            alg (DeviceRegistrationAlg):
            app_version (str | Unset):
            attestation (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
            step_up_key (DeviceKeyRegistration | Unset):
    """

    device_id: UUID
    name: str
    platform: DevicePlatform
    public_jwk: JsonObject
    alg: DeviceRegistrationAlg
    app_version: str | Unset = UNSET
    attestation: JsonObject | Unset = UNSET
    step_up_key: DeviceKeyRegistration | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        device_id = str(self.device_id)

        name = self.name

        platform = self.platform.value

        public_jwk = self.public_jwk.to_dict()

        alg = self.alg.value

        app_version = self.app_version

        attestation: dict[str, Any] | Unset = UNSET
        if not isinstance(self.attestation, Unset):
            attestation = self.attestation.to_dict()

        step_up_key: dict[str, Any] | Unset = UNSET
        if not isinstance(self.step_up_key, Unset):
            step_up_key = self.step_up_key.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "device_id": device_id,
                "name": name,
                "platform": platform,
                "public_jwk": public_jwk,
                "alg": alg,
            }
        )
        if app_version is not UNSET:
            field_dict["app_version"] = app_version
        if attestation is not UNSET:
            field_dict["attestation"] = attestation
        if step_up_key is not UNSET:
            field_dict["step_up_key"] = step_up_key

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device_key_registration import DeviceKeyRegistration
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        device_id = UUID(d.pop("device_id"))

        name = d.pop("name")

        platform = DevicePlatform(d.pop("platform"))

        public_jwk = JsonObject.from_dict(d.pop("public_jwk"))

        alg = DeviceRegistrationAlg(d.pop("alg"))

        app_version = d.pop("app_version", UNSET)

        _attestation = d.pop("attestation", UNSET)
        attestation: JsonObject | Unset
        if isinstance(_attestation, Unset):
            attestation = UNSET
        else:
            attestation = JsonObject.from_dict(_attestation)

        _step_up_key = d.pop("step_up_key", UNSET)
        step_up_key: DeviceKeyRegistration | Unset
        if isinstance(_step_up_key, Unset):
            step_up_key = UNSET
        else:
            step_up_key = DeviceKeyRegistration.from_dict(_step_up_key)

        device_registration = cls(
            device_id=device_id,
            name=name,
            platform=platform,
            public_jwk=public_jwk,
            alg=alg,
            app_version=app_version,
            attestation=attestation,
            step_up_key=step_up_key,
        )

        return device_registration
