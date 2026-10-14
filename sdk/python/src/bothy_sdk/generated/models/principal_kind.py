from enum import StrEnum


class PrincipalKind(StrEnum):
    AGENT = "agent"
    APP = "app"
    USER = "user"

    def __str__(self) -> str:
        return str(self.value)
