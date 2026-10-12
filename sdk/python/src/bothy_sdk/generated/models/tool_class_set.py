from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.tool_class import ToolClass

T = TypeVar("T", bound="ToolClassSet")


@_attrs_define
class ToolClassSet:
    """
    Attributes:
        tool_class (ToolClass): Tool class of an agent runtime, ordered `chat` < `scoped` < `host` (07, 2.11).
        reason (str):
    """

    tool_class: ToolClass
    reason: str

    def to_dict(self) -> dict[str, Any]:
        tool_class = self.tool_class.value

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "tool_class": tool_class,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        tool_class = ToolClass(d.pop("tool_class"))

        reason = d.pop("reason")

        tool_class_set = cls(
            tool_class=tool_class,
            reason=reason,
        )

        return tool_class_set
