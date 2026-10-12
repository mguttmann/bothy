from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="AgentCreate")


@_attrs_define
class AgentCreate:
    """
    Attributes:
        handle (str):
        display_name (str):
        adapter_kind (str):
        description (str | Unset):
        operator_config (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
    """

    handle: str
    display_name: str
    adapter_kind: str
    description: str | Unset = UNSET
    operator_config: JsonObject | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        handle = self.handle

        display_name = self.display_name

        adapter_kind = self.adapter_kind

        description = self.description

        operator_config: dict[str, Any] | Unset = UNSET
        if not isinstance(self.operator_config, Unset):
            operator_config = self.operator_config.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "handle": handle,
                "display_name": display_name,
                "adapter_kind": adapter_kind,
            }
        )
        if description is not UNSET:
            field_dict["description"] = description
        if operator_config is not UNSET:
            field_dict["operator_config"] = operator_config

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        handle = d.pop("handle")

        display_name = d.pop("display_name")

        adapter_kind = d.pop("adapter_kind")

        description = d.pop("description", UNSET)

        _operator_config = d.pop("operator_config", UNSET)
        operator_config: JsonObject | Unset
        if isinstance(_operator_config, Unset):
            operator_config = UNSET
        else:
            operator_config = JsonObject.from_dict(_operator_config)

        agent_create = cls(
            handle=handle,
            display_name=display_name,
            adapter_kind=adapter_kind,
            description=description,
            operator_config=operator_config,
        )

        return agent_create
