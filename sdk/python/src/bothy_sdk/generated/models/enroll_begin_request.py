from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="EnrollBeginRequest")


@_attrs_define
class EnrollBeginRequest:
    """
    Attributes:
        enrollment_code (str):
    """

    enrollment_code: str

    def to_dict(self) -> dict[str, Any]:
        enrollment_code = self.enrollment_code

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "enrollment_code": enrollment_code,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        enrollment_code = d.pop("enrollment_code")

        enroll_begin_request = cls(
            enrollment_code=enrollment_code,
        )

        return enroll_begin_request
