from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.agent_runtime_status import AgentRuntimeStatus


T = TypeVar("T", bound="HeartbeatAck")


@_attrs_define
class HeartbeatAck:
    """
    Attributes:
        presence (AgentRuntimeStatus): Reported state of the agent runtime (heartbeat), not the configured state.
        interval_ms (int): Interval the adapter should keep, `sync.heartbeat_interval`.
    """

    presence: AgentRuntimeStatus
    interval_ms: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        presence = self.presence.to_dict()

        interval_ms = self.interval_ms

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "presence": presence,
                "interval_ms": interval_ms,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_runtime_status import AgentRuntimeStatus

        d = dict(src_dict)
        presence = AgentRuntimeStatus.from_dict(d.pop("presence"))

        interval_ms = d.pop("interval_ms")

        heartbeat_ack = cls(
            presence=presence,
            interval_ms=interval_ms,
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
