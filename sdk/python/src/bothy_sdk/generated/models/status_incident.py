from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.status_incident_impact import StatusIncidentImpact
from ..models.status_incident_kind import StatusIncidentKind
from ..models.status_incident_state import StatusIncidentState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.status_incident_update import StatusIncidentUpdate


T = TypeVar("T", bound="StatusIncident")


@_attrs_define
class StatusIncident:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        title (str):
        kind (StatusIncidentKind):
        impact (StatusIncidentImpact):
        state (StatusIncidentState):
        started_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        component_ids (list[UUID]):
        updates (list[StatusIncidentUpdate]):
        resolved_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    title: str
    kind: StatusIncidentKind
    impact: StatusIncidentImpact
    state: StatusIncidentState
    started_at: datetime.datetime
    component_ids: list[UUID]
    updates: list[StatusIncidentUpdate]
    resolved_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        title = self.title

        kind = self.kind.value

        impact = self.impact.value

        state = self.state.value

        started_at = self.started_at.isoformat()

        component_ids = []
        for component_ids_item_data in self.component_ids:
            component_ids_item = str(component_ids_item_data)
            component_ids.append(component_ids_item)

        updates = []
        for updates_item_data in self.updates:
            updates_item = updates_item_data.to_dict()
            updates.append(updates_item)

        resolved_at: str | Unset = UNSET
        if not isinstance(self.resolved_at, Unset):
            resolved_at = self.resolved_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "title": title,
                "kind": kind,
                "impact": impact,
                "state": state,
                "started_at": started_at,
                "component_ids": component_ids,
                "updates": updates,
            }
        )
        if resolved_at is not UNSET:
            field_dict["resolved_at"] = resolved_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.status_incident_update import StatusIncidentUpdate

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        title = d.pop("title")

        kind = StatusIncidentKind(d.pop("kind"))

        impact = StatusIncidentImpact(d.pop("impact"))

        state = StatusIncidentState(d.pop("state"))

        started_at = datetime.datetime.fromisoformat(d.pop("started_at"))

        component_ids = []
        _component_ids = d.pop("component_ids")
        for component_ids_item_data in _component_ids:
            component_ids_item = UUID(component_ids_item_data)

            component_ids.append(component_ids_item)

        updates = []
        _updates = d.pop("updates")
        for updates_item_data in _updates:
            updates_item = StatusIncidentUpdate.from_dict(updates_item_data)

            updates.append(updates_item)

        _resolved_at = d.pop("resolved_at", UNSET)
        resolved_at: datetime.datetime | Unset
        if isinstance(_resolved_at, Unset):
            resolved_at = UNSET
        else:
            resolved_at = datetime.datetime.fromisoformat(_resolved_at)

        status_incident = cls(
            id=id,
            title=title,
            kind=kind,
            impact=impact,
            state=state,
            started_at=started_at,
            component_ids=component_ids,
            updates=updates,
            resolved_at=resolved_at,
        )

        status_incident.additional_properties = d
        return status_incident

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
