from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ChannelReadOnlySet")


@_attrs_define
class ChannelReadOnlySet:
    """
    Attributes:
        read_only (bool):
        reason (str):
    """

    read_only: bool
    reason: str

    def to_dict(self) -> dict[str, Any]:
        read_only = self.read_only

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "read_only": read_only,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        read_only = d.pop("read_only")

        reason = d.pop("reason")

        channel_read_only_set = cls(
            read_only=read_only,
            reason=reason,
        )

        return channel_read_only_set
