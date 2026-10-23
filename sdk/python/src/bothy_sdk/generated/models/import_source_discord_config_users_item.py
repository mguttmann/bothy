from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ImportSourceDiscordConfigUsersItem")


@_attrs_define
class ImportSourceDiscordConfigUsersItem:
    """
    Attributes:
        discord_user (str): A Discord snowflake as a string of digits.
        target (str): `user:<handle>`, `agent:<handle>` or `placeholder`.
    """

    discord_user: str
    target: str

    def to_dict(self) -> dict[str, Any]:
        discord_user = self.discord_user

        target = self.target

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "discord_user": discord_user,
                "target": target,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        discord_user = d.pop("discord_user")

        target = d.pop("target")

        import_source_discord_config_users_item = cls(
            discord_user=discord_user,
            target=target,
        )

        return import_source_discord_config_users_item
