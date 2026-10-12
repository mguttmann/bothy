from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.stats_live_agent import StatsLiveAgent
    from ..models.stats_live_connections import StatsLiveConnections
    from ..models.stats_live_rate import StatsLiveRate
    from ..models.stats_live_run import StatsLiveRun
    from ..models.stats_live_sli import StatsLiveSli
    from ..models.stats_scope import StatsScope


T = TypeVar("T", bound="StatsLiveFrame")


@_attrs_define
class StatsLiveFrame:
    """Live frame (docs/spec/25-visualization.md, section 5.2). No person values, no content.

    Attributes:
        at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        interval_ms (int):
        scope (StatsScope):
        runs (list[StatsLiveRun]):
        runs_truncated (bool):
        agents (list[StatsLiveAgent]):
        rates (list[StatsLiveRate]):
        sli (list[StatsLiveSli] | Unset): Only for the instance scope.
        connections (StatsLiveConnections | Unset): Gateway connections of this node; only with `cluster.mode = single`.
    """

    at: datetime.datetime
    interval_ms: int
    scope: StatsScope
    runs: list[StatsLiveRun]
    runs_truncated: bool
    agents: list[StatsLiveAgent]
    rates: list[StatsLiveRate]
    sli: list[StatsLiveSli] | Unset = UNSET
    connections: StatsLiveConnections | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        at = self.at.isoformat()

        interval_ms = self.interval_ms

        scope = self.scope.to_dict()

        runs = []
        for runs_item_data in self.runs:
            runs_item = runs_item_data.to_dict()
            runs.append(runs_item)

        runs_truncated = self.runs_truncated

        agents = []
        for agents_item_data in self.agents:
            agents_item = agents_item_data.to_dict()
            agents.append(agents_item)

        rates = []
        for rates_item_data in self.rates:
            rates_item = rates_item_data.to_dict()
            rates.append(rates_item)

        sli: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.sli, Unset):
            sli = []
            for sli_item_data in self.sli:
                sli_item = sli_item_data.to_dict()
                sli.append(sli_item)

        connections: dict[str, Any] | Unset = UNSET
        if not isinstance(self.connections, Unset):
            connections = self.connections.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "at": at,
                "interval_ms": interval_ms,
                "scope": scope,
                "runs": runs,
                "runs_truncated": runs_truncated,
                "agents": agents,
                "rates": rates,
            }
        )
        if sli is not UNSET:
            field_dict["sli"] = sli
        if connections is not UNSET:
            field_dict["connections"] = connections

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.stats_live_agent import StatsLiveAgent
        from ..models.stats_live_connections import StatsLiveConnections
        from ..models.stats_live_rate import StatsLiveRate
        from ..models.stats_live_run import StatsLiveRun
        from ..models.stats_live_sli import StatsLiveSli
        from ..models.stats_scope import StatsScope

        d = dict(src_dict)
        at = datetime.datetime.fromisoformat(d.pop("at"))

        interval_ms = d.pop("interval_ms")

        scope = StatsScope.from_dict(d.pop("scope"))

        runs = []
        _runs = d.pop("runs")
        for runs_item_data in _runs:
            runs_item = StatsLiveRun.from_dict(runs_item_data)

            runs.append(runs_item)

        runs_truncated = d.pop("runs_truncated")

        agents = []
        _agents = d.pop("agents")
        for agents_item_data in _agents:
            agents_item = StatsLiveAgent.from_dict(agents_item_data)

            agents.append(agents_item)

        rates = []
        _rates = d.pop("rates")
        for rates_item_data in _rates:
            rates_item = StatsLiveRate.from_dict(rates_item_data)

            rates.append(rates_item)

        _sli = d.pop("sli", UNSET)
        sli: list[StatsLiveSli] | Unset = UNSET
        if _sli is not UNSET:
            sli = []
            for sli_item_data in _sli:
                sli_item = StatsLiveSli.from_dict(sli_item_data)

                sli.append(sli_item)

        _connections = d.pop("connections", UNSET)
        connections: StatsLiveConnections | Unset
        if isinstance(_connections, Unset):
            connections = UNSET
        else:
            connections = StatsLiveConnections.from_dict(_connections)

        stats_live_frame = cls(
            at=at,
            interval_ms=interval_ms,
            scope=scope,
            runs=runs,
            runs_truncated=runs_truncated,
            agents=agents,
            rates=rates,
            sli=sli,
            connections=connections,
        )

        stats_live_frame.additional_properties = d
        return stats_live_frame

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
