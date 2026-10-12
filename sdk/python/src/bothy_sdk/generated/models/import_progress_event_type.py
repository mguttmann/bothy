from enum import StrEnum


class ImportProgressEventType(StrEnum):
    IMPORT_PROGRESS = "import.progress"

    def __str__(self) -> str:
        return str(self.value)
