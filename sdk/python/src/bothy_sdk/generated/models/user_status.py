from enum import StrEnum


class UserStatus(StrEnum):
    ACTIVE = "active"
    DELETED = "deleted"
    DISABLED = "disabled"
    PENDING = "pending"

    def __str__(self) -> str:
        return str(self.value)
