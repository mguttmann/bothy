from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ChannelDelete")


@_attrs_define
class ChannelDelete:
    """
    Attributes:
        trash_project_folder (bool | Unset): Also move the project folder of a forum with a project binding to the trash
            of the serving runner's host once the deletion is final (09 18.15). Needs `channel.project.bind` and an ok
            folder check. Default: False.
    """

    trash_project_folder: bool | Unset = False

    def to_dict(self) -> dict[str, Any]:
        trash_project_folder = self.trash_project_folder

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if trash_project_folder is not UNSET:
            field_dict["trash_project_folder"] = trash_project_folder

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        trash_project_folder = d.pop("trash_project_folder", UNSET)

        channel_delete = cls(
            trash_project_folder=trash_project_folder,
        )

        return channel_delete
