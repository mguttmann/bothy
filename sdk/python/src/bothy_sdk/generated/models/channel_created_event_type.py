from enum import StrEnum


class ChannelCreatedEventType(StrEnum):
    CHANNEL_CREATED = "channel.created"

    def __str__(self) -> str:
        return str(self.value)
