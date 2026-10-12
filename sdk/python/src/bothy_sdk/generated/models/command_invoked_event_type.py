from enum import StrEnum


class CommandInvokedEventType(StrEnum):
    COMMAND_INVOKED = "command.invoked"

    def __str__(self) -> str:
        return str(self.value)
