from enum import StrEnum


class LlmCredentials(StrEnum):
    NONE = "none"
    ORGANIZATION = "organization"
    PERSONAL = "personal"

    def __str__(self) -> str:
        return str(self.value)
