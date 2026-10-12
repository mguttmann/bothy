from enum import StrEnum


class NotificationPrefUpdatedEventType(StrEnum):
    NOTIFICATION_PREF_UPDATED = "notification_pref.updated"

    def __str__(self) -> str:
        return str(self.value)
