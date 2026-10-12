from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.status_component_state import StatusComponentState
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatusComponentInput")


@_attrs_define
class StatusComponentInput:
    """
    Attributes:
        name (str):
        description (str | Unset):
        state (StatusComponentState | Unset):
        health_source (str | Unset):
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        after_component_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    name: str
    description: str | Unset = UNSET
    state: StatusComponentState | Unset = UNSET
    health_source: str | Unset = UNSET
    agent_id: UUID | Unset = UNSET
    after_component_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        description = self.description

        state: str | Unset = UNSET
        if not isinstance(self.state, Unset):
            state = self.state.value

        health_source = self.health_source

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        after_component_id: str | Unset = UNSET
        if not isinstance(self.after_component_id, Unset):
            after_component_id = str(self.after_component_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
            }
        )
        if description is not UNSET:
            field_dict["description"] = description
        if state is not UNSET:
            field_dict["state"] = state
        if health_source is not UNSET:
            field_dict["health_source"] = health_source
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if after_component_id is not UNSET:
            field_dict["after_component_id"] = after_component_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        description = d.pop("description", UNSET)

        _state = d.pop("state", UNSET)
        state: StatusComponentState | Unset
        if isinstance(_state, Unset):
            state = UNSET
        else:
            state = StatusComponentState(_state)

        health_source = d.pop("health_source", UNSET)

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        _after_component_id = d.pop("after_component_id", UNSET)
        after_component_id: UUID | Unset
        if isinstance(_after_component_id, Unset):
            after_component_id = UNSET
        else:
            after_component_id = UUID(_after_component_id)

        status_component_input = cls(
            name=name,
            description=description,
            state=state,
            health_source=health_source,
            agent_id=agent_id,
            after_component_id=after_component_id,
        )

        return status_component_input
