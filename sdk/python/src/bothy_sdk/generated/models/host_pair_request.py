from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.host_platform import HostPlatform
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.host_profile import HostProfile
    from ..models.host_reported_session_user import HostReportedSessionUser
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="HostPairRequest")


@_attrs_define
class HostPairRequest:
    """
    Attributes:
        pairing_code (str):
        noise_public_key (str): Static X25519 public key, 32 bytes base64url without padding.
        auth_public_jwk (JsonObject): Free-form JSON object whose schema is named in the field description.
        platform (HostPlatform):
        host_version (str):
        profiles (list[HostProfile]):
        session_users (list[HostReportedSessionUser]): The session users of `host_service.session_users` with the
            display names of their working directories (24 section 2.2).
        os_version (str | Unset):
    """

    pairing_code: str
    noise_public_key: str
    auth_public_jwk: JsonObject
    platform: HostPlatform
    host_version: str
    profiles: list[HostProfile]
    session_users: list[HostReportedSessionUser]
    os_version: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        pairing_code = self.pairing_code

        noise_public_key = self.noise_public_key

        auth_public_jwk = self.auth_public_jwk.to_dict()

        platform = self.platform.value

        host_version = self.host_version

        profiles = []
        for profiles_item_data in self.profiles:
            profiles_item = profiles_item_data.to_dict()
            profiles.append(profiles_item)

        session_users = []
        for session_users_item_data in self.session_users:
            session_users_item = session_users_item_data.to_dict()
            session_users.append(session_users_item)

        os_version = self.os_version

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "pairing_code": pairing_code,
                "noise_public_key": noise_public_key,
                "auth_public_jwk": auth_public_jwk,
                "platform": platform,
                "host_version": host_version,
                "profiles": profiles,
                "session_users": session_users,
            }
        )
        if os_version is not UNSET:
            field_dict["os_version"] = os_version

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_profile import HostProfile
        from ..models.host_reported_session_user import HostReportedSessionUser
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        pairing_code = d.pop("pairing_code")

        noise_public_key = d.pop("noise_public_key")

        auth_public_jwk = JsonObject.from_dict(d.pop("auth_public_jwk"))

        platform = HostPlatform(d.pop("platform"))

        host_version = d.pop("host_version")

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

        os_version = d.pop("os_version", UNSET)

        host_pair_request = cls(
            pairing_code=pairing_code,
            noise_public_key=noise_public_key,
            auth_public_jwk=auth_public_jwk,
            platform=platform,
            host_version=host_version,
            profiles=profiles,
            session_users=session_users,
            os_version=os_version,
        )

        return host_pair_request
