from enum import StrEnum


class WaitingRunReason(StrEnum):
    SLOTS_FULL = "slots_full"
    THREAD_BUSY = "thread_busy"

    def __str__(self) -> str:
        return str(self.value)
