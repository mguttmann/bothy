from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.project_folder import ProjectFolder


T = TypeVar("T", bound="ProjectFolderList")


@_attrs_define
class ProjectFolderList:
    """
    Attributes:
        items (list[ProjectFolder]): The reported folders without a binding, in name order.
        reported (bool): Whether the runner has reported its folders at least once; false for a runner that never did
            (an older runner or not yet connected).
    """

    items: list[ProjectFolder]
    reported: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        items = []
        for items_item_data in self.items:
            items_item = items_item_data.to_dict()
            items.append(items_item)

        reported = self.reported

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "items": items,
                "reported": reported,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.project_folder import ProjectFolder

        d = dict(src_dict)
        items = []
        _items = d.pop("items")
        for items_item_data in _items:
            items_item = ProjectFolder.from_dict(items_item_data)

            items.append(items_item)

        reported = d.pop("reported")

        project_folder_list = cls(
            items=items,
            reported=reported,
        )

        project_folder_list.additional_properties = d
        return project_folder_list

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
