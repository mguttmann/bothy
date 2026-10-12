from enum import StrEnum


class ThreadCreatedEventType(StrEnum):
    THREAD_CREATED = "thread.created"

    def __str__(self) -> str:
        return str(self.value)
