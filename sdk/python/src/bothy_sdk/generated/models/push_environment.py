from enum import StrEnum


class PushEnvironment(StrEnum):
    PRODUCTION = "production"
    SANDBOX = "sandbox"

    def __str__(self) -> str:
        return str(self.value)
