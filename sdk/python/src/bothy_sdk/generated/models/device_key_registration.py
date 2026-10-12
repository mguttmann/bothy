from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.device_key_registration_alg import DeviceKeyRegistrationAlg
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="DeviceKeyRegistration")


@_attrs_define
class DeviceKeyRegistration:
    """
    Attributes:
        public_jwk (JsonObject): Free-form JSON object whose schema is named in the field description.
        alg (DeviceKeyRegistrationAlg):
        attestation (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
    """

    public_jwk: JsonObject
    alg: DeviceKeyRegistrationAlg
    attestation: JsonObject | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        public_jwk = self.public_jwk.to_dict()

        alg = self.alg.value

        attestation: dict[str, Any] | Unset = UNSET
        if not isinstance(self.attestation, Unset):
            attestation = self.attestation.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "public_jwk": public_jwk,
                "alg": alg,
            }
        )
        if attestation is not UNSET:
            field_dict["attestation"] = attestation

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        public_jwk = JsonObject.from_dict(d.pop("public_jwk"))

        alg = DeviceKeyRegistrationAlg(d.pop("alg"))

        _attestation = d.pop("attestation", UNSET)
        attestation: JsonObject | Unset
        if isinstance(_attestation, Unset):
            attestation = UNSET
        else:
            attestation = JsonObject.from_dict(_attestation)

        device_key_registration = cls(
            public_jwk=public_jwk,
            alg=alg,
            attestation=attestation,
        )

        return device_key_registration
