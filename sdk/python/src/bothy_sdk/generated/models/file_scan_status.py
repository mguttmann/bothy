from enum import StrEnum


class FileScanStatus(StrEnum):
    CLEAN = "clean"
    ERROR = "error"
    INFECTED = "infected"
    PENDING = "pending"
    SKIPPED = "skipped"

    def __str__(self) -> str:
        return str(self.value)
