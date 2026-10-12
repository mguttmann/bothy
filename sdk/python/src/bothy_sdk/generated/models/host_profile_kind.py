from enum import StrEnum


class HostProfileKind(StrEnum):
    CLAUDE = "claude"
    COMMAND = "command"
    SHELL = "shell"

    def __str__(self) -> str:
        return str(self.value)
