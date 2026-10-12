from enum import StrEnum


class DeviceState(StrEnum):
    ACTIVE = "active"
    PENDING = "pending"
    REVOKED = "revoked"

    def __str__(self) -> str:
        return str(self.value)
