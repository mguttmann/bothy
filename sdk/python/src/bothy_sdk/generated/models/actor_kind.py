from enum import StrEnum


class ActorKind(StrEnum):
    AGENT = "agent"
    APP = "app"
    SYSTEM = "system"
    USER = "user"

    def __str__(self) -> str:
        return str(self.value)
