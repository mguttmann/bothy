from enum import StrEnum


class GrantSubjectKind(StrEnum):
    AGENT = "agent"
    APP = "app"
    GROUP = "group"
    USER = "user"

    def __str__(self) -> str:
        return str(self.value)
