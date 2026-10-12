from enum import StrEnum


class StatsLiveRunState(StrEnum):
    QUEUED = "queued"
    RUNNING = "running"
    WAITING_INPUT = "waiting_input"

    def __str__(self) -> str:
        return str(self.value)
