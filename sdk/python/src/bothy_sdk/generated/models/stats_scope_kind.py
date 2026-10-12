from enum import StrEnum


class StatsScopeKind(StrEnum):
    INSTANCE = "instance"
    SPACE = "space"

    def __str__(self) -> str:
        return str(self.value)
