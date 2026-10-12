from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.host_doctor_check_check import HostDoctorCheckCheck
from ..types import UNSET, Unset

T = TypeVar("T", bound="HostDoctorCheck")


@_attrs_define
class HostDoctorCheck:
    """Result of one self-check of the host service (`host_doctor.v1`, checks HD1 to HD8 in 24, 2.2). Codes only.

    Attributes:
        check (HostDoctorCheckCheck):
        ok (bool):
        code (str | Unset):
    """

    check: HostDoctorCheckCheck
    ok: bool
    code: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        check = self.check.value

        ok = self.ok

        code = self.code

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "check": check,
                "ok": ok,
            }
        )
        if code is not UNSET:
            field_dict["code"] = code

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        check = HostDoctorCheckCheck(d.pop("check"))

        ok = d.pop("ok")

        code = d.pop("code", UNSET)

        host_doctor_check = cls(
            check=check,
            ok=ok,
            code=code,
        )

        host_doctor_check.additional_properties = d
        return host_doctor_check

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
