from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="WebAuthnChallenge")


@_attrs_define
class WebAuthnChallenge:
    """
    Attributes:
        challenge_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        public_key (JsonObject): Free-form JSON object whose schema is named in the field description.
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    challenge_id: UUID
    public_key: JsonObject
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        public_key = self.public_key.to_dict()

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "challenge_id": challenge_id,
                "public_key": public_key,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        challenge_id = UUID(d.pop("challenge_id"))

        public_key = JsonObject.from_dict(d.pop("public_key"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        web_authn_challenge = cls(
            challenge_id=challenge_id,
            public_key=public_key,
            expires_at=expires_at,
        )

        web_authn_challenge.additional_properties = d
        return web_authn_challenge

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
