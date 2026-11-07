from enum import StrEnum


class RunWakeChangedEventType(StrEnum):
    RUN_WAKE_CHANGED = "run.wake_changed"

    def __str__(self) -> str:
        return str(self.value)
