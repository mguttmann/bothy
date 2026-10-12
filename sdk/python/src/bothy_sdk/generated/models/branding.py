from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="Branding")


@_attrs_define
class Branding:
    """
    Attributes:
        product_name (str):
        version (int):
        accent_color (str | Unset):
        logo_file_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    product_name: str
    version: int
    accent_color: str | Unset = UNSET
    logo_file_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        product_name = self.product_name

        version = self.version

        accent_color = self.accent_color

        logo_file_id: str | Unset = UNSET
        if not isinstance(self.logo_file_id, Unset):
            logo_file_id = str(self.logo_file_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "product_name": product_name,
                "version": version,
            }
        )
        if accent_color is not UNSET:
            field_dict["accent_color"] = accent_color
        if logo_file_id is not UNSET:
            field_dict["logo_file_id"] = logo_file_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        product_name = d.pop("product_name")

        version = d.pop("version")

        accent_color = d.pop("accent_color", UNSET)

        _logo_file_id = d.pop("logo_file_id", UNSET)
        logo_file_id: UUID | Unset
        if isinstance(_logo_file_id, Unset):
            logo_file_id = UNSET
        else:
            logo_file_id = UUID(_logo_file_id)

        branding = cls(
            product_name=product_name,
            version=version,
            accent_color=accent_color,
            logo_file_id=logo_file_id,
        )

        branding.additional_properties = d
        return branding

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
