from enum import StrEnum


class NotificationLevel(StrEnum):
    ALL = "all"
    MENTIONS = "mentions"
    MUTE = "mute"

    def __str__(self) -> str:
        return str(self.value)
