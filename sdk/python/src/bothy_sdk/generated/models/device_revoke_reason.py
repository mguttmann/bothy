from enum import StrEnum


class DeviceRevokeReason(StrEnum):
    INACTIVE = "inactive"
    PAIRING_EXPIRED = "pairing_expired"
    PENDING_EXPIRED = "pending_expired"
    REVOKED_BY_OWNER = "revoked_by_owner"
    REVOKED_BY_SELF = "revoked_by_self"
    USER_DISABLED = "user_disabled"
    VERIFICATION_FAILED = "verification_failed"

    def __str__(self) -> str:
        return str(self.value)
