from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.status_incident_state import StatusIncidentState

T = TypeVar("T", bound="StatusIncidentUpdateCreate")


@_attrs_define
class StatusIncidentUpdateCreate:
    """
    Attributes:
        state (StatusIncidentState):
        body_md (str):
    """

    state: StatusIncidentState
    body_md: str

    def to_dict(self) -> dict[str, Any]:
        state = self.state.value

        body_md = self.body_md

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "state": state,
                "body_md": body_md,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        state = StatusIncidentState(d.pop("state"))

        body_md = d.pop("body_md")

        status_incident_update_create = cls(
            state=state,
            body_md=body_md,
        )

        return status_incident_update_create
