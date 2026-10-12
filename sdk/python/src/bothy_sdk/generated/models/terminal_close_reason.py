from enum import StrEnum


class TerminalCloseReason(StrEnum):
    ACCESS_REVOKED = "access_revoked"
    CLIENT_CLOSED = "client_closed"
    CLOSED_BY_OWNER = "closed_by_owner"
    DETACH_EXPIRED = "detach_expired"
    DEVICE_REVOKED = "device_revoked"
    HOST_DISABLED = "host_disabled"
    HOST_POLICY_VIOLATION = "host_policy_violation"
    HOST_REKEYED = "host_rekeyed"
    HOST_RETIRED = "host_retired"
    I6_VIOLATION = "i6_violation"
    IDLE_TIMEOUT = "idle_timeout"
    LOGOUT = "logout"
    MAX_DURATION = "max_duration"
    OPEN_FAILED = "open_failed"
    PROCESS_EXITED = "process_exited"
    SESSION_REVOKED = "session_revoked"
    USER_DISABLED = "user_disabled"

    def __str__(self) -> str:
        return str(self.value)
