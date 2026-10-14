from enum import StrEnum


class QuietHoursUpdatedEventType(StrEnum):
    QUIET_HOURS_UPDATED = "quiet_hours.updated"

    def __str__(self) -> str:
        return str(self.value)
