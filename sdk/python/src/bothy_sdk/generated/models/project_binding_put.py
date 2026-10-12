from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="ProjectBindingPut")


@_attrs_define
class ProjectBindingPut:
    """
    Attributes:
        workspace_ref (str):
        reason (str):
        repository_url (str | Unset):
        default_branch (str | Unset):
        runner_config (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
    """

    workspace_ref: str
    reason: str
    repository_url: str | Unset = UNSET
    default_branch: str | Unset = UNSET
    runner_config: JsonObject | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        workspace_ref = self.workspace_ref

        reason = self.reason

        repository_url = self.repository_url

        default_branch = self.default_branch

        runner_config: dict[str, Any] | Unset = UNSET
        if not isinstance(self.runner_config, Unset):
            runner_config = self.runner_config.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "workspace_ref": workspace_ref,
                "reason": reason,
            }
        )
        if repository_url is not UNSET:
            field_dict["repository_url"] = repository_url
        if default_branch is not UNSET:
            field_dict["default_branch"] = default_branch
        if runner_config is not UNSET:
            field_dict["runner_config"] = runner_config

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        workspace_ref = d.pop("workspace_ref")

        reason = d.pop("reason")

        repository_url = d.pop("repository_url", UNSET)

        default_branch = d.pop("default_branch", UNSET)

        _runner_config = d.pop("runner_config", UNSET)
        runner_config: JsonObject | Unset
        if isinstance(_runner_config, Unset):
            runner_config = UNSET
        else:
            runner_config = JsonObject.from_dict(_runner_config)

        project_binding_put = cls(
            workspace_ref=workspace_ref,
            reason=reason,
            repository_url=repository_url,
            default_branch=default_branch,
            runner_config=runner_config,
        )

        return project_binding_put
