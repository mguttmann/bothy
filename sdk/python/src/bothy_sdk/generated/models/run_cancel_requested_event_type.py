from enum import StrEnum


class RunCancelRequestedEventType(StrEnum):
    RUN_CANCEL_REQUESTED = "run.cancel_requested"

    def __str__(self) -> str:
        return str(self.value)
