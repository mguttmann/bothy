from enum import StrEnum


class SearchHitKind(StrEnum):
    FILE = "file"
    MESSAGE = "message"
    THREAD = "thread"

    def __str__(self) -> str:
        return str(self.value)
