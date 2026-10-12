from enum import StrEnum


class ThreadUpdatedEventType(StrEnum):
    THREAD_UPDATED = "thread.updated"

    def __str__(self) -> str:
        return str(self.value)
