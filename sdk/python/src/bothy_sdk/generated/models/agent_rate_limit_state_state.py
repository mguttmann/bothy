from enum import StrEnum


class AgentRateLimitStateState(StrEnum):
    LIMITED = "limited"
    NEAR_LIMIT = "near_limit"
    OK = "ok"
    UNKNOWN = "unknown"

    def __str__(self) -> str:
        return str(self.value)
