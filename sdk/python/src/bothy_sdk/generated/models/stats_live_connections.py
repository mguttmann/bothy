from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="StatsLiveConnections")


@_attrs_define
class StatsLiveConnections:
    """Gateway connections of this node; only with `cluster.mode = single`.

    Attributes:
        user (int):
        agent (int):
        app (int):
    """

    user: int
    agent: int
    app: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        user = self.user

        agent = self.agent

        app = self.app

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "user": user,
                "agent": agent,
                "app": app,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        user = d.pop("user")

        agent = d.pop("agent")

        app = d.pop("app")

        stats_live_connections = cls(
            user=user,
            agent=agent,
            app=app,
        )

        stats_live_connections.additional_properties = d
        return stats_live_connections

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
