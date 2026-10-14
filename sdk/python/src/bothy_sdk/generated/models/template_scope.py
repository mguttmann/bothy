from enum import StrEnum


class TemplateScope(StrEnum):
    CHANNEL = "channel"
    PERSONAL = "personal"

    def __str__(self) -> str:
        return str(self.value)
