from enum import StrEnum


class CommandLevel(StrEnum):
    TRIGGER = "trigger"
    WRITE = "write"

    def __str__(self) -> str:
        return str(self.value)
