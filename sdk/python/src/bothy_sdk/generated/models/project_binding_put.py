from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.project_provision import ProjectProvision
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.project_runner_config import ProjectRunnerConfig


T = TypeVar("T", bound="ProjectBindingPut")


@_attrs_define
class ProjectBindingPut:
    """
    Attributes:
        workspace_ref (str):
        reason (str):
        repository_url (str | Unset):
        default_branch (str | Unset):
        runner_config (ProjectRunnerConfig | Unset): Schema `project_runner_config.v1`. No secrets.
        provision (ProjectProvision | Unset): Whether the serving runner creates the folder of the binding (09 18.12).
            `none` creates nothing and the folder must exist; `init` creates an empty folder with `git init`; `clone` clones
            `repository_url`, which must be `https`. With `init` or `clone` the `workspace_ref` is one segment of
            `^[A-Za-z0-9][A-Za-z0-9._-]*$`. Absent means `none`; the server always sends it.
    """

    workspace_ref: str
    reason: str
    repository_url: str | Unset = UNSET
    default_branch: str | Unset = UNSET
    runner_config: ProjectRunnerConfig | Unset = UNSET
    provision: ProjectProvision | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        workspace_ref = self.workspace_ref

        reason = self.reason

        repository_url = self.repository_url

        default_branch = self.default_branch

        runner_config: dict[str, Any] | Unset = UNSET
        if not isinstance(self.runner_config, Unset):
            runner_config = self.runner_config.to_dict()

        provision: str | Unset = UNSET
        if not isinstance(self.provision, Unset):
            provision = self.provision.value

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
        if provision is not UNSET:
            field_dict["provision"] = provision

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.project_runner_config import ProjectRunnerConfig

        d = dict(src_dict)
        workspace_ref = d.pop("workspace_ref")

        reason = d.pop("reason")

        repository_url = d.pop("repository_url", UNSET)

        default_branch = d.pop("default_branch", UNSET)

        _runner_config = d.pop("runner_config", UNSET)
        runner_config: ProjectRunnerConfig | Unset
        if isinstance(_runner_config, Unset):
            runner_config = UNSET
        else:
            runner_config = ProjectRunnerConfig.from_dict(_runner_config)

        _provision = d.pop("provision", UNSET)
        provision: ProjectProvision | Unset
        if isinstance(_provision, Unset):
            provision = UNSET
        else:
            provision = ProjectProvision(_provision)

        project_binding_put = cls(
            workspace_ref=workspace_ref,
            reason=reason,
            repository_url=repository_url,
            default_branch=default_branch,
            runner_config=runner_config,
            provision=provision,
        )

        return project_binding_put
