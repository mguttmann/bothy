from enum import StrEnum


class RunWakeReason(StrEnum):
    AGENT = "agent"
    RECOVERY = "recovery"
    USAGE_LIMIT = "usage_limit"

    def __str__(self) -> str:
        return str(self.value)
