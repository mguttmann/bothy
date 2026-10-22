from enum import StrEnum


class ExpiringAccessItemKind(StrEnum):
    GRANT = "grant"
    MEMBERSHIP = "membership"
    TOKEN = "token"

    def __str__(self) -> str:
        return str(self.value)
