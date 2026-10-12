from enum import StrEnum


class TerminalSessionGrantPrologueVersion(StrEnum):
    BOTHY_TERMINAL1 = "bothy-terminal/1"

    def __str__(self) -> str:
        return str(self.value)
