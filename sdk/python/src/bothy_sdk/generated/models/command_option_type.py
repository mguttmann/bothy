from enum import StrEnum


class CommandOptionType(StrEnum):
    BOOLEAN = "boolean"
    CHOICE = "choice"
    FILE = "file"
    INTEGER = "integer"
    STRING = "string"
    USER = "user"

    def __str__(self) -> str:
        return str(self.value)
