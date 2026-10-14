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
    """Result of one self-check of the host service (`host_doctor.v1`, checks HD1 to HD11 in 24, 2.2). Codes and, following
    the rule for names (24, 2.2), the name of each account a person with a terminal can cause the finding in; never
    content, paths or values. When HD6 or HD11 fails, the list has one entry per affected session user; when HD10 fails
    with a wrong peer, one entry per session user of that channel; when HD9 fails, one entry per former session user
    whose home can be listed, plus one entry without a name for any other user; `check` and `session_user` together are
    unique in the list.

        Attributes:
            check (HostDoctorCheckCheck):
            ok (bool):
            code (str | Unset):
            session_user (str | Unset): Only when not passed: with `hd6_llm_login` and `hd11_shared_locations` the session
                user whose account the finding is in; with `hd10_helper_integrity` and a wrong peer the session user of that
                channel; with `hd9_foreign_homes` a former session user of this host (bound to a person in the state directory,
                no longer in `host_service.session_users`) whose home can be listed. The operator knows where to look, and the
                console shows the person of the assignment (24, 2.2 and 7.4). One entry per such account; names of other users
                are never sent. A name, never a path.
    """

    check: HostDoctorCheckCheck
    ok: bool
    code: str | Unset = UNSET
    session_user: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        check = self.check.value

        ok = self.ok

        code = self.code

        session_user = self.session_user

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
        if session_user is not UNSET:
            field_dict["session_user"] = session_user

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        check = HostDoctorCheckCheck(d.pop("check"))

        ok = d.pop("ok")

        code = d.pop("code", UNSET)

        session_user = d.pop("session_user", UNSET)

        host_doctor_check = cls(
            check=check,
            ok=ok,
            code=code,
            session_user=session_user,
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
