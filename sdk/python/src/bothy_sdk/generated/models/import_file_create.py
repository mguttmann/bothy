from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportFileCreate")


@_attrs_define
class ImportFileCreate:
    """
    Attributes:
        origin_key (str):
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        author_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        size_bytes (int):
        declared_mime_type (str | Unset):
    """

    origin_key: str
    channel_id: UUID
    author_id: UUID
    name: str
    size_bytes: int
    declared_mime_type: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        origin_key = self.origin_key

        channel_id = str(self.channel_id)

        author_id = str(self.author_id)

        name = self.name

        size_bytes = self.size_bytes

        declared_mime_type = self.declared_mime_type

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "origin_key": origin_key,
                "channel_id": channel_id,
                "author_id": author_id,
                "name": name,
                "size_bytes": size_bytes,
            }
        )
        if declared_mime_type is not UNSET:
            field_dict["declared_mime_type"] = declared_mime_type

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        origin_key = d.pop("origin_key")

        channel_id = UUID(d.pop("channel_id"))

        author_id = UUID(d.pop("author_id"))

        name = d.pop("name")

        size_bytes = d.pop("size_bytes")

        declared_mime_type = d.pop("declared_mime_type", UNSET)

        import_file_create = cls(
            origin_key=origin_key,
            channel_id=channel_id,
            author_id=author_id,
            name=name,
            size_bytes=size_bytes,
            declared_mime_type=declared_mime_type,
        )

        return import_file_create
