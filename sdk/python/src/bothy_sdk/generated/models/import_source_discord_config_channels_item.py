from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.import_source_discord_config_channels_item_lead import (
    ImportSourceDiscordConfigChannelsItemLead,
)
from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportSourceDiscordConfigChannelsItem")


@_attrs_define
class ImportSourceDiscordConfigChannelsItem:
    """
    Attributes:
        discord_channel (str): A Discord snowflake as a string of digits.
        target (Any): Exactly one of the string `skip`, `{create: {name}}` or `{existing: <uuid>}` (22, 5.2). The server
            checks the form and reports a violation at the pointer of the member.
        planned_operator (str | Unset): `agent:<handle>` of an active agent or `none` (default).
        lead (ImportSourceDiscordConfigChannelsItemLead | Unset):  Default:
            ImportSourceDiscordConfigChannelsItemLead.DISCORD.
    """

    discord_channel: str
    target: Any
    planned_operator: str | Unset = UNSET
    lead: ImportSourceDiscordConfigChannelsItemLead | Unset = (
        ImportSourceDiscordConfigChannelsItemLead.DISCORD
    )

    def to_dict(self) -> dict[str, Any]:
        discord_channel = self.discord_channel

        target = self.target

        planned_operator = self.planned_operator

        lead: str | Unset = UNSET
        if not isinstance(self.lead, Unset):
            lead = self.lead.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "discord_channel": discord_channel,
                "target": target,
            }
        )
        if planned_operator is not UNSET:
            field_dict["planned_operator"] = planned_operator
        if lead is not UNSET:
            field_dict["lead"] = lead

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        discord_channel = d.pop("discord_channel")

        target = d.pop("target")

        planned_operator = d.pop("planned_operator", UNSET)

        _lead = d.pop("lead", UNSET)
        lead: ImportSourceDiscordConfigChannelsItemLead | Unset
        if isinstance(_lead, Unset):
            lead = UNSET
        else:
            lead = ImportSourceDiscordConfigChannelsItemLead(_lead)

        import_source_discord_config_channels_item = cls(
            discord_channel=discord_channel,
            target=target,
            planned_operator=planned_operator,
            lead=lead,
        )

        return import_source_discord_config_channels_item
