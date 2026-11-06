from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..models.tool_class import ToolClass
from ..types import UNSET, Unset

T = TypeVar("T", bound="AgentDefinitionSettings")


@_attrs_define
class AgentDefinitionSettings:
    """Settings of the built-in agent; the tool class itself is the agent's `tool_class` (07 2.11), never set here.

    Attributes:
        tool_class_wish (ToolClass): Tool class of an agent runtime, ordered `chat` < `scoped` < `host` (07, 2.11).
        model (str | Unset): The pattern of `default_model` (09 2.2), not starting with `-`; absent means the runner's
            default model.
        allowed_tools (list[str] | Unset): At most `agents.definition.max_allowed_tools` distinct tool names (a CLI tool
            name, optionally with a rule in parentheses, or `mcp__<server>[__<tool>]` of a listed server). Under `scoped` it
            narrows `runner.scoped_allowed_tools` of the hosting runner; absent means no narrowing.
        max_cost_usd_per_run (float | Unset): Cost cap of one run in USD, above zero and at most
            `agents.definition.max_cost_usd_per_run`; absent means no cap.
    """

    tool_class_wish: ToolClass
    model: str | Unset = UNSET
    allowed_tools: list[str] | Unset = UNSET
    max_cost_usd_per_run: float | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        tool_class_wish = self.tool_class_wish.value

        model = self.model

        allowed_tools: list[str] | Unset = UNSET
        if not isinstance(self.allowed_tools, Unset):
            allowed_tools = self.allowed_tools

        max_cost_usd_per_run = self.max_cost_usd_per_run

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "tool_class_wish": tool_class_wish,
            }
        )
        if model is not UNSET:
            field_dict["model"] = model
        if allowed_tools is not UNSET:
            field_dict["allowed_tools"] = allowed_tools
        if max_cost_usd_per_run is not UNSET:
            field_dict["max_cost_usd_per_run"] = max_cost_usd_per_run

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        tool_class_wish = ToolClass(d.pop("tool_class_wish"))

        model = d.pop("model", UNSET)

        allowed_tools = cast(list[str], d.pop("allowed_tools", UNSET))

        max_cost_usd_per_run = d.pop("max_cost_usd_per_run", UNSET)

        agent_definition_settings = cls(
            tool_class_wish=tool_class_wish,
            model=model,
            allowed_tools=allowed_tools,
            max_cost_usd_per_run=max_cost_usd_per_run,
        )

        return agent_definition_settings
