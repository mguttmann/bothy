from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ImportSourceDiscordConfigSpacesItem")


@_attrs_define
class ImportSourceDiscordConfigSpacesItem:
    """
    Attributes:
        discord_category (str): A Discord snowflake as a string of digits.
        target (Any): Exactly one of the string `skip`, `{create: {name}}` or `{existing: <uuid>}` (22, 5.2). The server
            checks the form and reports a violation at the pointer of the member.
    """

    discord_category: str
    target: Any

    def to_dict(self) -> dict[str, Any]:
        discord_category = self.discord_category

        target = self.target

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "discord_category": discord_category,
                "target": target,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        discord_category = d.pop("discord_category")

        target = d.pop("target")

        import_source_discord_config_spaces_item = cls(
            discord_category=discord_category,
            target=target,
        )

        return import_source_discord_config_spaces_item
