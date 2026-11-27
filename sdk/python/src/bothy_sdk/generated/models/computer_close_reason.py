from enum import StrEnum


class ComputerCloseReason(StrEnum):
    ACCESS_REVOKED = "access_revoked"
    CONNECTION_LOST = "connection_lost"
    DEVICE_DISABLED = "device_disabled"
    DONE = "done"
    ERROR = "error"
    HOST_DISABLED = "host_disabled"
    HOST_REKEYED = "host_rekeyed"
    HOST_RESTARTED = "host_restarted"
    HOST_RETIRED = "host_retired"
    OPEN_FAILED = "open_failed"
    RUN_ENDED = "run_ended"
    STOPPED_BY_USER = "stopped_by_user"

    def __str__(self) -> str:
        return str(self.value)
