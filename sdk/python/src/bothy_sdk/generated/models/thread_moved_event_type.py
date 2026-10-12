from enum import StrEnum


class ThreadMovedEventType(StrEnum):
    THREAD_MOVED = "thread.moved"

    def __str__(self) -> str:
        return str(self.value)
