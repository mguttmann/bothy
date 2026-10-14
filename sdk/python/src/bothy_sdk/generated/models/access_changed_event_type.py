from enum import StrEnum


class AccessChangedEventType(StrEnum):
    ACCESS_CHANGED = "access.changed"

    def __str__(self) -> str:
        return str(self.value)
