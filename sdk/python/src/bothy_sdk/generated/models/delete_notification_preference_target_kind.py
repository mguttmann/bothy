from enum import StrEnum


class DeleteNotificationPreferenceTargetKind(StrEnum):
    CHANNEL = "channel"
    SPACE = "space"
    THREAD = "thread"

    def __str__(self) -> str:
        return str(self.value)
