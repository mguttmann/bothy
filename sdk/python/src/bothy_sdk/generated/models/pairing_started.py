from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="PairingStarted")


@_attrs_define
class PairingStarted:
    """
    Attributes:
        pairing_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        pairing_code (str): Shown on the new device only; `auth.pairing_code_chars` characters (Crockford Base32).
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    pairing_id: UUID
    pairing_code: str
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        pairing_id = str(self.pairing_id)

        pairing_code = self.pairing_code

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "pairing_id": pairing_id,
                "pairing_code": pairing_code,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        pairing_id = UUID(d.pop("pairing_id"))

        pairing_code = d.pop("pairing_code")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        pairing_started = cls(
            pairing_id=pairing_id,
            pairing_code=pairing_code,
            expires_at=expires_at,
        )

        pairing_started.additional_properties = d
        return pairing_started

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
