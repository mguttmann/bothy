from enum import StrEnum


class ThreadKind(StrEnum):
    MAIN = "main"
    THREAD = "thread"

    def __str__(self) -> str:
        return str(self.value)
