from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ChannelAutonomousRunsSet")


@_attrs_define
class ChannelAutonomousRunsSet:
    """
    Attributes:
        allow_autonomous_runs (bool):
    """

    allow_autonomous_runs: bool

    def to_dict(self) -> dict[str, Any]:
        allow_autonomous_runs = self.allow_autonomous_runs

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "allow_autonomous_runs": allow_autonomous_runs,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        allow_autonomous_runs = d.pop("allow_autonomous_runs")

        channel_autonomous_runs_set = cls(
            allow_autonomous_runs=allow_autonomous_runs,
        )

        return channel_autonomous_runs_set
