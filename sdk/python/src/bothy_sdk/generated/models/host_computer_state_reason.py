from enum import StrEnum


class HostComputerStateReason(StrEnum):
    LOCKED = "locked"

    def __str__(self) -> str:
        return str(self.value)
