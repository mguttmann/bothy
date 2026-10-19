from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.host_disabled_reason import HostDisabledReason
from ..models.host_platform import HostPlatform
from ..models.host_state import HostState
from ..models.llm_credentials import LlmCredentials
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.host_doctor_check import HostDoctorCheck
    from ..models.host_profile import HostProfile
    from ..models.host_reported_session_user import HostReportedSessionUser
    from ..models.host_session_user import HostSessionUser


T = TypeVar("T", bound="Host")


@_attrs_define
class Host:
    """A connected host. Keys are public; the client pins `noise_public_key` per device (24, 5.5).

    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        state (HostState): Life cycle of a host (24, 3.1). Only `active` allows sessions.
        llm_credentials (LlmCredentials): Owner declaration of the LLM login on a host (24, 7.2). `personal` needs
            `credential_holder_id` (I6).
        declared_server_host (bool):
        key_generation (int):
        profiles (list[HostProfile]):
        doctor (list[HostDoctorCheck]): One entry per check HD1 to HD11; a failed HD6, HD9, HD10 or HD11 has one entry
            per account named by the rule for names (24, 2.2).
        connected (bool): The control connection of the host service is open (memory of the server).
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        version (int):
        disabled_reason (HostDisabledReason | Unset): Present exactly with `state = disabled`. `duplicate_connection`
            means a second live control connection with the same keys; only re-pairing helps (24, 3.4).
        platform (HostPlatform | Unset):
        assigned_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        credential_holder_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        reported_is_server_host (bool | Unset): Report of the host service (24, 2.3); absent before the first report.
        noise_public_key (str | Unset): Static X25519 key of the host, 32 bytes base64url without padding; absent while
            the host has no keys (before pairing and after a re-pairing or the fallback of `confirmHost`).
        key_fingerprint (str | Unset): Fingerprint of the stored keys (24, 4.3), 13 groups of four Crockford base32
            characters; absent while the host has no keys.
        session_users (list[HostSessionUser] | Unset): Valid session user assignments (ruling T12), visible like every
            other member to everyone with `host.read`. Absent when there is none.
        reported_session_users (list[HostReportedSessionUser] | Unset): Session users the host service reported at
            pairing and in its last `report` (24 section 2.2). An assignment works only while its name is in this list
            (SR12); `confirmHost` and `updateHost` accept only these names. Absent before pairing.
        require_local_confirm (bool | Unset): Report whether `host_service.require_local_confirm` is on.
        host_version (str | Unset):
        os_version (str | Unset):
        last_seen_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    name: str
    state: HostState
    llm_credentials: LlmCredentials
    declared_server_host: bool
    key_generation: int
    profiles: list[HostProfile]
    doctor: list[HostDoctorCheck]
    connected: bool
    created_at: datetime.datetime
    version: int
    disabled_reason: HostDisabledReason | Unset = UNSET
    platform: HostPlatform | Unset = UNSET
    assigned_user_id: UUID | Unset = UNSET
    credential_holder_id: UUID | Unset = UNSET
    reported_is_server_host: bool | Unset = UNSET
    noise_public_key: str | Unset = UNSET
    key_fingerprint: str | Unset = UNSET
    session_users: list[HostSessionUser] | Unset = UNSET
    reported_session_users: list[HostReportedSessionUser] | Unset = UNSET
    require_local_confirm: bool | Unset = UNSET
    host_version: str | Unset = UNSET
    os_version: str | Unset = UNSET
    last_seen_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        name = self.name

        state = self.state.value

        llm_credentials = self.llm_credentials.value

        declared_server_host = self.declared_server_host

        key_generation = self.key_generation

        profiles = []
        for profiles_item_data in self.profiles:
            profiles_item = profiles_item_data.to_dict()
            profiles.append(profiles_item)

        doctor = []
        for doctor_item_data in self.doctor:
            doctor_item = doctor_item_data.to_dict()
            doctor.append(doctor_item)

        connected = self.connected

        created_at = self.created_at.isoformat()

        version = self.version

        disabled_reason: str | Unset = UNSET
        if not isinstance(self.disabled_reason, Unset):
            disabled_reason = self.disabled_reason.value

        platform: str | Unset = UNSET
        if not isinstance(self.platform, Unset):
            platform = self.platform.value

        assigned_user_id: str | Unset = UNSET
        if not isinstance(self.assigned_user_id, Unset):
            assigned_user_id = str(self.assigned_user_id)

        credential_holder_id: str | Unset = UNSET
        if not isinstance(self.credential_holder_id, Unset):
            credential_holder_id = str(self.credential_holder_id)

        reported_is_server_host = self.reported_is_server_host

        noise_public_key = self.noise_public_key

        key_fingerprint = self.key_fingerprint

        session_users: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.session_users, Unset):
            session_users = []
            for session_users_item_data in self.session_users:
                session_users_item = session_users_item_data.to_dict()
                session_users.append(session_users_item)

        reported_session_users: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.reported_session_users, Unset):
            reported_session_users = []
            for reported_session_users_item_data in self.reported_session_users:
                reported_session_users_item = reported_session_users_item_data.to_dict()
                reported_session_users.append(reported_session_users_item)

        require_local_confirm = self.require_local_confirm

        host_version = self.host_version

        os_version = self.os_version

        last_seen_at: str | Unset = UNSET
        if not isinstance(self.last_seen_at, Unset):
            last_seen_at = self.last_seen_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "state": state,
                "llm_credentials": llm_credentials,
                "declared_server_host": declared_server_host,
                "key_generation": key_generation,
                "profiles": profiles,
                "doctor": doctor,
                "connected": connected,
                "created_at": created_at,
                "version": version,
            }
        )
        if disabled_reason is not UNSET:
            field_dict["disabled_reason"] = disabled_reason
        if platform is not UNSET:
            field_dict["platform"] = platform
        if assigned_user_id is not UNSET:
            field_dict["assigned_user_id"] = assigned_user_id
        if credential_holder_id is not UNSET:
            field_dict["credential_holder_id"] = credential_holder_id
        if reported_is_server_host is not UNSET:
            field_dict["reported_is_server_host"] = reported_is_server_host
        if noise_public_key is not UNSET:
            field_dict["noise_public_key"] = noise_public_key
        if key_fingerprint is not UNSET:
            field_dict["key_fingerprint"] = key_fingerprint
        if session_users is not UNSET:
            field_dict["session_users"] = session_users
        if reported_session_users is not UNSET:
            field_dict["reported_session_users"] = reported_session_users
        if require_local_confirm is not UNSET:
            field_dict["require_local_confirm"] = require_local_confirm
        if host_version is not UNSET:
            field_dict["host_version"] = host_version
        if os_version is not UNSET:
            field_dict["os_version"] = os_version
        if last_seen_at is not UNSET:
            field_dict["last_seen_at"] = last_seen_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_doctor_check import HostDoctorCheck
        from ..models.host_profile import HostProfile
        from ..models.host_reported_session_user import HostReportedSessionUser
        from ..models.host_session_user import HostSessionUser

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        name = d.pop("name")

        state = HostState(d.pop("state"))

        llm_credentials = LlmCredentials(d.pop("llm_credentials"))

        declared_server_host = d.pop("declared_server_host")

        key_generation = d.pop("key_generation")

        profiles = []
        _profiles = d.pop("profiles")
        for profiles_item_data in _profiles:
            profiles_item = HostProfile.from_dict(profiles_item_data)

            profiles.append(profiles_item)

        doctor = []
        _doctor = d.pop("doctor")
        for doctor_item_data in _doctor:
            doctor_item = HostDoctorCheck.from_dict(doctor_item_data)

            doctor.append(doctor_item)

        connected = d.pop("connected")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        version = d.pop("version")

        _disabled_reason = d.pop("disabled_reason", UNSET)
        disabled_reason: HostDisabledReason | Unset
        if isinstance(_disabled_reason, Unset):
            disabled_reason = UNSET
        else:
            disabled_reason = HostDisabledReason(_disabled_reason)

        _platform = d.pop("platform", UNSET)
        platform: HostPlatform | Unset
        if isinstance(_platform, Unset):
            platform = UNSET
        else:
            platform = HostPlatform(_platform)

        _assigned_user_id = d.pop("assigned_user_id", UNSET)
        assigned_user_id: UUID | Unset
        if isinstance(_assigned_user_id, Unset):
            assigned_user_id = UNSET
        else:
            assigned_user_id = UUID(_assigned_user_id)

        _credential_holder_id = d.pop("credential_holder_id", UNSET)
        credential_holder_id: UUID | Unset
        if isinstance(_credential_holder_id, Unset):
            credential_holder_id = UNSET
        else:
            credential_holder_id = UUID(_credential_holder_id)

        reported_is_server_host = d.pop("reported_is_server_host", UNSET)

        noise_public_key = d.pop("noise_public_key", UNSET)

        key_fingerprint = d.pop("key_fingerprint", UNSET)

        _session_users = d.pop("session_users", UNSET)
        session_users: list[HostSessionUser] | Unset = UNSET
        if _session_users is not UNSET:
            session_users = []
            for session_users_item_data in _session_users:
                session_users_item = HostSessionUser.from_dict(session_users_item_data)

                session_users.append(session_users_item)

        _reported_session_users = d.pop("reported_session_users", UNSET)
        reported_session_users: list[HostReportedSessionUser] | Unset = UNSET
        if _reported_session_users is not UNSET:
            reported_session_users = []
            for reported_session_users_item_data in _reported_session_users:
                reported_session_users_item = HostReportedSessionUser.from_dict(
                    reported_session_users_item_data
                )

                reported_session_users.append(reported_session_users_item)

        require_local_confirm = d.pop("require_local_confirm", UNSET)

        host_version = d.pop("host_version", UNSET)

        os_version = d.pop("os_version", UNSET)

        _last_seen_at = d.pop("last_seen_at", UNSET)
        last_seen_at: datetime.datetime | Unset
        if isinstance(_last_seen_at, Unset):
            last_seen_at = UNSET
        else:
            last_seen_at = datetime.datetime.fromisoformat(_last_seen_at)

        host = cls(
            id=id,
            name=name,
            state=state,
            llm_credentials=llm_credentials,
            declared_server_host=declared_server_host,
            key_generation=key_generation,
            profiles=profiles,
            doctor=doctor,
            connected=connected,
            created_at=created_at,
            version=version,
            disabled_reason=disabled_reason,
            platform=platform,
            assigned_user_id=assigned_user_id,
            credential_holder_id=credential_holder_id,
            reported_is_server_host=reported_is_server_host,
            noise_public_key=noise_public_key,
            key_fingerprint=key_fingerprint,
            session_users=session_users,
            reported_session_users=reported_session_users,
            require_local_confirm=require_local_confirm,
            host_version=host_version,
            os_version=os_version,
            last_seen_at=last_seen_at,
        )

        host.additional_properties = d
        return host

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
