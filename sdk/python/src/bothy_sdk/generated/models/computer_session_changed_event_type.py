from enum import StrEnum


class ComputerSessionChangedEventType(StrEnum):
    COMPUTER_SESSION_CHANGED = "computer_session.changed"

    def __str__(self) -> str:
        return str(self.value)
