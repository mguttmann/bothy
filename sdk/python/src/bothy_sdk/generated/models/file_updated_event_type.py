from enum import StrEnum


class FileUpdatedEventType(StrEnum):
    FILE_UPDATED = "file.updated"

    def __str__(self) -> str:
        return str(self.value)
