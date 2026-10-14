from enum import StrEnum


class ThreadReopenedEventType(StrEnum):
    THREAD_REOPENED = "thread.reopened"

    def __str__(self) -> str:
        return str(self.value)
