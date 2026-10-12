from enum import StrEnum


class ImportObjectItemKind(StrEnum):
    CHANNEL = "channel"
    FORUM_TAG = "forum_tag"
    MESSAGE = "message"
    PIN = "pin"
    SPACE = "space"
    THREAD = "thread"
    USER = "user"

    def __str__(self) -> str:
        return str(self.value)
