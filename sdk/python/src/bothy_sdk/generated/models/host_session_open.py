from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="HostSessionOpen")


@_attrs_define
class HostSessionOpen:
    """The server asks the host service to open the data connection of a session.

    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        data_path (str):
        profile (str):
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    session_id: UUID
    data_path: str
    profile: str
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        data_path = self.data_path

        profile = self.profile

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "session_id": session_id,
                "data_path": data_path,
                "profile": profile,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        data_path = d.pop("data_path")

        profile = d.pop("profile")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        host_session_open = cls(
            session_id=session_id,
            data_path=data_path,
            profile=profile,
            expires_at=expires_at,
        )

        host_session_open.additional_properties = d
        return host_session_open

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
