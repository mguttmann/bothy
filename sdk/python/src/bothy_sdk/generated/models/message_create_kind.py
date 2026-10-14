from enum import StrEnum


class MessageCreateKind(StrEnum):
    CARD = "card"
    TEXT = "text"
    VOICE = "voice"

    def __str__(self) -> str:
        return str(self.value)
