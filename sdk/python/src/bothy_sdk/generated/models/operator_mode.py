from enum import StrEnum


class OperatorMode(StrEnum):
    AGENT = "agent"
    INHERIT = "inherit"
    NONE = "none"

    def __str__(self) -> str:
        return str(self.value)
