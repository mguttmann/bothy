from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.status_component_state import StatusComponentState
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatusComponent")


@_attrs_define
class StatusComponent:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        description (str):
        sort_key (str):
        state (StatusComponentState):
        updated_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        health_source (str | Unset):
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    id: UUID
    name: str
    description: str
    sort_key: str
    state: StatusComponentState
    updated_at: datetime.datetime
    health_source: str | Unset = UNSET
    agent_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        name = self.name

        description = self.description

        sort_key = self.sort_key

        state = self.state.value

        updated_at = self.updated_at.isoformat()

        health_source = self.health_source

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "description": description,
                "sort_key": sort_key,
                "state": state,
                "updated_at": updated_at,
            }
        )
        if health_source is not UNSET:
            field_dict["health_source"] = health_source
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        name = d.pop("name")

        description = d.pop("description")

        sort_key = d.pop("sort_key")

        state = StatusComponentState(d.pop("state"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        health_source = d.pop("health_source", UNSET)

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        status_component = cls(
            id=id,
            name=name,
            description=description,
            sort_key=sort_key,
            state=state,
            updated_at=updated_at,
            health_source=health_source,
            agent_id=agent_id,
        )

        status_component.additional_properties = d
        return status_component

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
