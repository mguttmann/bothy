from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define

T = TypeVar("T", bound="HostReportedSessionUser")


@_attrs_define
class HostReportedSessionUser:
    """A session user the host service reports (`host_reported_session_users.v1`, 24 section 2.2), with the display names
    of its own working directories of the `claude` profile (ruling T13). Names only, never paths.

        Attributes:
            os_user (str): Operating system user name from `host_service.session_users`.
            workdirs (list[str]): Display names of the working directories of this session user in
                `host_service.claude_workdirs` after check HD7, in their order (a directory listed under two session users is
                dropped from both lists); empty means no `claude` profile for this session user. A client sends the index into
                its own list, never a path; the host service resolves the index in exactly this list (24, 2.2 and 5.3).
    """

    os_user: str
    workdirs: list[str]

    def to_dict(self) -> dict[str, Any]:
        os_user = self.os_user

        workdirs = self.workdirs

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "os_user": os_user,
                "workdirs": workdirs,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        os_user = d.pop("os_user")

        workdirs = cast(list[str], d.pop("workdirs"))

        host_reported_session_user = cls(
            os_user=os_user,
            workdirs=workdirs,
        )

        return host_reported_session_user
