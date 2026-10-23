from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.import_user_data_discord_kind import ImportUserDataDiscordKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportUserData")


@_attrs_define
class ImportUserData:
    """`data` of a `user` item (origin key `user:<user_id>`). The target comes from the stored mapping and its `defaults`;
    a webhook author is always a placeholder (22, 5.3).

        Attributes:
            discord_kind (ImportUserDataDiscordKind):
            username (str):
            global_name (str | Unset):
    """

    discord_kind: ImportUserDataDiscordKind
    username: str
    global_name: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        discord_kind = self.discord_kind.value

        username = self.username

        global_name = self.global_name

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "discord_kind": discord_kind,
                "username": username,
            }
        )
        if global_name is not UNSET:
            field_dict["global_name"] = global_name

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        discord_kind = ImportUserDataDiscordKind(d.pop("discord_kind"))

        username = d.pop("username")

        global_name = d.pop("global_name", UNSET)

        import_user_data = cls(
            discord_kind=discord_kind,
            username=username,
            global_name=global_name,
        )

        return import_user_data
