from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.deletion_info_state import DeletionInfoState
from ..models.deletion_info_target_kind import DeletionInfoTargetKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.deletion_contents import DeletionContents
    from ..models.project_folder_deletion import ProjectFolderDeletion


T = TypeVar("T", bound="DeletionInfo")


@_attrs_define
class DeletionInfo:
    """
    Attributes:
        target_kind (DeletionInfoTargetKind):
        target_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        state (DeletionInfoState): `live` before a deletion (a preview), `pending` while it can be undone, `final` after
            `undo_until`.
        contents (DeletionContents): What the deletion removes, counted when read; for a pending or final deletion what
            it removed or removes.
        deleted_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        undo_until (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        project_folder (ProjectFolderDeletion | Unset): The project folder of a forum with a project binding; only for
            owners (`channel.project.bind`).
    """

    target_kind: DeletionInfoTargetKind
    target_id: UUID
    state: DeletionInfoState
    contents: DeletionContents
    deleted_at: datetime.datetime | Unset = UNSET
    undo_until: datetime.datetime | Unset = UNSET
    project_folder: ProjectFolderDeletion | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        target_kind = self.target_kind.value

        target_id = str(self.target_id)

        state = self.state.value

        contents = self.contents.to_dict()

        deleted_at: str | Unset = UNSET
        if not isinstance(self.deleted_at, Unset):
            deleted_at = self.deleted_at.isoformat()

        undo_until: str | Unset = UNSET
        if not isinstance(self.undo_until, Unset):
            undo_until = self.undo_until.isoformat()

        project_folder: dict[str, Any] | Unset = UNSET
        if not isinstance(self.project_folder, Unset):
            project_folder = self.project_folder.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "target_kind": target_kind,
                "target_id": target_id,
                "state": state,
                "contents": contents,
            }
        )
        if deleted_at is not UNSET:
            field_dict["deleted_at"] = deleted_at
        if undo_until is not UNSET:
            field_dict["undo_until"] = undo_until
        if project_folder is not UNSET:
            field_dict["project_folder"] = project_folder

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.deletion_contents import DeletionContents
        from ..models.project_folder_deletion import ProjectFolderDeletion

        d = dict(src_dict)
        target_kind = DeletionInfoTargetKind(d.pop("target_kind"))

        target_id = UUID(d.pop("target_id"))

        state = DeletionInfoState(d.pop("state"))

        contents = DeletionContents.from_dict(d.pop("contents"))

        _deleted_at = d.pop("deleted_at", UNSET)
        deleted_at: datetime.datetime | Unset
        if isinstance(_deleted_at, Unset):
            deleted_at = UNSET
        else:
            deleted_at = datetime.datetime.fromisoformat(_deleted_at)

        _undo_until = d.pop("undo_until", UNSET)
        undo_until: datetime.datetime | Unset
        if isinstance(_undo_until, Unset):
            undo_until = UNSET
        else:
            undo_until = datetime.datetime.fromisoformat(_undo_until)

        _project_folder = d.pop("project_folder", UNSET)
        project_folder: ProjectFolderDeletion | Unset
        if isinstance(_project_folder, Unset):
            project_folder = UNSET
        else:
            project_folder = ProjectFolderDeletion.from_dict(_project_folder)

        deletion_info = cls(
            target_kind=target_kind,
            target_id=target_id,
            state=state,
            contents=contents,
            deleted_at=deleted_at,
            undo_until=undo_until,
            project_folder=project_folder,
        )

        deletion_info.additional_properties = d
        return deletion_info

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
