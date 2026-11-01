from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.project_folder_deletion_trash_state import ProjectFolderDeletionTrashState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.project_trash_check import ProjectTrashCheck


T = TypeVar("T", bound="ProjectFolderDeletion")


@_attrs_define
class ProjectFolderDeletion:
    """The project folder of a forum with a project binding; only for owners (`channel.project.bind`).

    Attributes:
        workspace_ref (str):
        trash_check (ProjectTrashCheck | Unset): Whether the serving runner would move the folder to the trash now (09
            18.15). Absent in `ProjectFolderDeletion` before the runner's first check.
        trash_state (ProjectFolderDeletionTrashState | Unset): Absent when no move was requested; `requested` until the
            runner reports.
        trash_reason (str | Unset): The refusal code of the runner with `trash_state` `refused` (09 18.15).
    """

    workspace_ref: str
    trash_check: ProjectTrashCheck | Unset = UNSET
    trash_state: ProjectFolderDeletionTrashState | Unset = UNSET
    trash_reason: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        workspace_ref = self.workspace_ref

        trash_check: dict[str, Any] | Unset = UNSET
        if not isinstance(self.trash_check, Unset):
            trash_check = self.trash_check.to_dict()

        trash_state: str | Unset = UNSET
        if not isinstance(self.trash_state, Unset):
            trash_state = self.trash_state.value

        trash_reason = self.trash_reason

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "workspace_ref": workspace_ref,
            }
        )
        if trash_check is not UNSET:
            field_dict["trash_check"] = trash_check
        if trash_state is not UNSET:
            field_dict["trash_state"] = trash_state
        if trash_reason is not UNSET:
            field_dict["trash_reason"] = trash_reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.project_trash_check import ProjectTrashCheck

        d = dict(src_dict)
        workspace_ref = d.pop("workspace_ref")

        _trash_check = d.pop("trash_check", UNSET)
        trash_check: ProjectTrashCheck | Unset
        if isinstance(_trash_check, Unset):
            trash_check = UNSET
        else:
            trash_check = ProjectTrashCheck.from_dict(_trash_check)

        _trash_state = d.pop("trash_state", UNSET)
        trash_state: ProjectFolderDeletionTrashState | Unset
        if isinstance(_trash_state, Unset):
            trash_state = UNSET
        else:
            trash_state = ProjectFolderDeletionTrashState(_trash_state)

        trash_reason = d.pop("trash_reason", UNSET)

        project_folder_deletion = cls(
            workspace_ref=workspace_ref,
            trash_check=trash_check,
            trash_state=trash_state,
            trash_reason=trash_reason,
        )

        project_folder_deletion.additional_properties = d
        return project_folder_deletion

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
