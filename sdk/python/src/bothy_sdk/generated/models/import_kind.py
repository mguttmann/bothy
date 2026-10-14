from enum import StrEnum


class ImportKind(StrEnum):
    ARCHIVE = "archive"
    DISCORD = "discord"

    def __str__(self) -> str:
        return str(self.value)
