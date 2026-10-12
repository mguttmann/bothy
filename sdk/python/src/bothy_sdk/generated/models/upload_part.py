from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.header_pair import HeaderPair


T = TypeVar("T", bound="UploadPart")


@_attrs_define
class UploadPart:
    """
    Attributes:
        part_number (int):
        url (str):
        headers (list[HeaderPair]): Headers the client must send with the PUT.
    """

    part_number: int
    url: str
    headers: list[HeaderPair]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        part_number = self.part_number

        url = self.url

        headers = []
        for headers_item_data in self.headers:
            headers_item = headers_item_data.to_dict()
            headers.append(headers_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "part_number": part_number,
                "url": url,
                "headers": headers,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.header_pair import HeaderPair

        d = dict(src_dict)
        part_number = d.pop("part_number")

        url = d.pop("url")

        headers = []
        _headers = d.pop("headers")
        for headers_item_data in _headers:
            headers_item = HeaderPair.from_dict(headers_item_data)

            headers.append(headers_item)

        upload_part = cls(
            part_number=part_number,
            url=url,
            headers=headers,
        )

        upload_part.additional_properties = d
        return upload_part

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
