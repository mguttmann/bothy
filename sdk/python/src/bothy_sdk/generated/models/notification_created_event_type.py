from enum import StrEnum


class NotificationCreatedEventType(StrEnum):
    NOTIFICATION_CREATED = "notification.created"

    def __str__(self) -> str:
        return str(self.value)
