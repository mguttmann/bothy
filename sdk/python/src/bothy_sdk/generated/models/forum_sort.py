from enum import StrEnum


class ForumSort(StrEnum):
    CREATED = "created"
    LAST_ACTIVITY = "last_activity"

    def __str__(self) -> str:
        return str(self.value)
