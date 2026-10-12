from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="StatsBackendDenials")


@_attrs_define
class StatsBackendDenials:
    """
    Attributes:
        i2_denied (int):
        billing_mode_mismatch (int):
    """

    i2_denied: int
    billing_mode_mismatch: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        i2_denied = self.i2_denied

        billing_mode_mismatch = self.billing_mode_mismatch

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "i2_denied": i2_denied,
                "billing_mode_mismatch": billing_mode_mismatch,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        i2_denied = d.pop("i2_denied")

        billing_mode_mismatch = d.pop("billing_mode_mismatch")

        stats_backend_denials = cls(
            i2_denied=i2_denied,
            billing_mode_mismatch=billing_mode_mismatch,
        )

        stats_backend_denials.additional_properties = d
        return stats_backend_denials

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
