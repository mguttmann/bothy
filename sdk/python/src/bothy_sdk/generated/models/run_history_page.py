from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.run_history_entry import RunHistoryEntry


T = TypeVar("T", bound="RunHistoryPage")


@_attrs_define
class RunHistoryPage:
    """One page of the run history. `next_cursor` is absent on the last page, which ends with the oldest run the caller may
    read.

        Attributes:
            items (list[RunHistoryEntry]):
            next_cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
    """

    items: list[RunHistoryEntry]
    next_cursor: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        items = []
        for items_item_data in self.items:
            items_item = items_item_data.to_dict()
            items.append(items_item)

        next_cursor = self.next_cursor

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "items": items,
            }
        )
        if next_cursor is not UNSET:
            field_dict["next_cursor"] = next_cursor

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.run_history_entry import RunHistoryEntry

        d = dict(src_dict)
        items = []
        _items = d.pop("items")
        for items_item_data in _items:
            items_item = RunHistoryEntry.from_dict(items_item_data)

            items.append(items_item)

        next_cursor = d.pop("next_cursor", UNSET)

        run_history_page = cls(
            items=items,
            next_cursor=next_cursor,
        )

        return run_history_page
