from enum import StrEnum


class TerminalSessionState(StrEnum):
    ACTIVE = "active"
    CLOSED = "closed"
    DETACHED = "detached"
    OPENING = "opening"

    def __str__(self) -> str:
        return str(self.value)
