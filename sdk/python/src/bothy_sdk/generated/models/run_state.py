from enum import StrEnum


class RunState(StrEnum):
    CANCELLED = "cancelled"
    COMPLETED = "completed"
    FAILED = "failed"
    QUEUED = "queued"
    RUNNING = "running"
    WAITING_INPUT = "waiting_input"

    def __str__(self) -> str:
        return str(self.value)
