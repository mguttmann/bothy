from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.thumbnail_variant import ThumbnailVariant

T = TypeVar("T", bound="Thumbnail")


@_attrs_define
class Thumbnail:
    """
    Attributes:
        variant (ThumbnailVariant):
        mime_type (str):
        width (int):
        height (int):
    """

    variant: ThumbnailVariant
    mime_type: str
    width: int
    height: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        variant = self.variant.value

        mime_type = self.mime_type

        width = self.width

        height = self.height

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "variant": variant,
                "mime_type": mime_type,
                "width": width,
                "height": height,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        variant = ThumbnailVariant(d.pop("variant"))

        mime_type = d.pop("mime_type")

        width = d.pop("width")

        height = d.pop("height")

        thumbnail = cls(
            variant=variant,
            mime_type=mime_type,
            width=width,
            height=height,
        )

        thumbnail.additional_properties = d
        return thumbnail

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
