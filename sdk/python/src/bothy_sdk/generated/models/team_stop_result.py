from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="TeamStopResult")


@_attrs_define
class TeamStopResult:
    """
    Attributes:
        stopped_run_ids (list[UUID]): The runs the stop ended or asked to end.
    """

    stopped_run_ids: list[UUID]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        stopped_run_ids = []
        for stopped_run_ids_item_data in self.stopped_run_ids:
            stopped_run_ids_item = str(stopped_run_ids_item_data)
            stopped_run_ids.append(stopped_run_ids_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "stopped_run_ids": stopped_run_ids,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        stopped_run_ids = []
        _stopped_run_ids = d.pop("stopped_run_ids")
        for stopped_run_ids_item_data in _stopped_run_ids:
            stopped_run_ids_item = UUID(stopped_run_ids_item_data)

            stopped_run_ids.append(stopped_run_ids_item)

        team_stop_result = cls(
            stopped_run_ids=stopped_run_ids,
        )

        team_stop_result.additional_properties = d
        return team_stop_result

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
