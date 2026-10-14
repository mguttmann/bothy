from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="CostReconcileResult")


@_attrs_define
class CostReconcileResult:
    """
    Attributes:
        reconciled (int):
        unmatched_run_ids (list[UUID]):
    """

    reconciled: int
    unmatched_run_ids: list[UUID]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        reconciled = self.reconciled

        unmatched_run_ids = []
        for unmatched_run_ids_item_data in self.unmatched_run_ids:
            unmatched_run_ids_item = str(unmatched_run_ids_item_data)
            unmatched_run_ids.append(unmatched_run_ids_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "reconciled": reconciled,
                "unmatched_run_ids": unmatched_run_ids,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        reconciled = d.pop("reconciled")

        unmatched_run_ids = []
        _unmatched_run_ids = d.pop("unmatched_run_ids")
        for unmatched_run_ids_item_data in _unmatched_run_ids:
            unmatched_run_ids_item = UUID(unmatched_run_ids_item_data)

            unmatched_run_ids.append(unmatched_run_ids_item)

        cost_reconcile_result = cls(
            reconciled=reconciled,
            unmatched_run_ids=unmatched_run_ids,
        )

        cost_reconcile_result.additional_properties = d
        return cost_reconcile_result

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
