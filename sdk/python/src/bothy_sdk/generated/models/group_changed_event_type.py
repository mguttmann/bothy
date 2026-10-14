from enum import StrEnum


class GroupChangedEventType(StrEnum):
    GROUP_CHANGED = "group.changed"

    def __str__(self) -> str:
        return str(self.value)
