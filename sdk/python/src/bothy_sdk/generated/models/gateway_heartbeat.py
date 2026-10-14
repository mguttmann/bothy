from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.heartbeat import Heartbeat


T = TypeVar("T", bound="GatewayHeartbeat")


@_attrs_define
class GatewayHeartbeat:
    """
    Attributes:
        last_seq (int):
        agent (Heartbeat | Unset):
    """

    last_seq: int
    agent: Heartbeat | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        last_seq = self.last_seq

        agent: dict[str, Any] | Unset = UNSET
        if not isinstance(self.agent, Unset):
            agent = self.agent.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "last_seq": last_seq,
            }
        )
        if agent is not UNSET:
            field_dict["agent"] = agent

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.heartbeat import Heartbeat

        d = dict(src_dict)
        last_seq = d.pop("last_seq")

        _agent = d.pop("agent", UNSET)
        agent: Heartbeat | Unset
        if isinstance(_agent, Unset):
            agent = UNSET
        else:
            agent = Heartbeat.from_dict(_agent)

        gateway_heartbeat = cls(
            last_seq=last_seq,
            agent=agent,
        )

        gateway_heartbeat.additional_properties = d
        return gateway_heartbeat

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
