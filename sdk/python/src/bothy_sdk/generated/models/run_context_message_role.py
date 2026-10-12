from enum import StrEnum


class RunContextMessageRole(StrEnum):
    ASSISTANT = "assistant"
    OTHER = "other"
    USER = "user"

    def __str__(self) -> str:
        return str(self.value)
