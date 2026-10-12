from enum import StrEnum


class SessionRevokedEventType(StrEnum):
    SESSION_REVOKED = "session.revoked"

    def __str__(self) -> str:
        return str(self.value)
