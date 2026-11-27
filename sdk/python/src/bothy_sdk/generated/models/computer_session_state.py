from enum import StrEnum


class ComputerSessionState(StrEnum):
    ACTIVE = "active"
    CLOSED = "closed"
    OPENING = "opening"
    PAUSED = "paused"

    def __str__(self) -> str:
        return str(self.value)
