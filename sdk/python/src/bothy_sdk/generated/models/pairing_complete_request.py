from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="PairingCompleteRequest")


@_attrs_define
class PairingCompleteRequest:
    """
    Attributes:
        pairing_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    pairing_id: UUID

    def to_dict(self) -> dict[str, Any]:
        pairing_id = str(self.pairing_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "pairing_id": pairing_id,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        pairing_id = UUID(d.pop("pairing_id"))

        pairing_complete_request = cls(
            pairing_id=pairing_id,
        )

        return pairing_complete_request
