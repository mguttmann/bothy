from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ProjectRunnerConfig")


@_attrs_define
class ProjectRunnerConfig:
    """Schema `project_runner_config.v1`. No secrets.

    Attributes:
        model (str | Unset):
        allowed_tools (list[str] | Unset):
    """

    model: str | Unset = UNSET
    allowed_tools: list[str] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        model = self.model

        allowed_tools: list[str] | Unset = UNSET
        if not isinstance(self.allowed_tools, Unset):
            allowed_tools = self.allowed_tools

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if model is not UNSET:
            field_dict["model"] = model
        if allowed_tools is not UNSET:
            field_dict["allowed_tools"] = allowed_tools

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        model = d.pop("model", UNSET)

        allowed_tools = cast(list[str], d.pop("allowed_tools", UNSET))

        project_runner_config = cls(
            model=model,
            allowed_tools=allowed_tools,
        )

        return project_runner_config
