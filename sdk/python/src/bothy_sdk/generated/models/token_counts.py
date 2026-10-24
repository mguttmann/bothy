from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="TokenCounts")


@_attrs_define
class TokenCounts:
    """
    Attributes:
        tokens_in (int):
        tokens_out (int):
        tokens_cache_read (int):
        tokens_cache_write (int):
        tokens_cache_write_1h (int):
    """

    tokens_in: int
    tokens_out: int
    tokens_cache_read: int
    tokens_cache_write: int
    tokens_cache_write_1h: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        tokens_in = self.tokens_in

        tokens_out = self.tokens_out

        tokens_cache_read = self.tokens_cache_read

        tokens_cache_write = self.tokens_cache_write

        tokens_cache_write_1h = self.tokens_cache_write_1h

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "tokens_in": tokens_in,
                "tokens_out": tokens_out,
                "tokens_cache_read": tokens_cache_read,
                "tokens_cache_write": tokens_cache_write,
                "tokens_cache_write_1h": tokens_cache_write_1h,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        tokens_in = d.pop("tokens_in")

        tokens_out = d.pop("tokens_out")

        tokens_cache_read = d.pop("tokens_cache_read")

        tokens_cache_write = d.pop("tokens_cache_write")

        tokens_cache_write_1h = d.pop("tokens_cache_write_1h")

        token_counts = cls(
            tokens_in=tokens_in,
            tokens_out=tokens_out,
            tokens_cache_read=tokens_cache_read,
            tokens_cache_write=tokens_cache_write,
            tokens_cache_write_1h=tokens_cache_write_1h,
        )

        token_counts.additional_properties = d
        return token_counts

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
