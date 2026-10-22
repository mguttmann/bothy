from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.access_labels import AccessLabels
    from ..models.expiring_access_item import ExpiringAccessItem


T = TypeVar("T", bound="ExpiringAccessPage")


@_attrs_define
class ExpiringAccessPage:
    """One page. `next_cursor` is absent on the last page.

    Attributes:
        items (list[ExpiringAccessItem]):
        labels (AccessLabels): The names of the principals, groups, spaces and channels a page of hints or expiring
            access names, as the administration lists show them; never content.
        next_cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
    """

    items: list[ExpiringAccessItem]
    labels: AccessLabels
    next_cursor: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        items = []
        for items_item_data in self.items:
            items_item = items_item_data.to_dict()
            items.append(items_item)

        labels = self.labels.to_dict()

        next_cursor = self.next_cursor

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "items": items,
                "labels": labels,
            }
        )
        if next_cursor is not UNSET:
            field_dict["next_cursor"] = next_cursor

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.access_labels import AccessLabels
        from ..models.expiring_access_item import ExpiringAccessItem

        d = dict(src_dict)
        items = []
        _items = d.pop("items")
        for items_item_data in _items:
            items_item = ExpiringAccessItem.from_dict(items_item_data)

            items.append(items_item)

        labels = AccessLabels.from_dict(d.pop("labels"))

        next_cursor = d.pop("next_cursor", UNSET)

        expiring_access_page = cls(
            items=items,
            labels=labels,
            next_cursor=next_cursor,
        )

        expiring_access_page.additional_properties = d
        return expiring_access_page

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
