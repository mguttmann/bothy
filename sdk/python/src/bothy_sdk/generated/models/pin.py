from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="Pin")


@_attrs_define
class Pin:
    """
    Attributes:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        pinned_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        pinned_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    message_id: UUID
    thread_id: UUID
    pinned_by: UUID
    pinned_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        message_id = str(self.message_id)

        thread_id = str(self.thread_id)

        pinned_by = str(self.pinned_by)

        pinned_at = self.pinned_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "message_id": message_id,
                "thread_id": thread_id,
                "pinned_by": pinned_by,
                "pinned_at": pinned_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        message_id = UUID(d.pop("message_id"))

        thread_id = UUID(d.pop("thread_id"))

        pinned_by = UUID(d.pop("pinned_by"))

        pinned_at = datetime.datetime.fromisoformat(d.pop("pinned_at"))

        pin = cls(
            message_id=message_id,
            thread_id=thread_id,
            pinned_by=pinned_by,
            pinned_at=pinned_at,
        )

        pin.additional_properties = d
        return pin

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
