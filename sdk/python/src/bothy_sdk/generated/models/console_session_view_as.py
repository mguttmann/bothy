from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ConsoleSessionViewAs")


@_attrs_define
class ConsoleSessionViewAs:
    """
    Attributes:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        display_name (str):
        started_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        ends_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    user_id: UUID
    display_name: str
    started_at: datetime.datetime
    ends_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        user_id = str(self.user_id)

        display_name = self.display_name

        started_at = self.started_at.isoformat()

        ends_at = self.ends_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "user_id": user_id,
                "display_name": display_name,
                "started_at": started_at,
                "ends_at": ends_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        user_id = UUID(d.pop("user_id"))

        display_name = d.pop("display_name")

        started_at = datetime.datetime.fromisoformat(d.pop("started_at"))

        ends_at = datetime.datetime.fromisoformat(d.pop("ends_at"))

        console_session_view_as = cls(
            user_id=user_id,
            display_name=display_name,
            started_at=started_at,
            ends_at=ends_at,
        )

        console_session_view_as.additional_properties = d
        return console_session_view_as

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
