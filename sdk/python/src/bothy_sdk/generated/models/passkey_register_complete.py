from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="PasskeyRegisterComplete")


@_attrs_define
class PasskeyRegisterComplete:
    """
    Attributes:
        challenge_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        credential (JsonObject): Free-form JSON object whose schema is named in the field description.
        name (str):
    """

    challenge_id: UUID
    credential: JsonObject
    name: str

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        credential = self.credential.to_dict()

        name = self.name

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "challenge_id": challenge_id,
                "credential": credential,
                "name": name,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        challenge_id = UUID(d.pop("challenge_id"))

        credential = JsonObject.from_dict(d.pop("credential"))

        name = d.pop("name")

        passkey_register_complete = cls(
            challenge_id=challenge_id,
            credential=credential,
            name=name,
        )

        return passkey_register_complete
