from enum import StrEnum


class HeartbeatState(StrEnum):
    DEGRADED = "degraded"
    ONLINE = "online"

    def __str__(self) -> str:
        return str(self.value)
