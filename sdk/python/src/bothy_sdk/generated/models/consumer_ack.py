from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ConsumerAck")


@_attrs_define
class ConsumerAck:
    """
    Attributes:
        seq (int):
    """

    seq: int

    def to_dict(self) -> dict[str, Any]:
        seq = self.seq

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "seq": seq,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        seq = d.pop("seq")

        consumer_ack = cls(
            seq=seq,
        )

        return consumer_ack
