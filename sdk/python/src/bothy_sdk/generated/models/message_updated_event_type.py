from enum import StrEnum


class MessageUpdatedEventType(StrEnum):
    MESSAGE_UPDATED = "message.updated"

    def __str__(self) -> str:
        return str(self.value)
