from enum import StrEnum


class ImportChannelDataKind(StrEnum):
    FORUM = "forum"
    TEXT = "text"

    def __str__(self) -> str:
        return str(self.value)
