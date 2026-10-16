from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="GatewayTyping")


@_attrs_define
class GatewayTyping:
    """
    Attributes:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    thread_id: UUID

    def to_dict(self) -> dict[str, Any]:
        thread_id = str(self.thread_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "thread_id": thread_id,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        thread_id = UUID(d.pop("thread_id"))

        gateway_typing = cls(
            thread_id=thread_id,
        )

        return gateway_typing
