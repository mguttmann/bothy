from enum import StrEnum


class ThreadMovedOutEventType(StrEnum):
    THREAD_MOVED_OUT = "thread.moved_out"

    def __str__(self) -> str:
        return str(self.value)
