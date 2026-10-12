from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.enrollment_code_purpose import EnrollmentCodePurpose

T = TypeVar("T", bound="EnrollmentCode")


@_attrs_define
class EnrollmentCode:
    """One-time code shown once to the owner, handed to the person out of band. A replay of the creating request with the
    same `Idempotency-Key` does not repeat it (`409 conflict`, `reason: one_time_secret`).

        Attributes:
            code (str):
            purpose (EnrollmentCodePurpose):
            expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    code: str
    purpose: EnrollmentCodePurpose
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        code = self.code

        purpose = self.purpose.value

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "code": code,
                "purpose": purpose,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        code = d.pop("code")

        purpose = EnrollmentCodePurpose(d.pop("purpose"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        enrollment_code = cls(
            code=code,
            purpose=purpose,
            expires_at=expires_at,
        )

        enrollment_code.additional_properties = d
        return enrollment_code

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
