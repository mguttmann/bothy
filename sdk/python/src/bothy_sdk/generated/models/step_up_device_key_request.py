from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="StepUpDeviceKeyRequest")


@_attrs_define
class StepUpDeviceKeyRequest:
    """
    Attributes:
        challenge_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        signature (str): Compact JWS signed by the active `step_up` key of the session's device after user verification
            on the device. Header `alg` only (no `jwk`, `jku`, `x5u`, `x5c`, `crit`); payload `{"challenge": <challenge>,
            "sid": <session id>, "iat": <NumericDate>}` (08, 6.2).
    """

    challenge_id: UUID
    signature: str

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        signature = self.signature

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "challenge_id": challenge_id,
                "signature": signature,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        challenge_id = UUID(d.pop("challenge_id"))

        signature = d.pop("signature")

        step_up_device_key_request = cls(
            challenge_id=challenge_id,
            signature=signature,
        )

        return step_up_device_key_request
