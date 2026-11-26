from enum import StrEnum


class HostComputerPermissionsInput(StrEnum):
    DENIED = "denied"
    GRANTED = "granted"
    UNKNOWN = "unknown"

    def __str__(self) -> str:
        return str(self.value)
