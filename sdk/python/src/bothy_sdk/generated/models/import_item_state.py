from enum import StrEnum


class ImportItemState(StrEnum):
    FAILED = "failed"
    PENDING = "pending"
    POSTED = "posted"
    SKIPPED = "skipped"

    def __str__(self) -> str:
        return str(self.value)
