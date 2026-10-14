from enum import StrEnum


class ThreadClosedEventType(StrEnum):
    THREAD_CLOSED = "thread.closed"

    def __str__(self) -> str:
        return str(self.value)
