from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ConsumerState")


@_attrs_define
class ConsumerState:
    """
    Attributes:
        consumer (str):
        acked_seq (int):
        head_seq (int):
    """

    consumer: str
    acked_seq: int
    head_seq: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        consumer = self.consumer

        acked_seq = self.acked_seq

        head_seq = self.head_seq

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "consumer": consumer,
                "acked_seq": acked_seq,
                "head_seq": head_seq,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        consumer = d.pop("consumer")

        acked_seq = d.pop("acked_seq")

        head_seq = d.pop("head_seq")

        consumer_state = cls(
            consumer=consumer,
            acked_seq=acked_seq,
            head_seq=head_seq,
        )

        consumer_state.additional_properties = d
        return consumer_state

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
