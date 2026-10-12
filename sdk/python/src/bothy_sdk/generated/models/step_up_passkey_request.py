from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="StepUpPasskeyRequest")


@_attrs_define
class StepUpPasskeyRequest:
    """
    Attributes:
        challenge_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        credential (JsonObject): Free-form JSON object whose schema is named in the field description.
    """

    challenge_id: UUID
    credential: JsonObject

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        credential = self.credential.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "challenge_id": challenge_id,
                "credential": credential,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        challenge_id = UUID(d.pop("challenge_id"))

        credential = JsonObject.from_dict(d.pop("credential"))

        step_up_passkey_request = cls(
            challenge_id=challenge_id,
            credential=credential,
        )

        return step_up_passkey_request
