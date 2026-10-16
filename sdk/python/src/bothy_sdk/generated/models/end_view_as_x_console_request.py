from enum import StrEnum


class EndViewAsXConsoleRequest(StrEnum):
    VALUE_0 = "1"

    def __str__(self) -> str:
        return str(self.value)
