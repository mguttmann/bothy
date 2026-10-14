from enum import StrEnum


class ImportSourceState(StrEnum):
    COMPLETED = "completed"
    DRAFT = "draft"
    FAILED = "failed"
    PAUSED = "paused"
    RUNNING = "running"

    def __str__(self) -> str:
        return str(self.value)
