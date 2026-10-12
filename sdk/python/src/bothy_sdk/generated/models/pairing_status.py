from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.pairing_status_status import PairingStatusStatus

T = TypeVar("T", bound="PairingStatus")


@_attrs_define
class PairingStatus:
    """
    Attributes:
        pairing_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        status (PairingStatusStatus): Not yet approved; ask again after a short wait.
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    pairing_id: UUID
    status: PairingStatusStatus
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        pairing_id = str(self.pairing_id)

        status = self.status.value

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "pairing_id": pairing_id,
                "status": status,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        pairing_id = UUID(d.pop("pairing_id"))

        status = PairingStatusStatus(d.pop("status"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        pairing_status = cls(
            pairing_id=pairing_id,
            status=status,
            expires_at=expires_at,
        )

        pairing_status.additional_properties = d
        return pairing_status

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
