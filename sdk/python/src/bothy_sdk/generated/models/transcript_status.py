from enum import StrEnum


class TranscriptStatus(StrEnum):
    DONE = "done"
    FAILED = "failed"
    NONE = "none"
    PENDING = "pending"

    def __str__(self) -> str:
        return str(self.value)
