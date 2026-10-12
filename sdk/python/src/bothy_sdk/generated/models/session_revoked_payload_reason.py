from enum import StrEnum


class SessionRevokedPayloadReason(StrEnum):
    ADMIN = "admin"
    DEVICE_REVOKED = "device_revoked"
    EXPIRED = "expired"
    INACTIVE = "inactive"
    LOGOUT = "logout"
    REFRESH_REUSE = "refresh_reuse"
    STEP_UP_FAILED = "step_up_failed"
    TRUSTED_HEADER_MISMATCH = "trusted_header_mismatch"
    USER_DISABLED = "user_disabled"

    def __str__(self) -> str:
        return str(self.value)
