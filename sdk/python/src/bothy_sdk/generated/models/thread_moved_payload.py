from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.thread import Thread


T = TypeVar("T", bound="ThreadMovedPayload")


@_attrs_define
class ThreadMovedPayload:
    """
    Attributes:
        thread (Thread):
        from_channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    thread: Thread
    from_channel_id: UUID
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread = self.thread.to_dict()

        from_channel_id = str(self.from_channel_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "thread": thread,
                "from_channel_id": from_channel_id,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.thread import Thread

        d = dict(src_dict)
        thread = Thread.from_dict(d.pop("thread"))

        from_channel_id = UUID(d.pop("from_channel_id"))

        thread_moved_payload = cls(
            thread=thread,
            from_channel_id=from_channel_id,
        )

        thread_moved_payload.additional_properties = d
        return thread_moved_payload

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
