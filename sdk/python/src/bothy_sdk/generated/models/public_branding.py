from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="PublicBranding")


@_attrs_define
class PublicBranding:
    """
    Attributes:
        product_name (str):
        accent_color (str | Unset): Key of a category color of the design system (`spruce`, `loch`, `heather`, `rowan`,
            `amber`, `gorse`, `moss`), never a free color value (ADR 0044).
        logo_url (str | Unset):
    """

    product_name: str
    accent_color: str | Unset = UNSET
    logo_url: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        product_name = self.product_name

        accent_color = self.accent_color

        logo_url = self.logo_url

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "product_name": product_name,
            }
        )
        if accent_color is not UNSET:
            field_dict["accent_color"] = accent_color
        if logo_url is not UNSET:
            field_dict["logo_url"] = logo_url

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        product_name = d.pop("product_name")

        accent_color = d.pop("accent_color", UNSET)

        logo_url = d.pop("logo_url", UNSET)

        public_branding = cls(
            product_name=product_name,
            accent_color=accent_color,
            logo_url=logo_url,
        )

        public_branding.additional_properties = d
        return public_branding

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
