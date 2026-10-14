from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="BillingModeSetAllResult")


@_attrs_define
class BillingModeSetAllResult:
    """
    Attributes:
        changed_agent_ids (list[UUID]):
    """

    changed_agent_ids: list[UUID]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        changed_agent_ids = []
        for changed_agent_ids_item_data in self.changed_agent_ids:
            changed_agent_ids_item = str(changed_agent_ids_item_data)
            changed_agent_ids.append(changed_agent_ids_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "changed_agent_ids": changed_agent_ids,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        changed_agent_ids = []
        _changed_agent_ids = d.pop("changed_agent_ids")
        for changed_agent_ids_item_data in _changed_agent_ids:
            changed_agent_ids_item = UUID(changed_agent_ids_item_data)

            changed_agent_ids.append(changed_agent_ids_item)

        billing_mode_set_all_result = cls(
            changed_agent_ids=changed_agent_ids,
        )

        billing_mode_set_all_result.additional_properties = d
        return billing_mode_set_all_result

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
