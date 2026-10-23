from enum import StrEnum


class ImportSourceDiscordConfigChannelsItemLead(StrEnum):
    BOTHY = "bothy"
    DISCORD = "discord"

    def __str__(self) -> str:
        return str(self.value)
