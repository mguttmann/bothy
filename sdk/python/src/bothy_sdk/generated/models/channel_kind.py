from enum import StrEnum


class ChannelKind(StrEnum):
    DM = "dm"
    FORUM = "forum"
    TEXT = "text"

    def __str__(self) -> str:
        return str(self.value)
