from enum import StrEnum


class RunWakeRecoverReason(StrEnum):
    RECOVERY = "recovery"
    USAGE_LIMIT = "usage_limit"

    def __str__(self) -> str:
        return str(self.value)
