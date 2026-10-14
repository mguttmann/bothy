from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="AgentTriggerers")


@_attrs_define
class AgentTriggerers:
    """Principals that may trigger the calling agent (allow list of the adapter).

    Attributes:
        principal_ids (list[UUID]):
        permission_version (int):
    """

    principal_ids: list[UUID]
    permission_version: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        principal_ids = []
        for principal_ids_item_data in self.principal_ids:
            principal_ids_item = str(principal_ids_item_data)
            principal_ids.append(principal_ids_item)

        permission_version = self.permission_version

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "principal_ids": principal_ids,
                "permission_version": permission_version,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        principal_ids = []
        _principal_ids = d.pop("principal_ids")
        for principal_ids_item_data in _principal_ids:
            principal_ids_item = UUID(principal_ids_item_data)

            principal_ids.append(principal_ids_item)

        permission_version = d.pop("permission_version")

        agent_triggerers = cls(
            principal_ids=principal_ids,
            permission_version=permission_version,
        )

        agent_triggerers.additional_properties = d
        return agent_triggerers

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
