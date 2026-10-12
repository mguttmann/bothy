from enum import StrEnum


class UserUpdatedEventType(StrEnum):
    USER_UPDATED = "user.updated"

    def __str__(self) -> str:
        return str(self.value)
