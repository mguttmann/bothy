from enum import StrEnum


class CommandChangedEventType(StrEnum):
    COMMAND_CHANGED = "command.changed"

    def __str__(self) -> str:
        return str(self.value)
