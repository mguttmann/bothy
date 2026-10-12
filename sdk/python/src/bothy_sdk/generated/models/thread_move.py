from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="ThreadMove")


@_attrs_define
class ThreadMove:
    """
    Attributes:
        target_channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        reason (str):
    """

    target_channel_id: UUID
    reason: str

    def to_dict(self) -> dict[str, Any]:
        target_channel_id = str(self.target_channel_id)

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "target_channel_id": target_channel_id,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        target_channel_id = UUID(d.pop("target_channel_id"))

        reason = d.pop("reason")

        thread_move = cls(
            target_channel_id=target_channel_id,
            reason=reason,
        )

        return thread_move
