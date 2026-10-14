from enum import StrEnum


class ImportItemItemKind(StrEnum):
    CHANNEL = "channel"
    FILE = "file"
    FORUM_TAG = "forum_tag"
    MESSAGE = "message"
    PIN = "pin"
    REACTION = "reaction"
    SPACE = "space"
    THREAD = "thread"
    USER = "user"

    def __str__(self) -> str:
        return str(self.value)
