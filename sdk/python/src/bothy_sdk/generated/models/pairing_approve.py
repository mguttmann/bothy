from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="PairingApprove")


@_attrs_define
class PairingApprove:
    """
    Attributes:
        pairing_code (str):
    """

    pairing_code: str

    def to_dict(self) -> dict[str, Any]:
        pairing_code = self.pairing_code

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "pairing_code": pairing_code,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        pairing_code = d.pop("pairing_code")

        pairing_approve = cls(
            pairing_code=pairing_code,
        )

        return pairing_approve
