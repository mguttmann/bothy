from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.status_component_state import StatusComponentState

if TYPE_CHECKING:
    from ..models.status_component import StatusComponent
    from ..models.status_incident import StatusIncident


T = TypeVar("T", bound="StatusPage")


@_attrs_define
class StatusPage:
    """
    Attributes:
        overall_state (StatusComponentState):
        components (list[StatusComponent]):
        incidents (list[StatusIncident]): Open incidents and recent ones within `status.history_window`.
    """

    overall_state: StatusComponentState
    components: list[StatusComponent]
    incidents: list[StatusIncident]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        overall_state = self.overall_state.value

        components = []
        for components_item_data in self.components:
            components_item = components_item_data.to_dict()
            components.append(components_item)

        incidents = []
        for incidents_item_data in self.incidents:
            incidents_item = incidents_item_data.to_dict()
            incidents.append(incidents_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "overall_state": overall_state,
                "components": components,
                "incidents": incidents,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.status_component import StatusComponent
        from ..models.status_incident import StatusIncident

        d = dict(src_dict)
        overall_state = StatusComponentState(d.pop("overall_state"))

        components = []
        _components = d.pop("components")
        for components_item_data in _components:
            components_item = StatusComponent.from_dict(components_item_data)

            components.append(components_item)

        incidents = []
        _incidents = d.pop("incidents")
        for incidents_item_data in _incidents:
            incidents_item = StatusIncident.from_dict(incidents_item_data)

            incidents.append(incidents_item)

        status_page = cls(
            overall_state=overall_state,
            components=components,
            incidents=incidents,
        )

        status_page.additional_properties = d
        return status_page

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
