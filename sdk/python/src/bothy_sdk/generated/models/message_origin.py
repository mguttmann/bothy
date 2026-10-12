from enum import StrEnum


class MessageOrigin(StrEnum):
    AGENT = "agent"
    APP = "app"
    HUMAN = "human"
    IMPORT = "import"
    SYSTEM = "system"

    def __str__(self) -> str:
        return str(self.value)
