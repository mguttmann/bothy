from enum import StrEnum


class RunTriggerKind(StrEnum):
    COMMAND = "command"
    INTERACTION = "interaction"
    MESSAGE = "message"
    WAKE = "wake"

    def __str__(self) -> str:
        return str(self.value)
