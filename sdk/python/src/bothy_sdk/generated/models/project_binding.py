from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="ProjectBinding")


@_attrs_define
class ProjectBinding:
    """
    Attributes:
        workspace_ref (str): Opaque reference the runner resolves; never a server path.
        runner_config (JsonObject): Free-form JSON object whose schema is named in the field description.
        ready (bool):
        version (int):
        repository_url (str | Unset): Display only; the server never fetches this URL.
        default_branch (str | Unset):
    """

    workspace_ref: str
    runner_config: JsonObject
    ready: bool
    version: int
    repository_url: str | Unset = UNSET
    default_branch: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        workspace_ref = self.workspace_ref

        runner_config = self.runner_config.to_dict()

        ready = self.ready

        version = self.version

        repository_url = self.repository_url

        default_branch = self.default_branch

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "workspace_ref": workspace_ref,
                "runner_config": runner_config,
                "ready": ready,
                "version": version,
            }
        )
        if repository_url is not UNSET:
            field_dict["repository_url"] = repository_url
        if default_branch is not UNSET:
            field_dict["default_branch"] = default_branch

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        workspace_ref = d.pop("workspace_ref")

        runner_config = JsonObject.from_dict(d.pop("runner_config"))

        ready = d.pop("ready")

        version = d.pop("version")

        repository_url = d.pop("repository_url", UNSET)

        default_branch = d.pop("default_branch", UNSET)

        project_binding = cls(
            workspace_ref=workspace_ref,
            runner_config=runner_config,
            ready=ready,
            version=version,
            repository_url=repository_url,
            default_branch=default_branch,
        )

        project_binding.additional_properties = d
        return project_binding

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
