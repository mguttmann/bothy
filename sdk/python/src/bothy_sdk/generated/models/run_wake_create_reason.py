from enum import StrEnum


class RunWakeCreateReason(StrEnum):
    AGENT = "agent"
    USAGE_LIMIT = "usage_limit"

    def __str__(self) -> str:
        return str(self.value)
