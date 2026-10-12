from enum import StrEnum


class ChannelUpdatedEventType(StrEnum):
    CHANNEL_UPDATED = "channel.updated"

    def __str__(self) -> str:
        return str(self.value)
