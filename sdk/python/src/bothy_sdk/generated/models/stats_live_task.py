from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.stats_live_task_kind import StatsLiveTaskKind
from ..models.stats_live_task_state import StatsLiveTaskState
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatsLiveTask")


@_attrs_define
class StatsLiveTask:
    """One child node of an active run in the live topology (25 5.2). Never content except `label`, which only a receiver
    with `run.read` on the run gets.

        Attributes:
            id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            kind (StatsLiveTaskKind):
            name (str): Step name, the adapter's type of the task, never content.
            state (StatsLiveTaskState):
            started_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            parent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            finished_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            tool (str | Unset): Name of the current tool of a running node.
            label (str | Unset): First line of the step title; only with `run.read` on the run.
    """

    id: UUID
    kind: StatsLiveTaskKind
    name: str
    state: StatsLiveTaskState
    started_at: datetime.datetime
    parent_id: UUID | Unset = UNSET
    finished_at: datetime.datetime | Unset = UNSET
    tool: str | Unset = UNSET
    label: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        kind = self.kind.value

        name = self.name

        state = self.state.value

        started_at = self.started_at.isoformat()

        parent_id: str | Unset = UNSET
        if not isinstance(self.parent_id, Unset):
            parent_id = str(self.parent_id)

        finished_at: str | Unset = UNSET
        if not isinstance(self.finished_at, Unset):
            finished_at = self.finished_at.isoformat()

        tool = self.tool

        label = self.label

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "kind": kind,
                "name": name,
                "state": state,
                "started_at": started_at,
            }
        )
        if parent_id is not UNSET:
            field_dict["parent_id"] = parent_id
        if finished_at is not UNSET:
            field_dict["finished_at"] = finished_at
        if tool is not UNSET:
            field_dict["tool"] = tool
        if label is not UNSET:
            field_dict["label"] = label

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        kind = StatsLiveTaskKind(d.pop("kind"))

        name = d.pop("name")

        state = StatsLiveTaskState(d.pop("state"))

        started_at = datetime.datetime.fromisoformat(d.pop("started_at"))

        _parent_id = d.pop("parent_id", UNSET)
        parent_id: UUID | Unset
        if isinstance(_parent_id, Unset):
            parent_id = UNSET
        else:
            parent_id = UUID(_parent_id)

        _finished_at = d.pop("finished_at", UNSET)
        finished_at: datetime.datetime | Unset
        if isinstance(_finished_at, Unset):
            finished_at = UNSET
        else:
            finished_at = datetime.datetime.fromisoformat(_finished_at)

        tool = d.pop("tool", UNSET)

        label = d.pop("label", UNSET)

        stats_live_task = cls(
            id=id,
            kind=kind,
            name=name,
            state=state,
            started_at=started_at,
            parent_id=parent_id,
            finished_at=finished_at,
            tool=tool,
            label=label,
        )

        stats_live_task.additional_properties = d
        return stats_live_task

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
