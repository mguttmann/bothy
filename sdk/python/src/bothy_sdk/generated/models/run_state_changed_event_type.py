from enum import StrEnum


class RunStateChangedEventType(StrEnum):
    RUN_STATE_CHANGED = "run.state_changed"

    def __str__(self) -> str:
        return str(self.value)
