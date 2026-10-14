from enum import StrEnum


class ChannelCreateKind(StrEnum):
    FORUM = "forum"
    TEXT = "text"

    def __str__(self) -> str:
        return str(self.value)
