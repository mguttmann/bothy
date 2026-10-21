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
        sha256 (str): Lowercase hex SHA-256 of the whole file; the verify pass checks it.
        declared_mime_type (str | Unset):
    """

    name: str
    size_bytes: int
    sha256: str
    declared_mime_type: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        size_bytes = self.size_bytes

        sha256 = self.sha256

        declared_mime_type = self.declared_mime_type

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "size_bytes": size_bytes,
                "sha256": sha256,
            }
        )
        if declared_mime_type is not UNSET:
            field_dict["declared_mime_type"] = declared_mime_type

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        size_bytes = d.pop("size_bytes")

        sha256 = d.pop("sha256")

        declared_mime_type = d.pop("declared_mime_type", UNSET)

        upload_create = cls(
            name=name,
            size_bytes=size_bytes,
            sha256=sha256,
            declared_mime_type=declared_mime_type,
        )

        return upload_create
