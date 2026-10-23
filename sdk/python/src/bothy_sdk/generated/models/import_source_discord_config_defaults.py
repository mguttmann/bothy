from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.import_source_discord_config_defaults_unmapped_bot import (
    ImportSourceDiscordConfigDefaultsUnmappedBot,
)
from ..models.import_source_discord_config_defaults_unmapped_human import (
    ImportSourceDiscordConfigDefaultsUnmappedHuman,
)

T = TypeVar("T", bound="ImportSourceDiscordConfigDefaults")


@_attrs_define
class ImportSourceDiscordConfigDefaults:
    """
    Attributes:
        unmapped_human (ImportSourceDiscordConfigDefaultsUnmappedHuman):
        unmapped_bot (ImportSourceDiscordConfigDefaultsUnmappedBot):
        uncategorized_space (str): `skip` or the `discord_category` of an entry in `spaces`.
    """

    unmapped_human: ImportSourceDiscordConfigDefaultsUnmappedHuman
    unmapped_bot: ImportSourceDiscordConfigDefaultsUnmappedBot
    uncategorized_space: str

    def to_dict(self) -> dict[str, Any]:
        unmapped_human = self.unmapped_human.value

        unmapped_bot = self.unmapped_bot.value

        uncategorized_space = self.uncategorized_space

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "unmapped_human": unmapped_human,
                "unmapped_bot": unmapped_bot,
                "uncategorized_space": uncategorized_space,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        unmapped_human = ImportSourceDiscordConfigDefaultsUnmappedHuman(
            d.pop("unmapped_human")
        )

        unmapped_bot = ImportSourceDiscordConfigDefaultsUnmappedBot(
            d.pop("unmapped_bot")
        )

        uncategorized_space = d.pop("uncategorized_space")

        import_source_discord_config_defaults = cls(
            unmapped_human=unmapped_human,
            unmapped_bot=unmapped_bot,
            uncategorized_space=uncategorized_space,
        )

        return import_source_discord_config_defaults
