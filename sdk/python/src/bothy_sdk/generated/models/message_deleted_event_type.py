from enum import StrEnum


class MessageDeletedEventType(StrEnum):
    MESSAGE_DELETED = "message.deleted"

    def __str__(self) -> str:
        return str(self.value)
