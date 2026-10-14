from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.access_target import AccessTarget


T = TypeVar("T", bound="AccessChangedPayload")


@_attrs_define
class AccessChangedPayload:
    """
    Attributes:
        permission_version (int):
        added (list[AccessTarget]):
        removed (list[AccessTarget]):
        changed (list[AccessTarget]):
    """

    permission_version: int
    added: list[AccessTarget]
    removed: list[AccessTarget]
    changed: list[AccessTarget]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        permission_version = self.permission_version

        added = []
        for added_item_data in self.added:
            added_item = added_item_data.to_dict()
            added.append(added_item)

        removed = []
        for removed_item_data in self.removed:
            removed_item = removed_item_data.to_dict()
            removed.append(removed_item)

        changed = []
        for changed_item_data in self.changed:
            changed_item = changed_item_data.to_dict()
            changed.append(changed_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "permission_version": permission_version,
                "added": added,
                "removed": removed,
                "changed": changed,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.access_target import AccessTarget

        d = dict(src_dict)
        permission_version = d.pop("permission_version")

        added = []
        _added = d.pop("added")
        for added_item_data in _added:
            added_item = AccessTarget.from_dict(added_item_data)

            added.append(added_item)

        removed = []
        _removed = d.pop("removed")
        for removed_item_data in _removed:
            removed_item = AccessTarget.from_dict(removed_item_data)

            removed.append(removed_item)

        changed = []
        _changed = d.pop("changed")
        for changed_item_data in _changed:
            changed_item = AccessTarget.from_dict(changed_item_data)

            changed.append(changed_item)

        access_changed_payload = cls(
            permission_version=permission_version,
            added=added,
            removed=removed,
            changed=changed,
        )

        access_changed_payload.additional_properties = d
        return access_changed_payload

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
