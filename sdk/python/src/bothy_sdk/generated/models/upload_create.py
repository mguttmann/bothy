from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="UploadCreate")


@_attrs_define
class UploadCreate:
    """
    Attributes:
        name (str):
        size_bytes (int):
        declared_mime_type (str | Unset):
        sha256 (str | Unset): Hex SHA-256; the server verifies it on completion when given.
    """

    name: str
    size_bytes: int
    declared_mime_type: str | Unset = UNSET
    sha256: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        size_bytes = self.size_bytes

        declared_mime_type = self.declared_mime_type

        sha256 = self.sha256

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "size_bytes": size_bytes,
            }
        )
        if declared_mime_type is not UNSET:
            field_dict["declared_mime_type"] = declared_mime_type
        if sha256 is not UNSET:
            field_dict["sha256"] = sha256

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        size_bytes = d.pop("size_bytes")

        declared_mime_type = d.pop("declared_mime_type", UNSET)

        sha256 = d.pop("sha256", UNSET)

        upload_create = cls(
            name=name,
            size_bytes=size_bytes,
            declared_mime_type=declared_mime_type,
            sha256=sha256,
        )

        return upload_create
