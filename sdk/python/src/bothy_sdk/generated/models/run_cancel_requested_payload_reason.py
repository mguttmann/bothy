from enum import StrEnum


class RunCancelRequestedPayloadReason(StrEnum):
    ACCESS_REVOKED = "access_revoked"
    AGENT_DISABLED = "agent_disabled"
    CHANNEL_READ_ONLY = "channel_read_only"
    USER_CANCELLED = "user_cancelled"
    USER_DISABLED = "user_disabled"

    def __str__(self) -> str:
        return str(self.value)
