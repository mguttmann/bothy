from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ReadStatePut")


@_attrs_define
class ReadStatePut:
    """Advances the read position. The server keeps the maximum of old and new.

    Attributes:
        last_read_seq (int):
    """

    last_read_seq: int

    def to_dict(self) -> dict[str, Any]:
        last_read_seq = self.last_read_seq

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "last_read_seq": last_read_seq,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        last_read_seq = d.pop("last_read_seq")

        read_state_put = cls(
            last_read_seq=last_read_seq,
        )

        return read_state_put
