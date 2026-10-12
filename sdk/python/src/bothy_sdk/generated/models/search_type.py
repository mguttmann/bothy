from enum import StrEnum


class SearchType(StrEnum):
    FILES = "files"
    MESSAGES = "messages"
    THREADS = "threads"

    def __str__(self) -> str:
        return str(self.value)
