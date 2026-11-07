from enum import StrEnum


class RunMetricsEndState(StrEnum):
    BLOCKED = "blocked"
    DONE = "done"
    WAITING = "waiting"

    def __str__(self) -> str:
        return str(self.value)
