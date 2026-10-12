from enum import StrEnum


class MessageKind(StrEnum):
    CARD = "card"
    RUN = "run"
    SYSTEM = "system"
    TEXT = "text"
    VOICE = "voice"

    def __str__(self) -> str:
        return str(self.value)
