from enum import StrEnum


class TypingStartedEventType(StrEnum):
    TYPING_STARTED = "typing.started"

    def __str__(self) -> str:
        return str(self.value)
