from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="ProjectTrashCheck")


@_attrs_define
class ProjectTrashCheck:
    """Whether the serving runner would move the folder to the trash now (09 18.15). Absent in `ProjectFolderDeletion`
    before the runner's first check.

        Attributes:
            ok (bool):
            reason (str | Unset): Why not, with `ok` false; a code of 09 18.15 (`outside_root`, `symlink`,
                `foreign_worktree`, `uncommitted_changes`, `unpushed_commits`, `contains_other_project`, `workspace_missing`,
                `check_failed`).
            checked_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    ok: bool
    reason: str | Unset = UNSET
    checked_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        ok = self.ok

        reason = self.reason

        checked_at: str | Unset = UNSET
        if not isinstance(self.checked_at, Unset):
            checked_at = self.checked_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "ok": ok,
            }
        )
        if reason is not UNSET:
            field_dict["reason"] = reason
        if checked_at is not UNSET:
            field_dict["checked_at"] = checked_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        ok = d.pop("ok")

        reason = d.pop("reason", UNSET)

        _checked_at = d.pop("checked_at", UNSET)
        checked_at: datetime.datetime | Unset
        if isinstance(_checked_at, Unset):
            checked_at = UNSET
        else:
            checked_at = datetime.datetime.fromisoformat(_checked_at)

        project_trash_check = cls(
            ok=ok,
            reason=reason,
            checked_at=checked_at,
        )

        project_trash_check.additional_properties = d
        return project_trash_check

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
