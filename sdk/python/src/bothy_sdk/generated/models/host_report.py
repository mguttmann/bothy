from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.host_doctor_check import HostDoctorCheck
    from ..models.host_profile import HostProfile
    from ..models.host_reported_session_user import HostReportedSessionUser


T = TypeVar("T", bound="HostReport")


@_attrs_define
class HostReport:
    """Report of profiles, session users and self-checks; never content, paths or values. Sent after every `ready` before
    the host service accepts `session.open`, and on every change. When `doctor` refuses the start, the host service
    still logs in, sends one `report` and closes the connection without accepting `session.open` (24 section 2.2).

        Attributes:
            profiles (list[HostProfile]):
            session_users (list[HostReportedSessionUser]):
            doctor (list[HostDoctorCheck]): One entry per check HD1 to HD11; a failed HD6, HD9, HD10 or HD11 has one entry
                per account named by the rule for names (24, 2.2).
            is_server_host (bool):
            require_local_confirm (bool):
            os_version (str | Unset):
    """

    profiles: list[HostProfile]
    session_users: list[HostReportedSessionUser]
    doctor: list[HostDoctorCheck]
    is_server_host: bool
    require_local_confirm: bool
    os_version: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        profiles = []
        for profiles_item_data in self.profiles:
            profiles_item = profiles_item_data.to_dict()
            profiles.append(profiles_item)

        session_users = []
        for session_users_item_data in self.session_users:
            session_users_item = session_users_item_data.to_dict()
            session_users.append(session_users_item)

        doctor = []
        for doctor_item_data in self.doctor:
            doctor_item = doctor_item_data.to_dict()
            doctor.append(doctor_item)

        is_server_host = self.is_server_host

        require_local_confirm = self.require_local_confirm

        os_version = self.os_version

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "profiles": profiles,
                "session_users": session_users,
                "doctor": doctor,
                "is_server_host": is_server_host,
                "require_local_confirm": require_local_confirm,
            }
        )
        if os_version is not UNSET:
            field_dict["os_version"] = os_version

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_doctor_check import HostDoctorCheck
        from ..models.host_profile import HostProfile
        from ..models.host_reported_session_user import HostReportedSessionUser

        d = dict(src_dict)
        profiles = []
        _profiles = d.pop("profiles")
        for profiles_item_data in _profiles:
            profiles_item = HostProfile.from_dict(profiles_item_data)

            profiles.append(profiles_item)

        session_users = []
        _session_users = d.pop("session_users")
        for session_users_item_data in _session_users:
            session_users_item = HostReportedSessionUser.from_dict(
                session_users_item_data
            )

            session_users.append(session_users_item)

        doctor = []
        _doctor = d.pop("doctor")
        for doctor_item_data in _doctor:
            doctor_item = HostDoctorCheck.from_dict(doctor_item_data)

            doctor.append(doctor_item)

        is_server_host = d.pop("is_server_host")

        require_local_confirm = d.pop("require_local_confirm")

        os_version = d.pop("os_version", UNSET)

        host_report = cls(
            profiles=profiles,
            session_users=session_users,
            doctor=doctor,
            is_server_host=is_server_host,
            require_local_confirm=require_local_confirm,
            os_version=os_version,
        )

        host_report.additional_properties = d
        return host_report

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
