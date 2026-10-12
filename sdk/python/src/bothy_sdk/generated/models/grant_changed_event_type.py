from enum import StrEnum


class GrantChangedEventType(StrEnum):
    GRANT_CHANGED = "grant.changed"

    def __str__(self) -> str:
        return str(self.value)
