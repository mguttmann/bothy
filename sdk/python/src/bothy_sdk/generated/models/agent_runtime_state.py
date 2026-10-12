from enum import StrEnum


class AgentRuntimeState(StrEnum):
    DEGRADED = "degraded"
    OFFLINE = "offline"
    ONLINE = "online"
    UNKNOWN = "unknown"

    def __str__(self) -> str:
        return str(self.value)
