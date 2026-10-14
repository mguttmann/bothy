from enum import StrEnum


class Level(StrEnum):
    READ = "read"
    TRIGGER = "trigger"
    WRITE = "write"

    def __str__(self) -> str:
        return str(self.value)
