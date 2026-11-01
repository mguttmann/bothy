from enum import StrEnum


class StatsLiveTaskState(StrEnum):
    CANCELLED = "cancelled"
    COMPLETED = "completed"
    FAILED = "failed"
    RUNNING = "running"

    def __str__(self) -> str:
        return str(self.value)
