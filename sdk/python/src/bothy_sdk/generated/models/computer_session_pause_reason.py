from enum import StrEnum


class ComputerSessionPauseReason(StrEnum):
    LOCKED = "locked"

    def __str__(self) -> str:
        return str(self.value)
