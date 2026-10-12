from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="HostPairingCode")


@_attrs_define
class HostPairingCode:
    """The pairing code is shown exactly once; the server stores only its HMAC.

    Attributes:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        pairing_code (str):
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    host_id: UUID
    pairing_code: str
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        host_id = str(self.host_id)

        pairing_code = self.pairing_code

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "host_id": host_id,
                "pairing_code": pairing_code,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        host_id = UUID(d.pop("host_id"))

        pairing_code = d.pop("pairing_code")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        host_pairing_code = cls(
            host_id=host_id,
            pairing_code=pairing_code,
            expires_at=expires_at,
        )

        host_pairing_code.additional_properties = d
        return host_pairing_code

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
