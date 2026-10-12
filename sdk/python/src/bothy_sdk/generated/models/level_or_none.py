from enum import StrEnum


class LevelOrNone(StrEnum):
    NONE = "none"
    READ = "read"
    TRIGGER = "trigger"
    WRITE = "write"

    def __str__(self) -> str:
        return str(self.value)
