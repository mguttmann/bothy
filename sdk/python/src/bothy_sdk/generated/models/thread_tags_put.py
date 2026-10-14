from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="ThreadTagsPut")


@_attrs_define
class ThreadTagsPut:
    """
    Attributes:
        tag_ids (list[UUID]): Complete set, at most `thread.max_tags`, without duplicates.
    """

    tag_ids: list[UUID]

    def to_dict(self) -> dict[str, Any]:
        tag_ids = []
        for tag_ids_item_data in self.tag_ids:
            tag_ids_item = str(tag_ids_item_data)
            tag_ids.append(tag_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "tag_ids": tag_ids,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        tag_ids = []
        _tag_ids = d.pop("tag_ids")
        for tag_ids_item_data in _tag_ids:
            tag_ids_item = UUID(tag_ids_item_data)

            tag_ids.append(tag_ids_item)

        thread_tags_put = cls(
            tag_ids=tag_ids,
        )

        return thread_tags_put
