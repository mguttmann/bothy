from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.status_component_state import StatusComponentState
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatusUpdatedPayload")


@_attrs_define
class StatusUpdatedPayload:
    """
    Attributes:
        overall_state (StatusComponentState):
        changed_component_ids (list[UUID] | Unset):
        changed_incident_ids (list[UUID] | Unset):
    """

    overall_state: StatusComponentState
    changed_component_ids: list[UUID] | Unset = UNSET
    changed_incident_ids: list[UUID] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        overall_state = self.overall_state.value

        changed_component_ids: list[str] | Unset = UNSET
        if not isinstance(self.changed_component_ids, Unset):
            changed_component_ids = []
            for changed_component_ids_item_data in self.changed_component_ids:
                changed_component_ids_item = str(changed_component_ids_item_data)
                changed_component_ids.append(changed_component_ids_item)

        changed_incident_ids: list[str] | Unset = UNSET
        if not isinstance(self.changed_incident_ids, Unset):
            changed_incident_ids = []
            for changed_incident_ids_item_data in self.changed_incident_ids:
                changed_incident_ids_item = str(changed_incident_ids_item_data)
                changed_incident_ids.append(changed_incident_ids_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "overall_state": overall_state,
            }
        )
        if changed_component_ids is not UNSET:
            field_dict["changed_component_ids"] = changed_component_ids
        if changed_incident_ids is not UNSET:
            field_dict["changed_incident_ids"] = changed_incident_ids

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        overall_state = StatusComponentState(d.pop("overall_state"))

        _changed_component_ids = d.pop("changed_component_ids", UNSET)
        changed_component_ids: list[UUID] | Unset = UNSET
        if _changed_component_ids is not UNSET:
            changed_component_ids = []
            for changed_component_ids_item_data in _changed_component_ids:
                changed_component_ids_item = UUID(changed_component_ids_item_data)

                changed_component_ids.append(changed_component_ids_item)

        _changed_incident_ids = d.pop("changed_incident_ids", UNSET)
        changed_incident_ids: list[UUID] | Unset = UNSET
        if _changed_incident_ids is not UNSET:
            changed_incident_ids = []
            for changed_incident_ids_item_data in _changed_incident_ids:
                changed_incident_ids_item = UUID(changed_incident_ids_item_data)

                changed_incident_ids.append(changed_incident_ids_item)

        status_updated_payload = cls(
            overall_state=overall_state,
            changed_component_ids=changed_component_ids,
            changed_incident_ids=changed_incident_ids,
        )

        status_updated_payload.additional_properties = d
        return status_updated_payload

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
