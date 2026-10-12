from enum import StrEnum


class MessageCreatedEventType(StrEnum):
    MESSAGE_CREATED = "message.created"

    def __str__(self) -> str:
        return str(self.value)
