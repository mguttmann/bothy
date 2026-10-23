from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.import_source_discord_config_version import (
    ImportSourceDiscordConfigVersion,
)
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.import_source_discord_config_channels_item import (
        ImportSourceDiscordConfigChannelsItem,
    )
    from ..models.import_source_discord_config_defaults import (
        ImportSourceDiscordConfigDefaults,
    )
    from ..models.import_source_discord_config_spaces_item import (
        ImportSourceDiscordConfigSpacesItem,
    )
    from ..models.import_source_discord_config_users_item import (
        ImportSourceDiscordConfigUsersItem,
    )


T = TypeVar("T", bound="ImportSourceDiscordConfig")


@_attrs_define
class ImportSourceDiscordConfig:
    """Mapping file of schema `import_source.discord.v1` as JSON (22, 5.2); the web console converts the YAML of the owner.
    It holds no secret.

        Attributes:
            version (ImportSourceDiscordConfigVersion):
            guild_id (str): A Discord snowflake as a string of digits.
            defaults (ImportSourceDiscordConfigDefaults):
            spaces (list[ImportSourceDiscordConfigSpacesItem] | Unset):
            channels (list[ImportSourceDiscordConfigChannelsItem] | Unset):
            users (list[ImportSourceDiscordConfigUsersItem] | Unset):
    """

    version: ImportSourceDiscordConfigVersion
    guild_id: str
    defaults: ImportSourceDiscordConfigDefaults
    spaces: list[ImportSourceDiscordConfigSpacesItem] | Unset = UNSET
    channels: list[ImportSourceDiscordConfigChannelsItem] | Unset = UNSET
    users: list[ImportSourceDiscordConfigUsersItem] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        version = self.version.value

        guild_id = self.guild_id

        defaults = self.defaults.to_dict()

        spaces: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.spaces, Unset):
            spaces = []
            for spaces_item_data in self.spaces:
                spaces_item = spaces_item_data.to_dict()
                spaces.append(spaces_item)

        channels: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.channels, Unset):
            channels = []
            for channels_item_data in self.channels:
                channels_item = channels_item_data.to_dict()
                channels.append(channels_item)

        users: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.users, Unset):
            users = []
            for users_item_data in self.users:
                users_item = users_item_data.to_dict()
                users.append(users_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "version": version,
                "guild_id": guild_id,
                "defaults": defaults,
            }
        )
        if spaces is not UNSET:
            field_dict["spaces"] = spaces
        if channels is not UNSET:
            field_dict["channels"] = channels
        if users is not UNSET:
            field_dict["users"] = users

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.import_source_discord_config_channels_item import (
            ImportSourceDiscordConfigChannelsItem,
        )
        from ..models.import_source_discord_config_defaults import (
            ImportSourceDiscordConfigDefaults,
        )
        from ..models.import_source_discord_config_spaces_item import (
            ImportSourceDiscordConfigSpacesItem,
        )
        from ..models.import_source_discord_config_users_item import (
            ImportSourceDiscordConfigUsersItem,
        )

        d = dict(src_dict)
        version = ImportSourceDiscordConfigVersion(d.pop("version"))

        guild_id = d.pop("guild_id")

        defaults = ImportSourceDiscordConfigDefaults.from_dict(d.pop("defaults"))

        _spaces = d.pop("spaces", UNSET)
        spaces: list[ImportSourceDiscordConfigSpacesItem] | Unset = UNSET
        if _spaces is not UNSET:
            spaces = []
            for spaces_item_data in _spaces:
                spaces_item = ImportSourceDiscordConfigSpacesItem.from_dict(
                    spaces_item_data
                )

                spaces.append(spaces_item)

        _channels = d.pop("channels", UNSET)
        channels: list[ImportSourceDiscordConfigChannelsItem] | Unset = UNSET
        if _channels is not UNSET:
            channels = []
            for channels_item_data in _channels:
                channels_item = ImportSourceDiscordConfigChannelsItem.from_dict(
                    channels_item_data
                )

                channels.append(channels_item)

        _users = d.pop("users", UNSET)
        users: list[ImportSourceDiscordConfigUsersItem] | Unset = UNSET
        if _users is not UNSET:
            users = []
            for users_item_data in _users:
                users_item = ImportSourceDiscordConfigUsersItem.from_dict(
                    users_item_data
                )

                users.append(users_item)

        import_source_discord_config = cls(
            version=version,
            guild_id=guild_id,
            defaults=defaults,
            spaces=spaces,
            channels=channels,
            users=users,
        )

        return import_source_discord_config
