from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="DeviceApprove")


@_attrs_define
class DeviceApprove:
    """
    Attributes:
        verification_code (str): Code shown on the pending device, typed by the approving person.
    """

    verification_code: str

    def to_dict(self) -> dict[str, Any]:
        verification_code = self.verification_code

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "verification_code": verification_code,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        verification_code = d.pop("verification_code")

        device_approve = cls(
            verification_code=verification_code,
        )

        return device_approve
