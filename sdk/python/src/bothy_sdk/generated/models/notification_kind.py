from enum import StrEnum


class NotificationKind(StrEnum):
    ADMIN = "admin"
    DM = "dm"
    INPUT_NEEDED = "input_needed"
    MENTION = "mention"
    MESSAGE = "message"
    RUN_DONE = "run_done"
    RUN_FAILED = "run_failed"
    SECURITY = "security"

    def __str__(self) -> str:
        return str(self.value)
