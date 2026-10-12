from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="BrandingUpdate")


@_attrs_define
class BrandingUpdate:
    """
    Attributes:
        product_name (str):
        accent_color (str | Unset):
    """

    product_name: str
    accent_color: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        product_name = self.product_name

        accent_color = self.accent_color

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "product_name": product_name,
            }
        )
        if accent_color is not UNSET:
            field_dict["accent_color"] = accent_color

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        product_name = d.pop("product_name")

        accent_color = d.pop("accent_color", UNSET)

        branding_update = cls(
            product_name=product_name,
            accent_color=accent_color,
        )

        return branding_update
