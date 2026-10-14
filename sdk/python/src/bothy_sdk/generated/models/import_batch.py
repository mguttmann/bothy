from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.import_object import ImportObject


T = TypeVar("T", bound="ImportBatch")


@_attrs_define
class ImportBatch:
    """One transaction. Objects of one thread, in source order; structure objects (space, channel, forum tag, person) come
    in batches without `thread_origin_key`.

        Attributes:
            items (list[ImportObject]): At most `import.batch_max_items`.
            thread_origin_key (str | Unset):
    """

    items: list[ImportObject]
    thread_origin_key: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        items = []
        for items_item_data in self.items:
            items_item = items_item_data.to_dict()
            items.append(items_item)

        thread_origin_key = self.thread_origin_key

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "items": items,
            }
        )
        if thread_origin_key is not UNSET:
            field_dict["thread_origin_key"] = thread_origin_key

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.import_object import ImportObject

        d = dict(src_dict)
        items = []
        _items = d.pop("items")
        for items_item_data in _items:
            items_item = ImportObject.from_dict(items_item_data)

            items.append(items_item)

        thread_origin_key = d.pop("thread_origin_key", UNSET)

        import_batch = cls(
            items=items,
            thread_origin_key=thread_origin_key,
        )

        return import_batch
