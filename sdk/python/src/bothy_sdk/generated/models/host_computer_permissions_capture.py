from enum import StrEnum


class HostComputerPermissionsCapture(StrEnum):
    DENIED = "denied"
    GRANTED = "granted"
    UNKNOWN = "unknown"

    def __str__(self) -> str:
        return str(self.value)
