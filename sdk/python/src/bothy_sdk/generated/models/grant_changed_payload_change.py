from enum import StrEnum


class GrantChangedPayloadChange(StrEnum):
    CREATED = "created"
    EXPIRED = "expired"
    REVOKED = "revoked"

    def __str__(self) -> str:
        return str(self.value)
