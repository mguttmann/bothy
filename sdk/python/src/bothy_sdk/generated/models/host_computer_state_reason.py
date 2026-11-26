from enum import StrEnum


class HostComputerStateReason(StrEnum):
    LOCKED = "locked"
    VALUE_0 = ""

    def __str__(self) -> str:
        return str(self.value)
