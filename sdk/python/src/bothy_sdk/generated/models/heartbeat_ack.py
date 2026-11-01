from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_runtime_status import AgentRuntimeStatus
    from ..models.project_trash_request import ProjectTrashRequest


T = TypeVar("T", bound="HeartbeatAck")


@_attrs_define
class HeartbeatAck:
    """
    Attributes:
        presence (AgentRuntimeStatus): Reported state of the agent runtime (heartbeat), not the configured state.
        interval_ms (int): Interval the adapter should keep, `sync.heartbeat_interval`.
        project_trash (list[ProjectTrashRequest] | Unset): Runners only, the project folders of deleted forums this
            agent served whose deletion is final and whose move is not reported yet (09 18.15); absent when there are none.
    """

    presence: AgentRuntimeStatus
    interval_ms: int
    project_trash: list[ProjectTrashRequest] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        presence = self.presence.to_dict()

        interval_ms = self.interval_ms

        project_trash: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.project_trash, Unset):
            project_trash = []
            for project_trash_item_data in self.project_trash:
                project_trash_item = project_trash_item_data.to_dict()
                project_trash.append(project_trash_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "presence": presence,
                "interval_ms": interval_ms,
            }
        )
        if project_trash is not UNSET:
            field_dict["project_trash"] = project_trash

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_runtime_status import AgentRuntimeStatus
        from ..models.project_trash_request import ProjectTrashRequest

        d = dict(src_dict)
        presence = AgentRuntimeStatus.from_dict(d.pop("presence"))

        interval_ms = d.pop("interval_ms")

        _project_trash = d.pop("project_trash", UNSET)
        project_trash: list[ProjectTrashRequest] | Unset = UNSET
        if _project_trash is not UNSET:
            project_trash = []
            for project_trash_item_data in _project_trash:
                project_trash_item = ProjectTrashRequest.from_dict(
                    project_trash_item_data
                )

                project_trash.append(project_trash_item)

        heartbeat_ack = cls(
            presence=presence,
            interval_ms=interval_ms,
            project_trash=project_trash,
        )

        heartbeat_ack.additional_properties = d
        return heartbeat_ack

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
