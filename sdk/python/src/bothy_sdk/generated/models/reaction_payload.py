from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ReactionPayload")


@_attrs_define
class ReactionPayload:
    """
    Attributes:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        emoji (str):
        principal_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        count (int): Count for this emoji after the change.
    """

    message_id: UUID
    thread_id: UUID
    emoji: str
    principal_id: UUID
    count: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        message_id = str(self.message_id)

        thread_id = str(self.thread_id)

        emoji = self.emoji

        principal_id = str(self.principal_id)

        count = self.count

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "message_id": message_id,
                "thread_id": thread_id,
                "emoji": emoji,
                "principal_id": principal_id,
                "count": count,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        message_id = UUID(d.pop("message_id"))

        thread_id = UUID(d.pop("thread_id"))

        emoji = d.pop("emoji")

        principal_id = UUID(d.pop("principal_id"))

        count = d.pop("count")

        reaction_payload = cls(
            message_id=message_id,
            thread_id=thread_id,
            emoji=emoji,
            principal_id=principal_id,
            count=count,
        )

        reaction_payload.additional_properties = d
        return reaction_payload

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
