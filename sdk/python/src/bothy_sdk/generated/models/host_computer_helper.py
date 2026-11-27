from enum import StrEnum


class HostComputerHelper(StrEnum):
    ABSENT = "absent"
    CONNECTED = "connected"
    REFUSED = "refused"

    def __str__(self) -> str:
        return str(self.value)
