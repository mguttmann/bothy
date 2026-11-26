from enum import StrEnum


class HostComputerEndedCloseReason(StrEnum):
    CONNECTION_LOST = "connection_lost"
    DEVICE_DISABLED = "device_disabled"
    DONE = "done"
    ERROR = "error"
    OPEN_FAILED = "open_failed"
    STOPPED_BY_USER = "stopped_by_user"

    def __str__(self) -> str:
        return str(self.value)
