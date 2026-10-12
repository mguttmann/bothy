from enum import StrEnum


class TerminalSessionChangedEventType(StrEnum):
    TERMINAL_SESSION_CHANGED = "terminal_session.changed"

    def __str__(self) -> str:
        return str(self.value)
