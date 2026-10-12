from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.status_incident_impact import StatusIncidentImpact
from ..models.status_incident_kind import StatusIncidentKind
from ..models.status_incident_state import StatusIncidentState
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatusIncidentCreate")


@_attrs_define
class StatusIncidentCreate:
    """
    Attributes:
        title (str):
        kind (StatusIncidentKind):
        impact (StatusIncidentImpact):
        started_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        component_ids (list[UUID]):
        body_md (str):
        state (StatusIncidentState | Unset):
    """

    title: str
    kind: StatusIncidentKind
    impact: StatusIncidentImpact
    started_at: datetime.datetime
    component_ids: list[UUID]
    body_md: str
    state: StatusIncidentState | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        title = self.title

        kind = self.kind.value

        impact = self.impact.value

        started_at = self.started_at.isoformat()

        component_ids = []
        for component_ids_item_data in self.component_ids:
            component_ids_item = str(component_ids_item_data)
            component_ids.append(component_ids_item)

        body_md = self.body_md

        state: str | Unset = UNSET
        if not isinstance(self.state, Unset):
            state = self.state.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "title": title,
                "kind": kind,
                "impact": impact,
                "started_at": started_at,
                "component_ids": component_ids,
                "body_md": body_md,
            }
        )
        if state is not UNSET:
            field_dict["state"] = state

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        title = d.pop("title")

        kind = StatusIncidentKind(d.pop("kind"))

        impact = StatusIncidentImpact(d.pop("impact"))

        started_at = datetime.datetime.fromisoformat(d.pop("started_at"))

        component_ids = []
        _component_ids = d.pop("component_ids")
        for component_ids_item_data in _component_ids:
            component_ids_item = UUID(component_ids_item_data)

            component_ids.append(component_ids_item)

        body_md = d.pop("body_md")

        _state = d.pop("state", UNSET)
        state: StatusIncidentState | Unset
        if isinstance(_state, Unset):
            state = UNSET
        else:
            state = StatusIncidentState(_state)

        status_incident_create = cls(
            title=title,
            kind=kind,
            impact=impact,
            started_at=started_at,
            component_ids=component_ids,
            body_md=body_md,
            state=state,
        )

        return status_incident_create
