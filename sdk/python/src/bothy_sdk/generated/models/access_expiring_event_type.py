from enum import StrEnum


class AccessExpiringEventType(StrEnum):
    ACCESS_EXPIRING = "access.expiring"

    def __str__(self) -> str:
        return str(self.value)
