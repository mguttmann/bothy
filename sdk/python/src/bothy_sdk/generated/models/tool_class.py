from enum import StrEnum


class ToolClass(StrEnum):
    CHAT = "chat"
    HOST = "host"
    SCOPED = "scoped"

    def __str__(self) -> str:
        return str(self.value)
