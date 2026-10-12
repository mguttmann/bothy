from enum import StrEnum


class StatusUpdatedEventType(StrEnum):
    STATUS_UPDATED = "status.updated"

    def __str__(self) -> str:
        return str(self.value)
