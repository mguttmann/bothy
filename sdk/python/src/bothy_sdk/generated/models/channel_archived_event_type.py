from enum import StrEnum


class ChannelArchivedEventType(StrEnum):
    CHANNEL_ARCHIVED = "channel.archived"

    def __str__(self) -> str:
        return str(self.value)
